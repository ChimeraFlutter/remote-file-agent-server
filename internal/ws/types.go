package ws

import (
	"encoding/json"

	"github.com/remote-file-manager/server/internal/devices"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MsgTypeHello        MessageType = "hello"
	MsgTypeHelloAck     MessageType = "hello_ack"
	MsgTypeListReq      MessageType = "list_req"
	MsgTypeListResp     MessageType = "list_resp"
	MsgTypeDeleteReq    MessageType = "delete_req"
	MsgTypeDeleteResp   MessageType = "delete_resp"
	MsgTypeZipReq       MessageType = "zip_req"
	MsgTypeZipResp      MessageType = "zip_resp"
	MsgTypeCompressReq  MessageType = "compress_req"
	MsgTypeCompressResp MessageType = "compress_resp"
	MsgTypeUploadReq    MessageType = "upload_req"
	MsgTypeUploadResp   MessageType = "upload_resp"
	MsgTypeFileInfoReq  MessageType = "file_info_req"
	MsgTypeFileInfoResp MessageType = "file_info_resp"
	MsgTypeProgress     MessageType = "progress"
	MsgTypeHeartbeat    MessageType = "heartbeat"
	MsgTypeError        MessageType = "error"
)

// Envelope is the message wrapper for all WebSocket messages
type Envelope struct {
	Type      MessageType     `json:"type"`
	ReqID     string          `json:"req_id"`
	Timestamp int64           `json:"ts"`
	DeviceID  string          `json:"device_id"`
	Payload   json.RawMessage `json:"payload"`
}

// HelloPayload is sent by the agent when connecting
type HelloPayload struct {
	EnrollToken  string           `json:"enroll_token"`
	DeviceID     string           `json:"device_id"`
	DeviceName   string           `json:"device_name"`
	Platform     devices.Platform `json:"platform"`
	Version      string           `json:"version"`
	AllowedRoots []devices.Root   `json:"allowed_roots"`
}

// HelloAckPayload is the server's response to hello
type HelloAckPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// HeartbeatPayload contains system metrics
type HeartbeatPayload struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   int64   `json:"memory_mb"`
	DiskFreeGB int64   `json:"disk_free_gb"`
}

// ErrorPayload represents an error message
type ErrorPayload struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	ReqID   string    `json:"req_id,omitempty"`
}

// ErrorCode represents error codes
type ErrorCode string

const (
	ErrAuthFailed              ErrorCode = "AUTH_FAILED"
	ErrAgentEnrollTokenInvalid ErrorCode = "AGENT_ENROLL_TOKEN_INVALID"
	ErrDeviceOffline           ErrorCode = "DEVICE_OFFLINE"
	ErrPathOutsideWhitelist    ErrorCode = "PATH_OUTSIDE_WHITELIST"
	ErrFileNotFound            ErrorCode = "FILE_NOT_FOUND"
	ErrFileTooLarge            ErrorCode = "FILE_TOO_LARGE"
	ErrPermissionDenied        ErrorCode = "PERMISSION_DENIED"
	ErrDiskFull                ErrorCode = "DISK_FULL"
	ErrNetworkError            ErrorCode = "NETWORK_ERROR"
	ErrRPCTimeout              ErrorCode = "RPC_TIMEOUT"
	ErrSHA256Mismatch          ErrorCode = "SHA256_MISMATCH"
	ErrTokenExpired            ErrorCode = "TOKEN_EXPIRED"
	ErrZipFailed               ErrorCode = "ZIP_FAILED"
	ErrDeleteFailed            ErrorCode = "DELETE_FAILED"
	ErrInternalError           ErrorCode = "INTERNAL_ERROR"
)

// AdminNotifier is an interface for notifying admin clients
type AdminNotifier interface {
	BroadcastDeviceUpdate(device *devices.Device, event string)
	BroadcastProgress(envelope *Envelope)
}
