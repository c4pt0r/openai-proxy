# OpenAI API Proxy

A flexible proxy server for OpenAI API that enables request/response modification, real-time request tracing, and WebSocket-based monitoring.

## Features

- 🔄 Forward requests to OpenAI API with full request/response modification support
- 🔍 Real-time request tracing and monitoring
- 🛠️ Customizable request/response hooks for modifying API behavior
- 📊 Built-in trace viewer for debugging and monitoring
- 🔒 Automatic handling of gzip compression
- 🌐 Support for all OpenAI API v1 endpoints

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/c4pt0r/openai-proxy.git
cd openai-proxy
```

2. Run the server:
```bash
go run main.go
```

3. Frontend (Request Viewer)
```
cd web
npm install
npm start
```

The server will start on:
- OpenAI API Proxy: http://localhost:8080
- Trace Viewer: http://localhost:8081
- WebSocket Endpoint: ws://localhost:8081/ws

## Usage

### Basic Proxy Usage

Simply point your OpenAI API client to `http://localhost:8080` instead of `https://api.openai.com`. All `/v1/` endpoints are supported.

Example:
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"messages":[{"role":"user","content":"Hello!"}]}'
```

## Configuration

The server can be configured using command-line flags:

- `--port`: Port to listen on (default: 8080)
- `--host`: Host to listen on (default: localhost)

## Custom Hooks

You can implement custom request and response hooks to modify API behavior:

```go
SetRequestHook(func(body []byte, headers http.Header) ([]byte, http.Header, error) {
    // Modify request
    return body, headers, nil
})

SetResponseHook(func(body []byte, headers http.Header) ([]byte, http.Header, error) {
    // Modify response
    return body, headers, nil
})
```
