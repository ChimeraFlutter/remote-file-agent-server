package objects

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"
)

// UploadHandler handles file uploads
type UploadHandler struct {
	storage *Storage
	logger  *zap.Logger
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(storage *Storage, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{
		storage: storage,
		logger:  logger,
	}
}

// HandleUpload processes a file upload
func (uh *UploadHandler) HandleUpload(objectID string, reader io.Reader) error {
	// Get object info
	obj, err := uh.storage.GetObject(objectID)
	if err != nil {
		return fmt.Errorf("object not found: %w", err)
	}

	// Check if object is in pending or uploading state
	if obj.Status != StatusPending && obj.Status != StatusUploading {
		return fmt.Errorf("object is not in uploadable state: %s", obj.Status)
	}

	// Update status to uploading
	if err := uh.storage.UpdateObjectStatus(objectID, StatusUploading); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Ensure storage directory exists
	if err := uh.storage.EnsureStorageDir(objectID); err != nil {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Get full storage path
	fullPath := uh.storage.GetStoragePath(objectID)

	// Create temporary file
	tempPath := fullPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		// Clean up temp file if upload failed
		if err != nil {
			os.Remove(tempPath)
		}
	}()

	// Create SHA256 hasher
	hasher := sha256.New()

	// Use TeeReader to calculate hash while writing
	teeReader := io.TeeReader(reader, hasher)

	// Copy data to file with size limit
	written, err := io.CopyN(tempFile, teeReader, obj.FileSize+1)
	if err != nil && err != io.EOF {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Check if file size matches
	if written != obj.FileSize {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("file size mismatch: expected %d, got %d", obj.FileSize, written)
	}

	// Close temp file before renaming
	if err := tempFile.Close(); err != nil {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Calculate SHA256
	calculatedHash := hex.EncodeToString(hasher.Sum(nil))

	// Verify SHA256 if provided
	if obj.SHA256 != "" && obj.SHA256 != calculatedHash {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", obj.SHA256, calculatedHash)
	}

	// Rename temp file to final path
	if err := os.Rename(tempPath, fullPath); err != nil {
		uh.storage.UpdateObjectStatus(objectID, StatusFailed)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Update status to completed
	if err := uh.storage.UpdateObjectStatus(objectID, StatusCompleted); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	uh.logger.Info("Upload completed",
		zap.String("object_id", objectID),
		zap.String("file_name", obj.FileName),
		zap.Int64("file_size", obj.FileSize),
		zap.String("sha256", calculatedHash))

	return nil
}

// GetUploadProgress returns the current upload progress for an object
func (uh *UploadHandler) GetUploadProgress(objectID string) (*UploadProgressUpdate, error) {
	obj, err := uh.storage.GetObject(objectID)
	if err != nil {
		return nil, err
	}

	// Get current file size
	fullPath := uh.storage.GetStoragePath(objectID)
	tempPath := fullPath + ".tmp"

	var currentSize int64
	if info, err := os.Stat(tempPath); err == nil {
		currentSize = info.Size()
	} else if info, err := os.Stat(fullPath); err == nil {
		currentSize = info.Size()
	}

	percentComplete := float64(0)
	if obj.FileSize > 0 {
		percentComplete = float64(currentSize) / float64(obj.FileSize) * 100
	}

	return &UploadProgressUpdate{
		ObjectID:        objectID,
		BytesUploaded:   currentSize,
		TotalBytes:      obj.FileSize,
		PercentComplete: percentComplete,
	}, nil
}

// CancelUpload cancels an ongoing upload
func (uh *UploadHandler) CancelUpload(objectID string) error {
	obj, err := uh.storage.GetObject(objectID)
	if err != nil {
		return err
	}

	// Only cancel if in pending or uploading state
	if obj.Status != StatusPending && obj.Status != StatusUploading {
		return fmt.Errorf("cannot cancel upload in state: %s", obj.Status)
	}

	// Delete temp file if exists
	fullPath := uh.storage.GetStoragePath(objectID)
	tempPath := fullPath + ".tmp"
	os.Remove(tempPath)

	// Update status to failed
	if err := uh.storage.UpdateObjectStatus(objectID, StatusFailed); err != nil {
		return err
	}

	uh.logger.Info("Upload cancelled", zap.String("object_id", objectID))

	return nil
}

// ValidateUploadRequest validates an upload initialization request
func ValidateUploadRequest(req *InitUploadRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}
	if req.SourcePath == "" {
		return fmt.Errorf("source_path is required")
	}
	if req.FileName == "" {
		return fmt.Errorf("file_name is required")
	}
	if req.FileSize <= 0 {
		return fmt.Errorf("file_size must be positive")
	}
	if req.FileSize > 10*1024*1024*1024 { // 10GB limit
		return fmt.Errorf("file_size exceeds maximum allowed size")
	}
	return nil
}
