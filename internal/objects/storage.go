package objects

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Storage manages object storage operations
type Storage struct {
	db          *sql.DB
	objectsDir  string
	logger      *zap.Logger
	maxFileSize int64 // Maximum file size in bytes
}

// NewStorage creates a new storage instance
func NewStorage(db *sql.DB, objectsDir string, maxFileSizeGB int, logger *zap.Logger) (*Storage, error) {
	// Create objects directory if it doesn't exist
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create objects directory: %w", err)
	}

	return &Storage{
		db:          db,
		objectsDir:  objectsDir,
		logger:      logger,
		maxFileSize: int64(maxFileSizeGB) * 1024 * 1024 * 1024,
	}, nil
}

// InitUpload initializes a new upload
func (s *Storage) InitUpload(req *InitUploadRequest) (*UploadedObject, error) {
	// Validate file size
	if req.FileSize > s.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size")
	}

	// Generate object ID
	objectID := uuid.New().String()

	// Generate storage path
	storagePath := s.getStoragePath(objectID)

	// Create object record
	obj := &UploadedObject{
		ObjectID:    objectID,
		DeviceID:    req.DeviceID,
		SourcePath:  req.SourcePath,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		SHA256:      req.SHA256,
		Status:      StatusPending,
		StoragePath: storagePath,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Insert into database
	query := `
		INSERT INTO uploaded_objects (
			object_id, device_id, source_path, file_name, file_size,
			sha256, status, storage_path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		obj.ObjectID, obj.DeviceID, obj.SourcePath, obj.FileName, obj.FileSize,
		obj.SHA256, obj.Status, obj.StoragePath, obj.CreatedAt, obj.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert object: %w", err)
	}

	s.logger.Info("Upload initialized",
		zap.String("object_id", objectID),
		zap.String("device_id", req.DeviceID),
		zap.String("file_name", req.FileName),
		zap.Int64("file_size", req.FileSize))

	return obj, nil
}

// GetObject retrieves an object by ID
func (s *Storage) GetObject(objectID string) (*UploadedObject, error) {
	query := `
		SELECT object_id, device_id, source_path, file_name, file_size,
		       sha256, status, storage_path, created_at, updated_at, completed_at
		FROM uploaded_objects
		WHERE object_id = ?
	`

	obj := &UploadedObject{}
	var completedAt sql.NullTime

	err := s.db.QueryRow(query, objectID).Scan(
		&obj.ObjectID, &obj.DeviceID, &obj.SourcePath, &obj.FileName, &obj.FileSize,
		&obj.SHA256, &obj.Status, &obj.StoragePath, &obj.CreatedAt, &obj.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("object not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query object: %w", err)
	}

	if completedAt.Valid {
		obj.CompletedAt = &completedAt.Time
	}

	return obj, nil
}

// UpdateObjectStatus updates the status of an object
func (s *Storage) UpdateObjectStatus(objectID string, status ObjectStatus) error {
	now := time.Now()

	var query string
	var args []interface{}

	if status == StatusCompleted {
		query = `
			UPDATE uploaded_objects
			SET status = ?, updated_at = ?, completed_at = ?
			WHERE object_id = ?
		`
		args = []interface{}{status, now, now, objectID}
	} else {
		query = `
			UPDATE uploaded_objects
			SET status = ?, updated_at = ?
			WHERE object_id = ?
		`
		args = []interface{}{status, now, objectID}
	}

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update object status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("object not found")
	}

	s.logger.Debug("Object status updated",
		zap.String("object_id", objectID),
		zap.String("status", string(status)))

	return nil
}

// ListObjects lists all objects, optionally filtered by device ID
func (s *Storage) ListObjects(deviceID string, limit, offset int) ([]*UploadedObject, error) {
	var query string
	var args []interface{}

	if deviceID != "" {
		query = `
			SELECT object_id, device_id, source_path, file_name, file_size,
			       sha256, status, storage_path, created_at, updated_at, completed_at
			FROM uploaded_objects
			WHERE device_id = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{deviceID, limit, offset}
	} else {
		query = `
			SELECT object_id, device_id, source_path, file_name, file_size,
			       sha256, status, storage_path, created_at, updated_at, completed_at
			FROM uploaded_objects
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{limit, offset}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query objects: %w", err)
	}
	defer rows.Close()

	var objects []*UploadedObject
	for rows.Next() {
		obj := &UploadedObject{}
		var completedAt sql.NullTime

		err := rows.Scan(
			&obj.ObjectID, &obj.DeviceID, &obj.SourcePath, &obj.FileName, &obj.FileSize,
			&obj.SHA256, &obj.Status, &obj.StoragePath, &obj.CreatedAt, &obj.UpdatedAt, &completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan object: %w", err)
		}

		if completedAt.Valid {
			obj.CompletedAt = &completedAt.Time
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// DeleteObject deletes an object and its file
func (s *Storage) DeleteObject(objectID string) error {
	// Get object info
	obj, err := s.GetObject(objectID)
	if err != nil {
		return err
	}

	// Delete file if it exists
	if obj.StoragePath != "" {
		fullPath := filepath.Join(s.objectsDir, obj.StoragePath)
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("Failed to delete object file",
				zap.String("object_id", objectID),
				zap.String("path", fullPath),
				zap.Error(err))
		}
	}

	// Delete from database
	query := `DELETE FROM uploaded_objects WHERE object_id = ?`
	_, err = s.db.Exec(query, objectID)
	if err != nil {
		return fmt.Errorf("failed to delete object from database: %w", err)
	}

	s.logger.Info("Object deleted",
		zap.String("object_id", objectID))

	return nil
}

// GetStoragePath returns the full storage path for an object
func (s *Storage) GetStoragePath(objectID string) string {
	obj, err := s.GetObject(objectID)
	if err != nil {
		return ""
	}
	return filepath.Join(s.objectsDir, obj.StoragePath)
}

// getStoragePath generates a storage path for an object
func (s *Storage) getStoragePath(objectID string) string {
	// Use first 2 characters of object ID as subdirectory for better distribution
	if len(objectID) < 2 {
		return objectID
	}
	return filepath.Join(objectID[:2], objectID)
}

// EnsureStorageDir ensures the storage directory for an object exists
func (s *Storage) EnsureStorageDir(objectID string) error {
	storagePath := s.getStoragePath(objectID)
	dir := filepath.Dir(filepath.Join(s.objectsDir, storagePath))
	return os.MkdirAll(dir, 0755)
}
