# OpenAI Proxy with Lua Hooks

A flexible OpenAI API proxy server with Lua-based request and response hooks for dynamic request/response modification.

## Features

- **Proxy OpenAI API requests** to `api.openai.com`
- **Lua-based hooks** for flexible request and response modification
- **JSON support** in Lua using `layeh.com/gopher-json`
- **Single file hooks** with both request and response processing
- **Real-time tracing** with WebSocket support
- **Request/Response logging** with detailed information
- **Built-in message processing** for OpenAI chat completions

## Installation

1. Make sure you have Go installed (version 1.19 or later)
2. Clone or download this repository
3. Install dependencies:
```bash
go mod init openai-proxy
go get github.com/yuin/gopher-lua
go get github.com/gorilla/websocket
go get layeh.com/gopher-json
```

## Usage

### Basic Usage
```bash
go run main.go
```

### With Lua Hooks
```bash
go run main.go -lua=hooks.lua
```

### Command Line Options
- `-port`: Port to listen on (default: 8080)
- `-host`: Host to bind to (default: localhost)
- `-lua`: Path to Lua script with processRequest and processResponse functions

## Lua Hook System

### Single File Approach

Create a single Lua file with both `processRequest` and `processResponse` functions:

```lua
local json = require("json")

function processRequest(body, headers)
    -- Parse JSON body
    local success, data = pcall(json.decode, body)
    if not success then
        return body, headers
    end
    
    -- Modify the request
    -- ... your logic here ...
    
    -- Return modified body and headers
    local modifiedBody = json.encode(data)
    return modifiedBody, headers
end

function processResponse(body, headers)
    -- Parse JSON body
    local success, data = pcall(json.decode, body)
    if not success then
        return body, headers
    end
    
    -- Modify the response
    -- ... your logic here ...
    
    -- Return modified body and headers
    local modifiedBody = json.encode(data)
    return modifiedBody, headers
end
```

### Function Requirements

- **processRequest(body, headers)**: Optional function for request processing
- **processResponse(body, headers)**: Optional function for response processing
- At least one function must be defined
- Both functions receive:
  - `body`: String containing JSON request/response body
  - `headers`: Table with HTTP headers
- Both functions must return:
  - Modified body (string)
  - Modified headers (table)

### JSON Support

The Lua environment includes full JSON support via `layeh.com/gopher-json`:

```lua
local json = require("json")

-- Parse JSON
local data = json.decode(jsonString)

-- Generate JSON
local jsonString = json.encode(data)
```

### Example Use Cases

1. **Rate Limiting**: Add custom rate limiting logic
2. **Content Filtering**: Filter or modify request/response content
3. **Analytics**: Log detailed request/response information with cost calculations
4. **Authentication**: Add custom authentication logic
5. **Content Enhancement**: Modify prompts or responses dynamically
6. **Debugging**: Add debug information to requests/responses
7. **Token Management**: Monitor and control token usage
8. **Response Enrichment**: Add metadata to responses

## Example Script

The included `hooks.lua` demonstrates:

### Request Processing
- Adds custom headers
- Automatically adds system messages
- Reduces high temperature values
- Sets reasonable max_tokens limits
- Logs model information

### Response Processing
- Logs token usage and cost estimation
- Adds metadata to response choices
- Counts words in responses
- Logs finish reasons and model info
- Adds processing timestamps

## API Endpoints

### Proxy Endpoint
- **URL**: `http://localhost:8080/v1/*`
- **Method**: All HTTP methods
- **Description**: Proxies requests to OpenAI API

### Trace Viewing
- **URL**: `http://localhost:8081/traces`
- **Method**: GET
- **Description**: Returns JSON array of request/response traces

### WebSocket
- **URL**: `ws://localhost:8081/ws`
- **Description**: Real-time trace updates via WebSocket

## Configuration

Set your OpenAI API key in your client application. The proxy forwards the `Authorization` header to OpenAI.

Example with curl:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-openai-api-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.9
  }'
```

With the example hooks, this will:
- Add a system message automatically
- Reduce temperature from 0.9 to 0.8
- Set max_tokens to 2000
- Log token usage and costs
- Add metadata to the response

## Development

### Adding New Hook Features

1. Modify your Lua script to add new functionality
2. Use the JSON library for proper parsing and modification
3. Test with the built-in tracing system
4. Monitor performance impact

### Error Handling

The Lua hook system is designed to be robust:
- If a Lua script fails to load, the server logs the error but continues
- If a Lua hook fails during execution, the original request/response is returned
- All errors are logged for debugging
- Invalid JSON is handled gracefully

### Debugging Lua Scripts

- Use `print()` statements in your Lua code for logging
- Check the server output for Lua execution logs
- Use the trace viewer to see before/after request/response data
- Test individual functions with simple JSON examples

## Security Considerations

- Lua scripts have access to request/response data
- Validate and sanitize any external input in Lua scripts
- Be careful when modifying request/response structures
- Consider the performance impact of complex Lua scripts
- Review scripts for potential security vulnerabilities

## Performance

- Lua scripts are executed for each request/response
- JSON parsing/encoding adds some overhead
- Complex scripts may impact latency
- Consider caching and optimization for high-traffic scenarios
- Monitor the proxy performance with built-in tracing

## License

This project is provided as-is for educational and development purposes.
