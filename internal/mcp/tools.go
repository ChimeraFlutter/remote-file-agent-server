package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/remote-file-manager/server/internal/devices"
	"github.com/remote-file-manager/server/internal/objects"
	"github.com/remote-file-manager/server/internal/rpc"
	"go.uber.org/zap"
)

// registerTools registers all MCP tools
func (s *Server) registerTools() {
	// Register MCP protocol methods
	s.RegisterTool("tools/list", s.handleToolsList)

	// Register device tools
	s.RegisterTool("list_devices", s.handleListDevices)
	s.RegisterTool("select_device", s.handleSelectDevice)
	s.RegisterTool("get_device_status", s.handleGetDeviceStatus)

	// Register file tools
	s.RegisterTool("list_files", s.handleListFiles)
	s.RegisterTool("check_path", s.handleCheckPath)
	s.RegisterTool("get_download_link", s.handleGetDownloadLink)

	s.logger.Info("MCP tools registered", zap.Int("count", len(s.tools)))
}

// MCP protocol handlers

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	tools := []map[string]interface{}{
		{
			"name":        "list_devices",
			"description": "列出所有在线设备",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"device_name_filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional device name filter (fuzzy matching, e.g., '扬州')",
					},
				},
			},
		},
		{
			"name":        "select_device",
			"description": "选择要操作的设备",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"device_name": map[string]interface{}{
						"type":        "string",
						"description": "设备名称（支持模糊匹配）",
					},
					"device_id": map[string]interface{}{
						"type":        "string",
						"description": "设备ID",
					},
				},
			},
		},
		{
			"name":        "get_device_status",
			"description": "获取设备状态",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"device_id": map[string]interface{}{
						"type":        "string",
						"description": "设备ID（可选）",
					},
				},
			},
		},
		{
			"name":        "list_files",
			"description": "列出目录内容",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "目录路径",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "check_path",
			"description": "检查路径是否存在",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "文件或目录路径",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "get_download_link",
			"description": "获取下载链接",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"paths": map[string]interface{}{
						"type":        "array",
						"items":       map[string]string{"type": "string"},
						"description": "文件路径列表",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "描述（可选）",
					},
				},
				"required": []string{"paths"},
			},
		},
	}

	return map[string]interface{}{
		"tools": tools,
	}, nil
}

// Device tool handlers

func (s *Server) handleListDevices(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	// Parse parameters to get optional filter
	var p struct {
		DeviceNameFilter string `json:"device_name_filter"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
		}
	}

	devices, err := s.registry.ListOnline()
	if err != nil {
		return nil, NewMCPError(ErrorCodeInternalError, "Failed to list devices", nil)
	}

	result := make([]DeviceInfo, 0, len(devices))
	for _, device := range devices {
		// Apply filter if provided
		if p.DeviceNameFilter != "" {
			filterLower := strings.ToLower(p.DeviceNameFilter)
			deviceNameLower := strings.ToLower(device.DeviceName)
			if !strings.Contains(deviceNameLower, filterLower) {
				continue
			}
		}

		result = append(result, DeviceInfo{
			DeviceID:     device.DeviceID,
			DeviceName:   device.DeviceName,
			Platform:     string(device.Platform),
			IP:           device.IP,
			AllowedRoots: rootsToStrings(device.AllowedRoots),
			Status:       string(device.Status),
			LastSeen:     device.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	s.logger.Info("Listed devices",
		zap.Int("count", len(result)),
		zap.String("filter", p.DeviceNameFilter))
	return result, nil
}

func (s *Server) handleSelectDevice(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	var p SelectDeviceParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
	}

	var device *devices.Device
	var err error

	if p.DeviceID != "" {
		device, err = s.registry.Get(p.DeviceID)
		if err != nil {
			return nil, ErrDeviceNotFound(p.DeviceID)
		}
	} else if p.DeviceName != "" {
		device = s.findDeviceByName(p.DeviceName)
		if device == nil {
			return nil, ErrDeviceNotFound(p.DeviceName)
		}
	} else {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Either device_name or device_id is required", nil)
	}

	if device.Status != devices.DeviceStatusOnline {
		return nil, ErrDeviceOffline(
			device.DeviceID,
			device.DeviceName,
			device.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
		)
	}

	session.SetSelectedDevice(device)

	s.logger.Info("Device selected",
		zap.String("device_id", device.DeviceID),
		zap.String("device_name", device.DeviceName))

	return DeviceInfo{
		DeviceID:     device.DeviceID,
		DeviceName:   device.DeviceName,
		Platform:     string(device.Platform),
		IP:           device.IP,
		AllowedRoots: rootsToStrings(device.AllowedRoots),
		Status:       string(device.Status),
		LastSeen:     device.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *Server) handleGetDeviceStatus(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	var p GetDeviceStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
	}

	var device *devices.Device
	var err error

	if p.DeviceID != "" {
		device, err = s.registry.Get(p.DeviceID)
		if err != nil {
			return nil, ErrDeviceNotFound(p.DeviceID)
		}
	} else {
		device = session.GetSelectedDevice()
		if device == nil {
			return nil, NewMCPError(ErrorCodeInvalidParams, "No device selected", nil)
		}
	}

	return DeviceStatusResponse{
		DeviceID:   device.DeviceID,
		DeviceName: device.DeviceName,
		Status:     string(device.Status),
		LastSeen:   device.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *Server) findDeviceByName(name string) *devices.Device {
	devices, err := s.registry.ListOnline()
	if err != nil {
		return nil
	}

	for _, device := range devices {
		if device.DeviceName == name {
			return device
		}
	}

	nameLower := strings.ToLower(name)
	for _, device := range devices {
		if strings.ToLower(device.DeviceName) == nameLower {
			return device
		}
	}

	for _, device := range devices {
		if strings.Contains(strings.ToLower(device.DeviceName), nameLower) {
			return device
		}
	}

	return nil
}

// File tool handlers

func (s *Server) handleListFiles(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	var p ListFilesParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
	}

	device := session.GetSelectedDevice()
	if device == nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "No device selected", nil)
	}

	if !s.isPathAllowed(p.Path, rootsToStrings(device.AllowedRoots)) {
		return nil, ErrPathNotAllowed(p.Path)
	}

	listReq := rpc.ListRequest{Path: p.Path}
	reqPayload, _ := json.Marshal(listReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := s.rpcManager.Call(rpcCtx, device.DeviceID, "list", reqPayload, 10*time.Second)
	if err != nil {
		s.logger.Error("List RPC failed", zap.Error(err))
		return nil, ErrRPCTimeout(10)
	}

	if !resp.Success {
		return nil, NewMCPError(ErrorCodeInternalError, "List failed", resp.Error)
	}

	var listResp rpc.ListResponse
	if err := json.Unmarshal(resp.Payload, &listResp); err != nil {
		return nil, NewMCPError(ErrorCodeInternalError, "Failed to parse response", nil)
	}

	result := make([]FileInfo, 0, len(listResp.Entries))
	for _, entry := range listResp.Entries {
		result = append(result, FileInfo{
			Name:         entry.Name,
			Path:         entry.Path,
			IsDir:        entry.IsDir,
			Size:         entry.Size,
			ModifiedTime: time.Unix(entry.ModifiedTime, 0).Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return result, nil
}

func (s *Server) handleCheckPath(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	var p CheckPathParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
	}

	device := session.GetSelectedDevice()
	if device == nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "No device selected", nil)
	}

	if !s.isPathAllowed(p.Path, rootsToStrings(device.AllowedRoots)) {
		return nil, ErrPathNotAllowed(p.Path)
	}

	fileInfoReq := map[string]string{"path": p.Path}
	reqPayload, _ := json.Marshal(fileInfoReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.rpcManager.Call(rpcCtx, device.DeviceID, "file_info", reqPayload, 30*time.Second)
	if err != nil {
		s.logger.Error("file_info RPC failed", zap.Error(err))
		return nil, ErrRPCTimeout(30)
	}

	if !resp.Success {
		return PathInfo{Exists: false}, nil
	}

	var fileInfo rpc.FileInfo
	if err := json.Unmarshal(resp.Payload, &fileInfo); err != nil {
		return nil, NewMCPError(ErrorCodeInternalError, "Failed to parse response", nil)
	}

	return PathInfo{
		Exists:       true,
		IsFile:       !fileInfo.IsDir,
		IsDir:        fileInfo.IsDir,
		Size:         fileInfo.Size,
		ModifiedTime: time.Unix(fileInfo.ModifiedTime, 0).Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *Server) handleGetDownloadLink(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error) {
	var p GetDownloadLinkParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "Invalid parameters", nil)
	}

	device := session.GetSelectedDevice()
	if device == nil {
		return nil, NewMCPError(ErrorCodeInvalidParams, "No device selected", nil)
	}

	for _, path := range p.Paths {
		if !s.isPathAllowed(path, rootsToStrings(device.AllowedRoots)) {
			return nil, ErrPathNotAllowed(path)
		}
	}

	if device.Status != devices.DeviceStatusOnline {
		return nil, ErrDeviceOffline(
			device.DeviceID,
			device.DeviceName,
			device.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
		)
	}

	var objectID string
	var fileSize int64
	var compressed bool
	var err error

	if len(p.Paths) == 1 {
		fileInfo, err := s.getFileInfo(ctx, device.DeviceID, p.Paths[0])
		if err != nil {
			return nil, fmt.Errorf("failed to check path: %w", err)
		}

		if !fileInfo.IsDir {
			objectID, fileSize, err = s.uploadFile(ctx, device.DeviceID, p.Paths[0])
			compressed = false
		} else {
			// Check if directory is empty before compressing
			isEmpty, checkErr := s.isDirectoryEmpty(ctx, device.DeviceID, p.Paths[0])
			if checkErr == nil && isEmpty {
				s.logger.Info("Directory is empty, skipping compression", zap.String("path", p.Paths[0]))
				// Return empty file size to signal that directory is empty
				return DownloadLinkResponse{
					DownloadURL: "",
					FileName:    fmt.Sprintf("%s-%s.zip", device.DeviceName, p.Description),
					FileSize:    0,
					ExpiresAt:   time.Now().Add(10 * time.Minute).Format("2006-01-02T15:04:05Z07:00"),
					Paths:       p.Paths,
					Compressed:  true,
				}, nil
			}
			// Directory has files, proceed with compression
			objectID, fileSize, err = s.compressAndUpload(ctx, device.DeviceID, p.Paths)
			compressed = true
		}
	} else {
		objectID, fileSize, err = s.compressAndUpload(ctx, device.DeviceID, p.Paths)
		compressed = true
	}

	if err != nil {
		s.logger.Error("Failed to get download link", zap.Error(err))
		return nil, NewMCPError(ErrorCodeInternalError, err.Error(), nil)
	}

	token, err := s.tokenManager.GenerateToken(objectID, 24*time.Hour)
	if err != nil {
		return nil, NewMCPError(ErrorCodeInternalError, "Failed to generate token", nil)
	}

	downloadURL := fmt.Sprintf("http://%s/api/objects/download/%s", s.serverAddr, token.Token)

	fileName := fmt.Sprintf("%s-files-%s", device.DeviceName, time.Now().Format("20060102-150405"))
	if p.Description != "" {
		fileName = fmt.Sprintf("%s-%s", device.DeviceName, p.Description)
	}
	if compressed {
		fileName += ".zip"
	} else {
		// For non-compressed files, preserve the original file extension
		if len(p.Paths) == 1 {
			ext := filepath.Ext(p.Paths[0])
			if ext != "" {
				fileName += ext
			}
		}
	}

	result := DownloadLinkResponse{
		DownloadURL: downloadURL,
		FileName:    fileName,
		FileSize:    fileSize,
		ExpiresAt:   token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		Paths:       p.Paths,
		Compressed:  compressed,
	}

	s.logger.Info("Download link generated",
		zap.String("object_id", objectID),
		zap.String("file_name", fileName),
		zap.Bool("compressed", compressed))

	return result, nil
}

// Helper methods

func (s *Server) getFileInfo(ctx context.Context, deviceID, path string) (*rpc.FileInfo, error) {
	fileInfoReq := map[string]string{"path": path}
	reqPayload, _ := json.Marshal(fileInfoReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := s.rpcManager.Call(rpcCtx, deviceID, "file_info", reqPayload, 10*time.Second)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("file_info failed: %v", resp.Error)
	}

	var fileInfo rpc.FileInfo
	if err := json.Unmarshal(resp.Payload, &fileInfo); err != nil {
		return nil, err
	}

	return &fileInfo, nil
}

// isDirectoryEmpty checks if a directory has any files (recursively)
func (s *Server) isDirectoryEmpty(ctx context.Context, deviceID, path string) (bool, error) {
	listReq := rpc.ListRequest{Path: path}
	reqPayload, _ := json.Marshal(listReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := s.rpcManager.Call(rpcCtx, deviceID, "list", reqPayload, 10*time.Second)
	if err != nil {
		// If list fails, assume directory is not empty to be safe
		s.logger.Warn("Failed to list directory, assuming not empty", zap.String("path", path), zap.Error(err))
		return false, err
	}

	if !resp.Success {
		// If list fails, assume directory is not empty to be safe
		s.logger.Warn("List RPC failed, assuming directory not empty", zap.String("path", path))
		return false, fmt.Errorf("list failed: %v", resp.Error)
	}

	var listResp rpc.ListResponse
	if err := json.Unmarshal(resp.Payload, &listResp); err != nil {
		// If parsing fails, assume directory is not empty to be safe
		s.logger.Warn("Failed to parse list response, assuming not empty", zap.String("path", path), zap.Error(err))
		return false, err
	}

	// Directory is empty if no entries or only empty subdirectories
	if len(listResp.Entries) == 0 {
		return true, nil
	}

	// Check if there are any non-directory entries (files)
	for _, entry := range listResp.Entries {
		if !entry.IsDir {
			// Found a file, directory is not empty
			return false, nil
		}
		// For directories, recursively check if they contain files
		isEmpty, err := s.isDirectoryEmpty(ctx, deviceID, entry.Path)
		if err != nil {
			// If recursive check fails, assume directory is not empty to be safe
			s.logger.Warn("Failed to recursively check subdirectory, assuming not empty",
				zap.String("path", entry.Path), zap.Error(err))
			return false, err
		}
		if !isEmpty {
			// Subdirectory has files
			return false, nil
		}
	}

	// All entries are empty directories
	return true, nil
}

func (s *Server) uploadFile(ctx context.Context, deviceID, path string) (string, int64, error) {
	// Step 1: Get file info from device
	fileInfo, err := s.getFileInfo(ctx, deviceID, path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get file info: %w", err)
	}

	// Step 2: Initialize upload in database
	initReq := &objects.InitUploadRequest{
		DeviceID:   deviceID,
		SourcePath: path,
		FileName:   fileInfo.Name,
		FileSize:   fileInfo.Size,
		SHA256:     "", // SHA256 will be calculated during upload
	}

	obj, err := s.storage.InitUpload(initReq)
	if err != nil {
		return "", 0, fmt.Errorf("failed to initialize upload: %w", err)
	}

	// Step 3: Generate upload URL
	uploadURL := fmt.Sprintf("http://%s/api/objects/upload/%s", s.serverAddr, obj.ObjectID)

	// Step 4: Send upload request to device with upload_url
	uploadReq := rpc.UploadRequest{
		ObjectID:  obj.ObjectID,
		Path:      path,
		UploadURL: uploadURL,
		SHA256:    "", // SHA256 will be calculated during upload
	}
	reqPayload, _ := json.Marshal(uploadReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	resp, err := s.rpcManager.Call(rpcCtx, deviceID, "upload", reqPayload, 10*time.Minute)
	if err != nil {
		s.storage.UpdateObjectStatus(obj.ObjectID, objects.StatusFailed)
		return "", 0, fmt.Errorf("upload request failed: %w", err)
	}

	if !resp.Success {
		s.storage.UpdateObjectStatus(obj.ObjectID, objects.StatusFailed)
		return "", 0, fmt.Errorf("upload failed: %v", resp.Error)
	}

	// Step 5: Wait for upload completion
	completedObj, err := s.waitForObjectCompletion(obj.ObjectID, 10*time.Minute)
	if err != nil {
		return "", 0, err
	}

	return obj.ObjectID, completedObj.FileSize, nil
}

func (s *Server) compressAndUpload(ctx context.Context, deviceID string, paths []string) (string, int64, error) {
	if len(paths) != 1 {
		return "", 0, fmt.Errorf("compress only supports single path")
	}

	// Use compress method instead of zip (compatible with Flutter agent)
	compressReq := map[string]string{
		"path": paths[0],
	}
	reqPayload, _ := json.Marshal(compressReq)

	rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Call compress instead of zip
	resp, err := s.rpcManager.Call(rpcCtx, deviceID, "compress", reqPayload, 5*time.Minute)
	if err != nil {
		return "", 0, fmt.Errorf("compress request failed: %w", err)
	}

	if !resp.Success {
		return "", 0, fmt.Errorf("compress failed: %v", resp.Error)
	}

	var compressResp map[string]interface{}
	if err := json.Unmarshal(resp.Payload, &compressResp); err != nil {
		return "", 0, fmt.Errorf("failed to parse compress response: %w", err)
	}

	// Get compressed file path
	compressedPath, ok := compressResp["zip_path"].(string)
	if !ok {
		return "", 0, fmt.Errorf("compress response missing zip_path")
	}

	// Upload the compressed file
	objectID, fileSize, err := s.uploadFile(ctx, deviceID, compressedPath)
	if err != nil {
		return "", 0, err
	}

	return objectID, fileSize, nil
}

func (s *Server) waitForObjectCompletion(objectID string, timeout time.Duration) (*objects.UploadedObject, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		obj, err := s.storage.GetObject(objectID)
		if err == nil && obj.Status == objects.StatusCompleted {
			return obj, nil
		}

		if err == nil && obj.Status == objects.StatusFailed {
			return nil, fmt.Errorf("upload failed")
		}

		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("upload timeout")
}

func (s *Server) isPathAllowed(path string, allowedRoots []string) bool {
	if strings.Contains(path, "..") {
		return false
	}

	path = filepath.Clean(path)

	for _, root := range allowedRoots {
		root = filepath.Clean(root)
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// rootsToStrings converts []devices.Root to []string by extracting AbsPath
func rootsToStrings(roots []devices.Root) []string {
	result := make([]string, len(roots))
	for i, root := range roots {
		result[i] = root.AbsPath
	}
	return result
}
