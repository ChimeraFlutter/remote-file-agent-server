package mcp

import "fmt"

// JSON-RPC 2.0 error codes
const (
	// Standard JSON-RPC errors
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603

	// Custom application errors
	ErrorCodeDeviceOffline  = -32002
	ErrorCodeDeviceNotFound = -32003
	ErrorCodePathNotAllowed = -32004
	ErrorCodeFileTooLarge   = -32005
	ErrorCodeRPCTimeout     = -32006
	ErrorCodeUnauthorized   = -32007
)

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// NewMCPError creates a new MCP error
func NewMCPError(code int, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Common error constructors
func ErrDeviceOffline(deviceID, deviceName, lastSeen string) *MCPError {
	return NewMCPError(ErrorCodeDeviceOffline, "设备离线", map[string]interface{}{
		"device_id":   deviceID,
		"device_name": deviceName,
		"last_seen":   lastSeen,
		"suggestion":  "设备可能已断开连接，请稍后重试或检查设备网络",
	})
}

func ErrDeviceNotFound(deviceName string) *MCPError {
	return NewMCPError(ErrorCodeDeviceNotFound, "设备未找到", map[string]interface{}{
		"device_name": deviceName,
		"suggestion":  "请使用 list_devices 查看可用设备",
	})
}

func ErrPathNotAllowed(path string) *MCPError {
	return NewMCPError(ErrorCodePathNotAllowed, "路径不在白名单内", map[string]interface{}{
		"path":       path,
		"suggestion": "请确保路径在设备的 allowed_roots 白名单内",
	})
}

func ErrFileTooLarge(sizeMB, maxSizeMB int) *MCPError {
	return NewMCPError(ErrorCodeFileTooLarge, "文件过大", map[string]interface{}{
		"file_size_mb": sizeMB,
		"max_size_mb":  maxSizeMB,
		"suggestion":   "请使用 get_download_link 工具下载大文件",
	})
}

func ErrRPCTimeout(timeoutSeconds int) *MCPError {
	return NewMCPError(ErrorCodeRPCTimeout, "RPC 超时", map[string]interface{}{
		"timeout_seconds": timeoutSeconds,
		"suggestion":      "操作耗时过长，可能是网络问题或文件过大",
	})
}

func ErrUnauthorized() *MCPError {
	return NewMCPError(ErrorCodeUnauthorized, "未授权", map[string]interface{}{
		"suggestion": "请提供有效的 Bearer token",
	})
}
