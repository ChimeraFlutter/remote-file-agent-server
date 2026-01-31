package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/remote-file-manager/server/internal/audit"
	"github.com/remote-file-manager/server/internal/devices"
	"github.com/remote-file-manager/server/internal/rpc"
	"github.com/remote-file-manager/server/internal/ws"
	"go.uber.org/zap"
)

// API handles admin API requests
type API struct {
	auth       *Auth
	registry   *devices.Registry
	logger     *zap.Logger
	auditor    *audit.Logger
	rpcManager *rpc.Manager
	wsManager  *ws.Manager
}

// NewAPI creates a new API instance
func NewAPI(auth *Auth, registry *devices.Registry, logger *zap.Logger, auditor *audit.Logger, rpcManager *rpc.Manager, wsManager *ws.Manager) *API {
	return &API{
		auth:       auth,
		registry:   registry,
		logger:     logger,
		auditor:    auditor,
		rpcManager: rpcManager,
		wsManager:  wsManager,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// HandleLogin handles admin login
func (api *API) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	// Validate password
	if !api.auth.ValidatePassword(req.Password) {
		api.auditor.Log("admin", "login", "", "", "failure", "invalid_password", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Error:   "invalid_password",
		})
		return
	}

	// Create session
	sessionID, err := api.auth.CreateSession(r.RemoteAddr)
	if err != nil {
		api.logger.Error("Failed to create session", zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600, // 1 hour
	})

	api.auditor.Log("admin", "login", "", "", "success", "", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Success: true,
	})
}

// HandleLogout handles admin logout
func (api *API) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil {
		api.auth.DeleteSession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// DeviceResponse represents a device in API response
type DeviceResponse struct {
	DeviceID     string         `json:"device_id"`
	DeviceName   string         `json:"device_name"`
	Platform     string         `json:"platform"`
	Version      string         `json:"version"`
	IP           string         `json:"ip"`
	LastSeen     time.Time      `json:"last_seen"`
	Status       string         `json:"status"`
	AllowedRoots []devices.Root `json:"allowed_roots"`
}

// HandleDeviceList handles device list requests
func (api *API) HandleDeviceList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get all devices
	deviceList, err := api.registry.List()
	if err != nil {
		api.logger.Error("Failed to list devices", zap.Error(err))
		http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	response := make([]DeviceResponse, 0, len(deviceList))
	for _, device := range deviceList {
		response = append(response, DeviceResponse{
			DeviceID:     device.DeviceID,
			DeviceName:   device.DeviceName,
			Platform:     string(device.Platform),
			Version:      device.Version,
			IP:           device.IP,
			LastSeen:     device.LastSeen,
			Status:       string(device.Status),
			AllowedRoots: device.AllowedRoots,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// RPCRequest represents an RPC request from admin
type RPCRequest struct {
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

// HandleDeviceRPC handles RPC requests to devices
func (api *API) HandleDeviceRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Get device ID from URL
	vars := mux.Vars(r)
	deviceID := vars["id"]

	// Check if device exists
	device, err := api.registry.Get(deviceID)
	if err != nil {
		api.logger.Warn("Device not found", zap.String("device_id", deviceID))
		http.Error(w, `{"error":"device_not_found"}`, http.StatusNotFound)
		return
	}

	// Check if device is online
	if device.Status != devices.DeviceStatusOnline {
		http.Error(w, `{"error":"device_offline"}`, http.StatusServiceUnavailable)
		return
	}

	// Parse request
	var req RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Call RPC
	resp, err := api.rpcManager.Call(ctx, deviceID, req.Method, req.Payload, 30*time.Second)
	if err != nil {
		api.logger.Error("RPC call failed",
			zap.String("device_id", deviceID),
			zap.String("method", req.Method),
			zap.Error(err))

		if err == rpc.ErrTimeout {
			http.Error(w, `{"error":"rpc_timeout"}`, http.StatusGatewayTimeout)
		} else if err == rpc.ErrDeviceOffline {
			http.Error(w, `{"error":"device_offline"}`, http.StatusServiceUnavailable)
		} else {
			http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		}
		return
	}

	// Check if RPC succeeded
	if !resp.Success {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// Log audit
	api.auditor.Log("admin", "rpc", deviceID, req.Method, "success", "", r.RemoteAddr)
}

// DeleteFileRequest represents a file deletion request from admin
type DeleteFileRequest struct {
	DeviceID string `json:"device_id"`
	Path     string `json:"path"`
}

// HandleDeleteFile handles file deletion requests
func (api *API) HandleDeleteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req DeleteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	// Validate request
	if req.DeviceID == "" {
		http.Error(w, `{"error":"device_id_required"}`, http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, `{"error":"path_required"}`, http.StatusBadRequest)
		return
	}

	// Check if device exists and is online
	device, err := api.registry.Get(req.DeviceID)
	if err != nil {
		api.logger.Warn("Device not found", zap.String("device_id", req.DeviceID))
		http.Error(w, `{"error":"device_not_found"}`, http.StatusNotFound)
		return
	}

	if device.Status != devices.DeviceStatusOnline {
		http.Error(w, `{"error":"device_offline"}`, http.StatusServiceUnavailable)
		return
	}

	// Create delete request payload
	deleteReq := map[string]string{
		"path": req.Path,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Call RPC
	resp, err := api.rpcManager.Call(ctx, req.DeviceID, "delete", deleteReq, 30*time.Second)
	if err != nil {
		api.logger.Error("Delete RPC call failed",
			zap.String("device_id", req.DeviceID),
			zap.String("path", req.Path),
			zap.Error(err))

		if err == rpc.ErrTimeout {
			http.Error(w, `{"error":"rpc_timeout"}`, http.StatusGatewayTimeout)
		} else if err == rpc.ErrDeviceOffline {
			http.Error(w, `{"error":"device_offline"}`, http.StatusServiceUnavailable)
		} else {
			http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
		}
		return
	}

	// Check if RPC succeeded
	if !resp.Success {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Log audit
	api.auditor.Log("admin", "delete_file", req.DeviceID, req.Path, "success", "", r.RemoteAddr)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
