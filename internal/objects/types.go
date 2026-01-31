package objects

import "time"

// ObjectStatus represents the status of an uploaded object
type ObjectStatus string

const (
	StatusPending   ObjectStatus = "pending"   // Upload initiated but not completed
	StatusUploading ObjectStatus = "uploading" // Upload in progress
	StatusCompleted ObjectStatus = "completed" // Upload completed successfully
	StatusFailed    ObjectStatus = "failed"    // Upload failed
)

// UploadedObject represents a file stored in the object storage
type UploadedObject struct {
	ObjectID    string       `json:"object_id"`              // Unique object identifier
	DeviceID    string       `json:"device_id"`              // Source device ID
	SourcePath  string       `json:"source_path"`            // Original file path on device
	FileName    string       `json:"file_name"`              // File name
	FileSize    int64        `json:"file_size"`              // File size in bytes
	SHA256      string       `json:"sha256"`                 // SHA256 hash
	Status      ObjectStatus `json:"status"`                 // Upload status
	StoragePath string       `json:"storage_path"`           // Path in object storage
	CreatedAt   time.Time    `json:"created_at"`             // Creation time
	UpdatedAt   time.Time    `json:"updated_at"`             // Last update time
	CompletedAt *time.Time   `json:"completed_at,omitempty"` // Completion time
}

// DownloadToken represents a temporary download token
type DownloadToken struct {
	Token     string     `json:"token"`             // Token string
	ObjectID  string     `json:"object_id"`         // Associated object ID
	ExpiresAt time.Time  `json:"expires_at"`        // Expiration time
	CreatedAt time.Time  `json:"created_at"`        // Creation time
	UsedAt    *time.Time `json:"used_at,omitempty"` // Time when token was used
}

// InitUploadRequest represents a request to initialize an upload
type InitUploadRequest struct {
	DeviceID   string `json:"device_id"`
	SourcePath string `json:"source_path"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	SHA256     string `json:"sha256"`
}

// InitUploadResponse represents the response to an upload initialization
type InitUploadResponse struct {
	ObjectID  string `json:"object_id"`
	UploadURL string `json:"upload_url"`
}

// UploadProgressUpdate represents an upload progress update
type UploadProgressUpdate struct {
	ObjectID        string  `json:"object_id"`
	BytesUploaded   int64   `json:"bytes_uploaded"`
	TotalBytes      int64   `json:"total_bytes"`
	PercentComplete float64 `json:"percent_complete"`
}
