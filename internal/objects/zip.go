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
}

// NewZipService creates a new zip service
func NewZipService(storage *Storage, rpcManager *rpc.Manager, logger *zap.Logger) *ZipService {
	return &ZipService{
		storage:    storage,
		rpcManager: rpcManager,
		logger:     logger,
	}
}

// RequestZip sends a zip request to a device and waits for completion
func (zs *ZipService) RequestZip(ctx context.Context, deviceID string, paths []string) (*UploadedObject, error) {
	// Generate zip ID
	zipID := fmt.Sprintf("zip-%d", time.Now().Unix())

	// Create zip request
	zipReq := rpc.ZipRequest{
		Paths:  paths,
		ZipID:  zipID,
		ZipDir: "/tmp", // Temporary directory on device
	}

	zs.logger.Info("Requesting zip creation",
		zap.String("device_id", deviceID),
		zap.String("zip_id", zipID),
		zap.Strings("paths", paths))

	// Send RPC request to device
	resp, err := zs.rpcManager.Call(ctx, deviceID, "zip", zipReq, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to request zip: %w", err)
	}

	if !resp.Success {
		errMsg := "unknown error"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return nil, fmt.Errorf("zip request failed: %s", errMsg)
	}

	// Parse zip response
	var zipResp rpc.ZipResponse
	if err := json.Unmarshal(resp.Payload, &zipResp); err != nil {
		return nil, fmt.Errorf("failed to parse zip response: %w", err)
	}

	if !zipResp.Success {
		return nil, fmt.Errorf("zip creation failed: %s", zipResp.Message)
	}

	zs.logger.Info("Zip created successfully",
		zap.String("device_id", deviceID),
		zap.String("zip_id", zipResp.ZipID),
		zap.String("zip_path", zipResp.ZipPath),
		zap.Int64("zip_size", zipResp.ZipSize),
		zap.String("sha256", zipResp.SHA256))

	// Now trigger upload of the zip file
	// Initialize upload in database
	initReq := &InitUploadRequest{
		DeviceID:   deviceID,
		SourcePath: zipResp.ZipPath,
		FileName:   fmt.Sprintf("%s.zip", zipID),
		FileSize:   zipResp.ZipSize,
		SHA256:     zipResp.SHA256,
	}

	obj, err := zs.storage.InitUpload(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize upload: %w", err)
	}

	// Send upload request to device
	uploadReq := rpc.UploadRequest{
		ObjectID: obj.ObjectID,
		Path:     zipResp.ZipPath,
		SHA256:   zipResp.SHA256,
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

	zs.logger.Info("Zip upload initiated",
		zap.String("device_id", deviceID),
		zap.String("object_id", obj.ObjectID),
		zap.String("zip_id", zipID))

	return obj, nil
}

// GetZipStatus returns the status of a zip operation
func (zs *ZipService) GetZipStatus(objectID string) (*UploadedObject, error) {
	return zs.storage.GetObject(objectID)
}
