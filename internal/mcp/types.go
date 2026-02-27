package mcp

import "encoding/json"

// JSON-RPC 2.0 message types

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification (no ID)
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Tool parameter types

// ListDevicesParams has no parameters
type ListDevicesParams struct{}

// SelectDeviceParams for selecting a device
type SelectDeviceParams struct {
	DeviceName string `json:"device_name,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
}

// ListFilesParams for listing files
type ListFilesParams struct {
	Path string `json:"path"`
}

// CheckPathParams for checking if a path exists
type CheckPathParams struct {
	Path string `json:"path"`
}

// GetDownloadLinkParams for getting download links
type GetDownloadLinkParams struct {
	Paths       []string `json:"paths"`
	Description string   `json:"description,omitempty"`
}

// GetDeviceStatusParams for checking device status
type GetDeviceStatusParams struct {
	DeviceID string `json:"device_id,omitempty"`
}

// Tool response types

// DeviceInfo represents device information
type DeviceInfo struct {
	DeviceID     string   `json:"device_id"`
	DeviceName   string   `json:"device_name"`
	Platform     string   `json:"platform"`
	IP           string   `json:"ip"`
	AllowedRoots []string `json:"allowed_roots"`
	Status       string   `json:"status"`
	LastSeen     string   `json:"last_seen"`
}

// FileInfo represents file information
type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
	ModifiedTime string `json:"modified_time"`
}

// PathInfo represents path existence information
type PathInfo struct {
	Exists       bool   `json:"exists"`
	IsFile       bool   `json:"is_file"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size,omitempty"`
	ModifiedTime string `json:"modified_time,omitempty"`
}

// DownloadLinkResponse represents a download link response
type DownloadLinkResponse struct {
	DownloadURL string   `json:"download_url"`
	FileName    string   `json:"file_name"`
	FileSize    int64    `json:"file_size"`
	ExpiresAt   string   `json:"expires_at"`
	Paths       []string `json:"paths"`
	Compressed  bool     `json:"compressed"`
}

// DeviceStatusResponse represents device status
type DeviceStatusResponse struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Status     string `json:"status"`
	LastSeen   string `json:"last_seen"`
}
