# MCP Server Usage Guide

## Overview

The MCP (Model Context Protocol) server provides AI-accessible tools for managing remote device files. It exposes file operations through a JSON-RPC 2.0 API over HTTP + SSE.

## Configuration

Add to `config.yaml`:

```yaml
mcp:
  enabled: true
  auth_token: "mcp-secret-token-2025"
  session_timeout_minutes: 30
  endpoint: "/mcp"
```

## Authentication

All requests require Bearer token authentication:

```
Authorization: Bearer mcp-secret-token-2025
```

## Endpoints

- `POST /mcp/messages` - Send JSON-RPC 2.0 requests
- `GET /mcp/sse` - Establish SSE connection for responses

## Available Tools

### 1. list_devices

Lists all online devices.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "list_devices",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": [
    {
      "device_id": "store-001",
      "device_name": "Windows_西小口店_10.1.23.120",
      "platform": "windows",
      "ip": "10.1.23.120",
      "allowed_roots": ["C:\\Users\\ACEWILL\\AppData\\Roaming\\CXJPos"],
      "status": "online",
      "last_seen": "2026-02-27T10:30:00Z"
    }
  ]
}
```

### 2. select_device

Selects a device for subsequent operations (supports fuzzy matching).

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "select_device",
  "params": {
    "device_name": "西小口店"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "device_id": "store-001",
    "device_name": "Windows_西小口店_10.1.23.120",
    "platform": "windows",
    "ip": "10.1.23.120",
    "allowed_roots": ["C:\\Users\\ACEWILL\\AppData\\Roaming\\CXJPos"],
    "status": "online",
    "last_seen": "2026-02-27T10:30:00Z"
  }
}
```

### 3. list_files

Lists files in a directory.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "list_files",
  "params": {
    "path": "Client/logs"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": [
    {
      "name": "2026-02-26",
      "path": "Client/logs/2026-02-26",
      "is_dir": true,
      "size": 0,
      "modified_time": "2026-02-26T00:00:00Z"
    }
  ]
}
```

### 4. check_path

Checks if a path exists and returns its info.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "check_path",
  "params": {
    "path": "Client/logs/2026-02-26"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "exists": true,
    "is_file": false,
    "is_dir": true,
    "size": 0,
    "modified_time": "2026-02-26T00:00:00Z"
  }
}
```

### 5. get_download_link

Gets download links for files/folders (automatically compresses folders).

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "get_download_link",
  "params": {
    "paths": ["Client/logs/2026-02-26"],
    "description": "client-logs-2026-02-26"
  }
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "download_url": "http://112.84.176.170:18120/api/objects/download/abc123",
    "file_name": "Windows_西小口店_10.1.23.120-client-logs-2026-02-26.zip",
    "file_size": 2621440,
    "expires_at": "2026-02-27T10:40:00Z",
    "paths": ["Client/logs/2026-02-26"],
    "compressed": true
  }
}
```

### 6. get_device_status

Checks device online status.

**Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "get_device_status",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "device_id": "store-001",
    "device_name": "Windows_西小口店_10.1.23.120",
    "status": "online",
    "last_seen": "2026-02-27T10:30:00Z"
  }
}
```

## Error Handling

Errors follow JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32002,
    "message": "设备离线",
    "data": {
      "device_id": "store-001",
      "device_name": "Windows_西小口店_10.1.23.120",
      "last_seen": "2026-02-27T01:30:00Z",
      "suggestion": "设备可能已断开连接，请稍后重试或检查设备网络"
    }
  }
}
```

### Error Codes

- `-32700` - Parse error
- `-32600` - Invalid request
- `-32601` - Method not found
- `-32602` - Invalid params
- `-32603` - Internal error
- `-32002` - Device offline
- `-32003` - Device not found
- `-32004` - Path not allowed
- `-32005` - File too large
- `-32006` - RPC timeout
- `-32007` - Unauthorized

## Testing with curl

```bash
# List devices
curl -X POST http://112.84.176.170:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "list_devices",
    "params": {}
  }'

# Select device
curl -X POST http://112.84.176.170:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "Content-Type: application/json" \
  -H "X-Session-ID: test-session-123" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "select_device",
    "params": {
      "device_name": "西小口店"
    }
  }'

# Get download link
curl -X POST http://112.84.176.170:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "Content-Type: application/json" \
  -H "X-Session-ID: test-session-123" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "get_download_link",
    "params": {
      "paths": ["Client/logs/2026-02-26"],
      "description": "client-logs"
    }
  }'
```

## Claude Desktop Configuration

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

## Session Management

- Sessions are created automatically when you provide an `X-Session-ID` header
- Sessions store the selected device context
- Sessions expire after 30 minutes of inactivity
- Use the same session ID across requests to maintain context

## Security

- All paths must be within device's `allowed_roots` whitelist
- Paths containing ".." are rejected (prevents directory traversal)
- Download tokens expire after 10 minutes
- Bearer token authentication required for all requests
