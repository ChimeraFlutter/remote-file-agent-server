package objects

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/remote-file-manager/server/internal/rpc"
	"go.uber.org/zap"
)

// ZipService handles zip operations
type ZipService struct {
	storage    *Storage
	rpcManager *rpc.Manager
	logger     *zap.Logger
	serverAddr string
}

// NewZipService creates a new zip service
func NewZipService(storage *Storage, rpcManager *rpc.Manager, logger *zap.Logger, serverAddr string) *ZipService {
	return &ZipService{
		storage:    storage,
		rpcManager: rpcManager,
		logger:     logger,
		serverAddr: serverAddr,
	}
}

// RequestZip sends a compress request to a device and waits for completion
func (zs *ZipService) RequestZip(ctx context.Context, deviceID string, paths []string) (*UploadedObject, error) {
	if len(paths) != 1 {
		return nil, fmt.Errorf("compress only supports single path")
	}

	// Create compress request (compatible with Flutter agent)
	compressReq := map[string]string{
		"path": paths[0],
	}

	zs.logger.Info("Requesting compress operation",
		zap.String("device_id", deviceID),
		zap.Strings("paths", paths))

	// Send RPC request to device using compress method instead of zip
	resp, err := zs.rpcManager.Call(ctx, deviceID, "compress", compressReq, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to request compress: %w", err)
	}

	if !resp.Success {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return nil, fmt.Errorf("compress request failed: %s", errMsg)
	}

	// Parse compress response
	var compressResp map[string]interface{}
	if err := json.Unmarshal(resp.Payload, &compressResp); err != nil {
		return nil, fmt.Errorf("failed to parse compress response: %w", err)
	}

	// Extract compressed file info
	zipPath, ok := compressResp["zip_path"].(string)
	if !ok {
		return nil, fmt.Errorf("compress response missing zip_path")
	}

	zipName, ok := compressResp["zip_name"].(string)
	if !ok {
		zipName = "compressed.zip"
	}

	zipSize, ok := compressResp["size"].(float64) // JSON numbers are float64
	if !ok {
		return nil, fmt.Errorf("compress response missing size")
	}

	zs.logger.Info("Compress completed successfully",
		zap.String("device_id", deviceID),
		zap.String("zip_path", zipPath),
		zap.String("zip_name", zipName),
		zap.Int64("zip_size", int64(zipSize)))

	// Now trigger upload of the compressed file
	// Initialize upload in database
	initReq := &InitUploadRequest{
		DeviceID:   deviceID,
		SourcePath: zipPath,
		FileName:   zipName,
		FileSize:   int64(zipSize),
	}

	obj, err := zs.storage.InitUpload(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize upload: %w", err)
	}

	// Generate upload URL
	uploadURL := fmt.Sprintf("http://%s/api/objects/upload/%s", zs.serverAddr, obj.ObjectID)

	// Send upload request to device with upload_url
	uploadReq := rpc.UploadRequest{
		ObjectID:  obj.ObjectID,
		Path:      zipPath,
		UploadURL: uploadURL,
	}

	uploadResp, err := zs.rpcManager.Call(ctx, deviceID, "upload", uploadReq, 30*time.Minute)
	if err != nil {
		zs.storage.UpdateObjectStatus(obj.ObjectID, StatusFailed)
		return nil, fmt.Errorf("failed to request upload: %w", err)
	}

	if !uploadResp.Success {
		zs.storage.UpdateObjectStatus(obj.ObjectID, StatusFailed)
		errMsg := "unknown error"
		if uploadResp.Error != nil {
			errMsg = uploadResp.Error.Message
		}
		return nil, fmt.Errorf("upload request failed: %s", errMsg)
	}

	zs.logger.Info("Compressed file upload initiated",
		zap.String("device_id", deviceID),
		zap.String("object_id", obj.ObjectID))

	return obj, nil
}

// GetZipStatus returns the status of a zip operation
func (zs *ZipService) GetZipStatus(objectID string) (*UploadedObject, error) {
	return zs.storage.GetObject(objectID)
}
