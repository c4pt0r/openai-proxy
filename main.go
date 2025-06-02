package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hook functions for request and response modification
type RequestHook func(body []byte, headers http.Header) ([]byte, http.Header, error)
type ResponseHook func(body []byte, headers http.Header) ([]byte, http.Header, error)

var (
	port = flag.Int("port", 8080, "OpenAI API port to listen on")
	host = flag.String("host", "localhost", "OpenAI API host to listen on")

	// Default hook implementations that can be replaced
	requestHook  RequestHook  = func(body []byte, headers http.Header) ([]byte, http.Header, error) { return body, headers, nil }
	responseHook ResponseHook = func(body []byte, headers http.Header) ([]byte, http.Header, error) { return body, headers, nil }
)

func messagesHook(messages []map[string]interface{}) ([]map[string]interface{}, error) {
	log.Printf("🔧 in messagesHook, Messages: %v", messages)
	messagesBuf := bytes.NewBuffer(nil)
	for _, message := range messages {
		content, contentOk := message["content"].(string)
		role, roleOk := message["role"].(string)
		if contentOk && roleOk {
			messagesBuf.WriteString(role + ": ")
			messagesBuf.WriteString(content)
			messagesBuf.WriteString("\n")
		}
	}
	trimLog := messagesBuf.String()
	if len(trimLog) > 2000 {
		trimLog = trimLog[:1000] + "\n......\n" + trimLog[len(trimLog)-1000:]
	}
	log.Printf("🔧 Messages in session: %s", trimLog)
	return messages, nil
}

func promptHook(body []byte, headers http.Header) ([]byte, http.Header, error) {
	// Parse the request body as JSON
	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		return body, headers, nil // Return original if not valid JSON
	}

	// Check if messages field exists and is an array
	if messages, ok := requestBody["messages"].([]interface{}); ok {
		// Convert messages to []map[string]interface{}
		messagesArray := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				messagesArray[i] = msgMap
			} else {
				return body, headers, nil // Return original if message format is invalid
			}
		}

		// Call messagesHook to process the messages
		modifiedMessages, err := messagesHook(messagesArray)
		if err != nil {
			return body, headers, err
		}

		// Update the messages in the request body
		requestBody["messages"] = modifiedMessages

		// Marshal back to JSON
		modifiedBody, err := json.Marshal(requestBody)
		if err != nil {
			return body, headers, err
		}

		return modifiedBody, headers, nil
	}

	return body, headers, nil
}

// SetRequestHook allows setting a custom request hook
func SetRequestHook(hook RequestHook) {
	requestHook = hook
}

// SetResponseHook allows setting a custom response hook
func SetResponseHook(hook ResponseHook) {
	responseHook = hook
}

// Trace holds information about a proxied request/response
type Trace struct {
	Id            string      `json:"id"`
	Timestamp     time.Time   `json:"timestamp"`
	Method        string      `json:"method"`
	URL           string      `json:"url"`
	Status        string      `json:"status"`
	Latency       float64     `json:"latency"`              // in seconds
	SessionId     string      `json:"session_id,omitempty"` // OpenAI API session ID
	RequestHeader http.Header `json:"request_headers,omitempty"`
	RequestBody   string      `json:"request_body,omitempty"`
	ResponseBody  string      `json:"response_body,omitempty"`
}

var traces []Trace
var tracesMax = 100 // keep only the latest 100 traces

// WebSocket specific
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for simplicity
	},
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan Trace
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan Trace),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		clients:    make(map[*websocket.Conn]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// Send existing traces to new client
			for _, trace := range traces {
				err := client.WriteJSON(trace)
				if err != nil {
					log.Printf("Error sending initial traces: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case trace := <-h.broadcast:
			h.mu.Lock()
			log.Printf("📡 Broadcasting trace to %d WebSocket clients", len(h.clients))
			for client := range h.clients {
				err := client.WriteJSON(trace)
				if err != nil {
					log.Printf("Write error: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

var hub = newHub()

// decompressBody decompresses gzipped response body
func decompressBody(body []byte, encoding string) ([]byte, error) {
	log.Printf("🔧 Attempting to decompress body with encoding: %s", encoding)

	if encoding == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			log.Printf("❌ Failed to create gzip reader: %v", err)
			return body, err // Return original if decompression fails
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			log.Printf("❌ Failed to decompress: %v", err)
			return body, err // Return original if decompression fails
		}

		log.Printf("✅ Successfully decompressed %d bytes -> %d bytes", len(body), len(decompressed))
		return decompressed, nil
	}

	log.Printf("ℹ️ No decompression needed for encoding: %s", encoding)
	return body, nil
}

// generateTraceID generates a simple unique ID for traces
func generateTraceID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// startOpenAIForwarder starts an HTTP server that forwards requests to OpenAI API
func startOpenAIForwarder() {
	// Create HTTP client for forwarding requests
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	SetRequestHook(promptHook)

	// Handler function for forwarding requests
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle /v1/ paths
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			http.Error(w, "Only /v1/ endpoints are supported", http.StatusNotFound)
			return
		}

		startTime := time.Now()
		log.Printf("\n🔄 === [FORWARDER REQUEST] ===")
		log.Printf("📍 Original URL: %s", r.URL.String())
		log.Printf("🔧 Method: %s", r.Method)

		// Create target URL
		targetURL := &url.URL{
			Scheme:   "https",
			Host:     "api.openai.com",
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}

		log.Printf("🎯 Target URL: %s", targetURL.String())

		// Read request body
		var bodyBytes []byte
		if r.Body != nil {
			var err error
			bodyBytes, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusInternalServerError)
				return
			}
		}

		// Apply request hook
		modifiedBody, modifiedHeaders, err := requestHook(bodyBytes, r.Header)
		if err != nil {
			log.Printf("❌ Request hook error: %v", err)
			http.Error(w, "Request hook error", http.StatusInternalServerError)
			return
		}
		bodyBytes = modifiedBody
		r.Header = modifiedHeaders

		// Create new request
		req, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers, but modify Accept-Encoding to disable compression for easier debugging
		for name, values := range r.Header {
			for _, value := range values {
				if name == "Accept-Encoding" {
					// Disable compression to get readable responses
					req.Header.Set(name, "identity")
				} else {
					req.Header.Add(name, value)
				}
			}
		}

		// If no Accept-Encoding was set, explicitly disable compression
		if req.Header.Get("Accept-Encoding") == "" {
			req.Header.Set("Accept-Encoding", "identity")
		}

		// Log important headers
		if auth := req.Header.Get("Authorization"); auth != "" {
			if strings.HasPrefix(auth, "Bearer sk-") && len(auth) > 20 {
				masked := auth[:15] + "***" + auth[len(auth)-4:]
				log.Printf("🔑 Authorization: %s", masked)
			}
		}
		if contentType := req.Header.Get("Content-Type"); contentType != "" {
			log.Printf("📄 Content-Type: %s", contentType)
		}

		// Execute request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("❌ Request failed: %v", err)
			http.Error(w, "Failed to forward request", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		latency := time.Since(startTime).Seconds()
		log.Printf("\n📥 === [FORWARDER RESPONSE] ===")
		log.Printf("📊 Status: %s", resp.Status)
		log.Printf("⏱️ Latency: %.3fs", latency)

		// Copy response headers first
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}

		// Set status code
		w.WriteHeader(resp.StatusCode)

		// Check if this is a streaming response (SSE)
		contentType := resp.Header.Get("Content-Type")
		isStreaming := strings.Contains(contentType, "text/event-stream") || strings.Contains(contentType, "text/plain")

		if isStreaming {
			log.Printf("🌊 Detected streaming response (Content-Type: %s), using streaming copy", contentType)

			// For streaming responses, copy directly without buffering
			bytesWritten, err := io.Copy(w, resp.Body)
			if err != nil {
				log.Printf("❌ Streaming copy error: %v", err)
				return
			}

			log.Printf("📏 Streamed %d bytes", bytesWritten)

			// Extract session ID from response
			sessionId := resp.Header.Get("X-Session-Id")
			log.Printf("🆔 Session ID: %s", sessionId)

			// Create trace for streaming request (without full response body)
			trace := Trace{
				Id:            generateTraceID(),
				Timestamp:     time.Now(),
				Method:        r.Method,
				URL:           targetURL.String(),
				Status:        resp.Status,
				Latency:       latency,
				SessionId:     sessionId,
				RequestHeader: r.Header,
				RequestBody:   string(bodyBytes),
				ResponseBody:  fmt.Sprintf("[STREAMING RESPONSE - %d bytes]", bytesWritten),
			}
			traces = append(traces, trace)
			if len(traces) > tracesMax {
				traces = traces[len(traces)-tracesMax:]
			}
			// Broadcast trace to WebSocket clients
			hub.broadcast <- trace
		} else {
			log.Printf("📦 Non-streaming response, buffering response body")

			// For non-streaming responses, use the original buffering approach
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, "Failed to read response", http.StatusInternalServerError)
				return
			}

			// Decompress if needed
			contentEncoding := resp.Header.Get("Content-Encoding")
			if contentEncoding != "" {
				decompressed, err := decompressBody(respBody, contentEncoding)
				if err == nil {
					respBody = decompressed
					// Remove Content-Encoding header since we're serving uncompressed content
					w.Header().Del("Content-Encoding")
				}
			}

			// Apply response hook
			modifiedRespBody, modifiedRespHeaders, err := responseHook(respBody, resp.Header)
			if err != nil {
				log.Printf("❌ Response hook error: %v", err)
				return
			}
			respBody = modifiedRespBody

			// Update headers if modified by hook
			for name, values := range modifiedRespHeaders {
				w.Header().Del(name)
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}

			// Write response body
			w.Write(respBody)

			// Log response body (truncated if too long)
			responseBodyStr := string(respBody)
			log.Printf("🔍 Response headers: %v", resp.Header)
			log.Printf("🔍 Content-Encoding: %s", resp.Header.Get("Content-Encoding"))
			log.Printf("📏 Response body length: %d bytes", len(respBody))

			if len(responseBodyStr) > 2000 {
				log.Printf("📝 Body: %s... (truncated, %d bytes total)", responseBodyStr[:2000], len(responseBodyStr))
			} else {
				log.Printf("📝 Body: %s", responseBodyStr)
			}

			// Extract session ID from response
			sessionId := resp.Header.Get("X-Session-Id")
			log.Printf("🆔 Session ID: %s", sessionId)

			// Create trace for this forwarded request
			trace := Trace{
				Id:            generateTraceID(),
				Timestamp:     time.Now(),
				Method:        r.Method,
				URL:           targetURL.String(),
				Status:        resp.Status,
				Latency:       latency,
				SessionId:     sessionId,
				RequestHeader: r.Header,
				RequestBody:   string(bodyBytes),
				ResponseBody:  responseBodyStr,
			}
			traces = append(traces, trace)
			if len(traces) > tracesMax {
				traces = traces[len(traces)-tracesMax:]
			}
			// Broadcast trace to WebSocket clients
			hub.broadcast <- trace
		}

		log.Println("=" + strings.Repeat("=", 30))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", *host, *port),
		Handler: handler,
	}

	log.Println("🌐 OpenAI API Server running on http://localhost:8080")
	log.Println("🔗 Example: http://localhost:8080/v1/chat/completions")
	log.Fatal(server.ListenAndServe())
}

func main() {
	go hub.run()

	// Start the OpenAI API server
	go startOpenAIForwarder()

	// Start HTTP server for trace viewing
	go func() {
		http.HandleFunc("/traces", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(traces)
		})
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			log.Printf("🔌 WebSocket connection attempt from %s", r.RemoteAddr)
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Printf("❌ WebSocket upgrade error: %v", err)
				return
			}
			log.Printf("✅ WebSocket connection established with %s", r.RemoteAddr)
			hub.register <- conn
			// Keep connection alive, unregister on error (e.g. client disconnects)
			go func() {
				defer func() {
					log.Printf("🔌 WebSocket connection closed with %s", r.RemoteAddr)
					hub.unregister <- conn
					conn.Close()
				}()
				for {
					// Read messages (optional, if you expect client messages)
					// For now, just keep the connection open.
					// If an error occurs (client disconnects), the loop will break.
					if _, _, err := conn.NextReader(); err != nil {
						break
					}
				}
			}()
		})
		log.Println("📊 Trace viewer running on :8081, WebSocket on /ws")
		log.Fatal(http.ListenAndServe(":8081", nil))
	}()

	// Keep the main goroutine running
	select {}
}
