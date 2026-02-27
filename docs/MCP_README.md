# MCP (Model Context Protocol) Implementation

## Overview

This implementation adds MCP support to the remote-file-agent-server, allowing AI assistants like Claude to interact with remote device files through a standardized JSON-RPC 2.0 API.

## Architecture

```
AI (Claude Desktop / Web)
    ↕ HTTP + SSE (JSON-RPC 2.0)
MCP Server (integrated in main server)
    ↕ Internal calls
Remote File Agent Server
    ↕ WebSocket
Remote Devices (clients)
```

## Features

### Core Capabilities

1. **Device Management**
   - List all online devices
   - Select device by name (fuzzy matching)
   - Check device status

2. **File Operations**
   - List files in directories
   - Check if paths exist
   - Get download links for files/folders

3. **Smart Download**
   - Single files: Direct upload (no compression)
   - Folders: Automatic compression
   - Multiple paths: Automatic compression
   - 10-minute download token expiry

### Security

- Bearer token authentication
- Path whitelist validation (allowed_roots)
- Directory traversal prevention (..)
- Session-based context management
- 30-minute session timeout

## Implementation Files

### Core MCP Package (`internal/mcp/`)

- `server.go` - HTTP + SSE server, authentication, routing
- `types.go` - JSON-RPC 2.0 types, tool parameters/responses
- `errors.go` - Error codes and constructors
- `session.go` - Session management with timeout
- `tools.go` - Tool implementations (devices, files)

### Configuration

- `pkg/config/config.go` - Added MCPConfig struct
- `config.yaml` - MCP configuration section

### Integration

- `cmd/server/main.go` - MCP server initialization and routing

## Configuration

```yaml
mcp:
  enabled: true
  auth_token: "mcp-secret-token-2025"
  session_timeout_minutes: 30
  endpoint: "/mcp"
```

## API Endpoints

- `POST /mcp/messages` - JSON-RPC 2.0 requests
- `GET /mcp/sse` - Server-Sent Events for responses

## Available Tools

1. `list_devices` - List all online devices
2. `select_device` - Select device (fuzzy matching)
3. `get_device_status` - Check device status
4. `list_files` - List directory contents
5. `check_path` - Verify path exists
6. `get_download_link` - Get download URL (smart compression)

## Usage Examples

### With curl

```bash
# List devices
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"list_devices","params":{}}'

# Select device
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "X-Session-ID: my-session" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"select_device","params":{"device_name":"西小口店"}}'

# Get download link
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "X-Session-ID: my-session" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"get_download_link","params":{"paths":["Client/logs/2026-02-26"]}}'
```

### With Claude Desktop

Add to `%APPDATA%\Claude\config.json`:

```json
{
  "mcpServers": {
    "remote-file-agent": {
      "url": "http://112.84.176.170:18120/mcp",
      "headers": {
        "Authorization": "Bearer mcp-secret-token-2025"
      }
    }
  }
}
```

Then ask Claude:
- "List all online devices"
- "Select the 西小口店 device"
- "Download yesterday's client logs"

## Error Handling

All errors follow JSON-RPC 2.0 format with custom error codes:

- `-32002` - Device offline
- `-32003` - Device not found
- `-32004` - Path not allowed
- `-32005` - File too large
- `-32006` - RPC timeout
- `-32007` - Unauthorized

Each error includes helpful suggestions in the `data` field.

## Session Management

- Sessions created automatically with `X-Session-ID` header
- Store selected device context
- 30-minute inactivity timeout
- Automatic cleanup of expired sessions

## Smart Download Logic

The `get_download_link` tool intelligently handles different scenarios:

1. **Single file**: Direct upload (no compression)
   - Faster for individual files
   - Preserves original file

2. **Single folder**: Compress then upload
   - Reduces transfer size
   - Maintains directory structure

3. **Multiple paths**: Compress then upload
   - Bundles everything into one archive
   - Efficient for batch downloads

## Testing

Run the test script:

```bash
bash scripts/test_mcp.sh
```

Or use the interactive test in `docs/MCP_USAGE.md`.

## Future Enhancements

- Real-time log monitoring (tail -f)
- Log content search (grep)
- Cross-device log comparison
- Time range filtering
- Incremental downloads
- AI-powered log analysis

## Troubleshooting

### Authentication fails
- Check `auth_token` in config.yaml
- Verify Bearer token in request header

### Device not found
- Ensure device is online
- Check device name spelling
- Try fuzzy matching (partial name)

### Path not allowed
- Verify path is in device's allowed_roots
- Check for ".." in path
- Use forward slashes (/)

### RPC timeout
- Check device network connection
- Increase timeout for large files
- Verify device is responding

## Documentation

- `docs/MCP_USAGE.md` - Detailed API documentation
- `scripts/test_mcp.sh` - Test script
- This README - Implementation overview
