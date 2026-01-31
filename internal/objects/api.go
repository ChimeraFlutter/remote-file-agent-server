package objects

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/remote-file-manager/server/internal/audit"
	"go.uber.org/zap"
)

// API handles object storage API requests
type API struct {
	storage         *Storage
	tokenManager    *TokenManager
	uploadHandler   *UploadHandler
	downloadHandler *DownloadHandler
	zipService      *ZipService
	logger          *zap.Logger
	auditor         *audit.Logger
	serverURL       string
}

// NewAPI creates a new objects API instance
func NewAPI(storage *Storage, tokenManager *TokenManager, uploadHandler *UploadHandler, zipService *ZipService, logger *zap.Logger, auditor *audit.Logger, serverURL string) *API {
	return &API{
		storage:         storage,
		tokenManager:    tokenManager,
		uploadHandler:   uploadHandler,
		downloadHandler: NewDownloadHandler(storage, logger),
		zipService:      zipService,
		logger:          logger,
		auditor:         auditor,
		serverURL:       serverURL,
	}
}

// HandleInitUpload handles upload initialization requests
func (api *API) HandleInitUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req InitUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if err := ValidateUploadRequest(&req); err != nil {
		api.logger.Warn("Invalid upload request", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Initialize upload
	obj, err := api.storage.InitUpload(&req)
	if err != nil {
		api.logger.Error("Failed to initialize upload", zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	// Generate upload URL
	uploadURL := fmt.Sprintf("%s/upload/%s", api.serverURL, obj.ObjectID)

	// Create response
	resp := InitUploadResponse{
		ObjectID:  obj.ObjectID,
		UploadURL: uploadURL,
	}

	// Log audit
	api.auditor.Log("admin", "init_upload", req.DeviceID, obj.ObjectID, "success", "", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleUpload handles file upload requests
func (api *API) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get object ID from URL
	vars := mux.Vars(r)
	objectID := vars["object_id"]

	// Verify object exists
	obj, err := api.storage.GetObject(objectID)
	if err != nil {
		api.logger.Warn("Object not found", zap.String("object_id", objectID))
		http.Error(w, `{"error":"object_not_found"}`, http.StatusNotFound)
		return
	}

	// Handle upload
	if err := api.uploadHandler.HandleUpload(objectID, r.Body); err != nil {
		api.logger.Error("Upload failed",
			zap.String("object_id", objectID),
			zap.Error(err))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Log audit
	api.auditor.Log("device", "upload", obj.DeviceID, objectID, "success", "", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"object_id": objectID,
	})
}

// HandleGetObject handles object info requests
func (api *API) HandleGetObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get object ID from URL
	vars := mux.Vars(r)
	objectID := vars["object_id"]

	// Get object
	obj, err := api.storage.GetObject(objectID)
	if err != nil {
		api.logger.Warn("Object not found", zap.String("object_id", objectID))
		http.Error(w, `{"error":"object_not_found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

// HandleListObjects handles object listing requests
func (api *API) HandleListObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	deviceID := r.URL.Query().Get("device_id")

	// List objects
	objects, err := api.storage.ListObjects(deviceID, 100, 0)
	if err != nil {
		api.logger.Error("Failed to list objects", zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(objects)
}

// HandleGenerateDownloadToken handles download token generation requests
func (api *API) HandleGenerateDownloadToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get object ID from URL
	vars := mux.Vars(r)
	objectID := vars["object_id"]

	// Verify object exists and is completed
	obj, err := api.storage.GetObject(objectID)
	if err != nil {
		api.logger.Warn("Object not found", zap.String("object_id", objectID))
		http.Error(w, `{"error":"object_not_found"}`, http.StatusNotFound)
		return
	}

	if obj.Status != StatusCompleted {
		http.Error(w, `{"error":"object_not_completed"}`, http.StatusBadRequest)
		return
	}

	// Generate token (valid for 10 minutes)
	token, err := api.tokenManager.GenerateToken(objectID, 10*time.Minute)
	if err != nil {
		api.logger.Error("Failed to generate token", zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	// Generate download URL
	downloadURL := fmt.Sprintf("%s/download/%s", api.serverURL, token.Token)

	// Log audit
	api.auditor.Log("admin", "generate_token", obj.DeviceID, objectID, "success", "", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":        token.Token,
		"download_url": downloadURL,
		"expires_at":   token.ExpiresAt,
	})
}

// HandleDownload handles file download requests
func (api *API) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get token from URL
	vars := mux.Vars(r)
	tokenStr := vars["token"]

	// Validate token
	token, err := api.tokenManager.ValidateToken(tokenStr)
	if err != nil {
		api.logger.Warn("Invalid download token", zap.String("token", tokenStr), zap.Error(err))
		http.Error(w, `{"error":"invalid_token"}`, http.StatusForbidden)
		return
	}

	// Get object
	obj, err := api.storage.GetObject(token.ObjectID)
	if err != nil {
		api.logger.Error("Object not found", zap.String("object_id", token.ObjectID))
		http.Error(w, `{"error":"object_not_found"}`, http.StatusNotFound)
		return
	}

	// Use download handler to serve file with Range support
	if err := api.downloadHandler.ServeFile(w, r, obj); err != nil {
		api.logger.Error("Failed to serve file", zap.Error(err))
		return
	}

	// Log audit
	api.auditor.Log("download", "file", obj.DeviceID, obj.ObjectID, "success", "", r.RemoteAddr)
}

// HandleDeleteObject handles object deletion requests
func (api *API) HandleDeleteObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get object ID from URL
	vars := mux.Vars(r)
	objectID := vars["object_id"]

	// Delete object
	if err := api.storage.DeleteObject(objectID); err != nil {
		api.logger.Error("Failed to delete object", zap.String("object_id", objectID), zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	// Log audit
	api.auditor.Log("admin", "delete_object", "", objectID, "success", "", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// ZipRequest represents a zip creation request from admin
type ZipRequest struct {
	DeviceID string   `json:"device_id"`
	Paths    []string `json:"paths"`
}

// HandleZipRequest handles zip creation requests
func (api *API) HandleZipRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req ZipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if req.DeviceID == "" {
		http.Error(w, `{"error":"device_id_required"}`, http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, `{"error":"paths_required"}`, http.StatusBadRequest)
		return
	}

	// Request zip creation
	obj, err := api.zipService.RequestZip(r.Context(), req.DeviceID, req.Paths)
	if err != nil {
		api.logger.Error("Failed to create zip",
			zap.String("device_id", req.DeviceID),
			zap.Strings("paths", req.Paths),
			zap.Error(err))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Log audit
	api.auditor.Log("admin", "zip_request", req.DeviceID, obj.ObjectID, "success", "", r.RemoteAddr)

	// Return object info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}
