# MCP Quick Start Guide

## 1. Start the Server

```bash
./bin/server.exe
```

The server will start on `http://0.0.0.0:18120` with MCP enabled at `/mcp`.

## 2. Test with curl

```bash
# Test authentication and list devices
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"list_devices","params":{}}'
```

Expected response:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": [
    {
      "device_id": "...",
      "device_name": "...",
      "status": "online",
      ...
    }
  ]
}
```

## 3. Configure Claude Desktop

Edit `%APPDATA%\Claude\config.json` (Windows) or `~/.config/claude/config.json` (Mac/Linux):

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

Restart Claude Desktop.

## 4. Test with Claude

Ask Claude:

```
"List all online devices"
```

Claude will call the `list_devices` tool and show you the results.

```
"Select the 西小口店 device"
```

Claude will call `select_device` with fuzzy matching.

```
"Show me the files in Client/logs"
```

Claude will call `list_files` to browse the directory.

```
"Download the logs from Client/logs/2026-02-26"
```

Claude will:
1. Call `check_path` to verify it exists
2. Call `get_download_link` to get the URL
3. Download the file
4. Analyze the contents

## 5. Common Commands

### List devices
```
"Show me all connected devices"
"Which devices are online?"
```

### Select device
```
"Select the 西小口店 device"
"Use the device named 西小口"
```

### Browse files
```
"List files in Client/logs"
"Show me what's in Backend/log"
```

### Download files
```
"Download yesterday's client logs"
"Get the logs from 2026-02-26"
"Download Client/logs/2026-02-26"
```

## 6. Troubleshooting

### "Unauthorized" error
- Check the `auth_token` in config.yaml matches the one in Claude config
- Verify the Bearer token is correct

### "Device not found"
- Run `list_devices` to see available devices
- Check device name spelling
- Try partial name (fuzzy matching works)

### "Path not allowed"
- Verify the path is in the device's allowed_roots
- Check for typos in the path
- Don't use ".." in paths

### "Device offline"
- Check if the device is connected
- Look at the `last_seen` timestamp
- Wait for device to reconnect

## 7. Advanced Usage

### Session management
Use the same `X-Session-ID` header across requests to maintain context:

```bash
SESSION_ID="my-session-123"

# Select device
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "X-Session-ID: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"select_device","params":{"device_name":"西小口店"}}'

# List files (uses selected device from session)
curl -X POST http://localhost:18120/mcp/messages \
  -H "Authorization: Bearer mcp-secret-token-2025" \
  -H "X-Session-ID: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"list_files","params":{"path":"Client/logs"}}'
```

### Download multiple paths
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "get_download_link",
  "params": {
    "paths": [
      "Client/logs/2026-02-26",
      "Backend/log/2026-02-26.zip"
    ],
    "description": "all-logs-2026-02-26"
  }
}
```

This will compress both paths into a single zip file.

## 8. Security Notes

- Change the default `auth_token` in production
- Use HTTPS in production (not HTTP)
- Restrict access to the MCP endpoint with firewall rules
- Monitor audit logs for suspicious activity

## 9. Next Steps

- Read `docs/MCP_USAGE.md` for detailed API documentation
- Read `docs/MCP_README.md` for implementation details
- Run `scripts/test_mcp.sh` for interactive testing
- Check server logs for debugging

## 10. Support

If you encounter issues:
1. Check server logs for errors
2. Verify configuration in config.yaml
3. Test with curl before using Claude
4. Check device connectivity
5. Review error messages (they include suggestions)

## Example Workflow

```
1. User: "List all devices"
   → Claude calls list_devices
   → Shows: Windows_西小口店_10.1.23.120 (online)

2. User: "Select 西小口店"
   → Claude calls select_device
   → Device selected

3. User: "What logs are available?"
   → Claude calls list_files on Client/logs
   → Shows: 2026-02-26, 2026-02-27, ...

4. User: "Download yesterday's logs"
   → Claude calls get_download_link
   → Gets URL, downloads, analyzes
   → Shows: "Found 3 errors in the logs..."
```

That's it! You're ready to use MCP with Claude.
