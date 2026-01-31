package rpc

import (
	"encoding/json"
	"time"
)

// Request represents an RPC request
type Request struct {
	ReqID     string          `json:"req_id"`
	DeviceID  string          `json:"device_id"`
	Method    string          `json:"method"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// Response represents an RPC response
type Response struct {
	ReqID     string          `json:"req_id"`
	Success   bool            `json:"success"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *ErrorInfo      `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ErrorInfo contains error details
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ListRequest represents a file list request
type ListRequest struct {
	Path string `json:"path"`
}

// ListResponse represents a file list response
type ListResponse struct {
	Path    string     `json:"path"`
	Entries []FileInfo `json:"entries"`
}

// FileInfo represents file metadata
type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
	ModifiedTime int64  `json:"modified_time"`
}

// DeleteRequest represents a file deletion request
type DeleteRequest struct {
	Path string `json:"path"`
}

// DeleteResponse represents a file deletion response
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ZipRequest represents a zip creation request
type ZipRequest struct {
	Paths  []string `json:"paths"`
	ZipID  string   `json:"zip_id"`
	ZipDir string   `json:"zip_dir"`
}

// ZipResponse represents a zip creation response
type ZipResponse struct {
	Success bool   `json:"success"`
	ZipID   string `json:"zip_id"`
	ZipPath string `json:"zip_path"`
	ZipSize int64  `json:"zip_size"`
	SHA256  string `json:"sha256"`
	Message string `json:"message,omitempty"`
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	ObjectID string `json:"object_id"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
}

// UploadResponse represents a file upload response
type UploadResponse struct {
	Success  bool   `json:"success"`
	ObjectID string `json:"object_id"`
	Message  string `json:"message,omitempty"`
}

// ProgressPayload represents upload/download progress
type ProgressPayload struct {
	ObjectID       string  `json:"object_id"`
	BytesProcessed int64   `json:"bytes_processed"`
	TotalBytes     int64   `json:"total_bytes"`
	Percent        float64 `json:"percent"`
}
