package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// DefaultTimeout is the default RPC timeout
	DefaultTimeout = 30 * time.Second
)

var (
	// ErrTimeout is returned when an RPC times out
	ErrTimeout = errors.New("rpc timeout")
	// ErrDeviceOffline is returned when the device is offline
	ErrDeviceOffline = errors.New("device offline")
	// ErrCancelled is returned when the RPC is cancelled
	ErrCancelled = errors.New("rpc cancelled")
)

// PendingRequest represents a pending RPC request
type PendingRequest struct {
	ReqID      string
	DeviceID   string
	Method     string
	Payload    json.RawMessage
	ResponseCh chan *Response
	CreatedAt  time.Time
	Timeout    time.Duration
	CancelFunc context.CancelFunc
}

// Manager manages RPC requests and responses
type Manager struct {
	pending   map[string]*PendingRequest
	mu        sync.RWMutex
	logger    *zap.Logger
	wsManager interface{} // WebSocket manager to send messages
}

// NewManager creates a new RPC manager
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		pending: make(map[string]*PendingRequest),
		logger:  logger,
	}
}

// SetWSManager sets the WebSocket manager
func (m *Manager) SetWSManager(wsManager interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wsManager = wsManager
}

// Call sends an RPC request and waits for the response
func (m *Manager) Call(ctx context.Context, deviceID, method string, payload interface{}, timeout time.Duration) (*Response, error) {
	// Generate request ID
	reqID := uuid.New().String()

	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Create context with timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)

	// Create pending request
	req := &PendingRequest{
		ReqID:      reqID,
		DeviceID:   deviceID,
		Method:     method,
		Payload:    payloadBytes,
		ResponseCh: make(chan *Response, 1),
		CreatedAt:  time.Now(),
		Timeout:    timeout,
		CancelFunc: cancel,
	}

	// Register pending request
	m.mu.Lock()
	m.pending[reqID] = req
	m.mu.Unlock()

	m.logger.Debug("RPC call initiated",
		zap.String("req_id", reqID),
		zap.String("device_id", deviceID),
		zap.String("method", method),
		zap.Duration("timeout", timeout))

	// Send message to device via WebSocket
	if err := m.sendToDevice(deviceID, reqID, method, payloadBytes); err != nil {
		m.mu.Lock()
		delete(m.pending, reqID)
		m.mu.Unlock()
		cancel()
		return nil, err
	}

	// Wait for response or timeout
	select {
	case resp := <-req.ResponseCh:
		m.logger.Debug("RPC call completed",
			zap.String("req_id", reqID),
			zap.Bool("success", resp.Success))
		return resp, nil
	case <-ctx.Done():
		// Clean up
		m.mu.Lock()
		delete(m.pending, reqID)
		m.mu.Unlock()

		if ctx.Err() == context.DeadlineExceeded {
			m.logger.Warn("RPC call timeout",
				zap.String("req_id", reqID),
				zap.String("device_id", deviceID),
				zap.String("method", method))
			return nil, ErrTimeout
		}
		return nil, ErrCancelled
	}
}

// sendToDevice sends a message to a device via WebSocket
func (m *Manager) sendToDevice(deviceID, reqID, method string, payload json.RawMessage) error {
	m.mu.RLock()
	wsManager := m.wsManager
	m.mu.RUnlock()

	if wsManager == nil {
		return errors.New("WebSocket manager not set")
	}

	// Import ws package types
	// We need to create the envelope in the correct format
	type WSManager interface {
		SendToDevice(deviceID string, envelope interface{}) error
	}

	// Type assert to ws.Manager
	type Envelope struct {
		Type      string          `json:"type"`
		ReqID     string          `json:"req_id"`
		Timestamp int64           `json:"ts"`
		DeviceID  string          `json:"device_id"`
		Payload   json.RawMessage `json:"payload"`
	}

	// Create message type based on method
	var messageType string
	switch method {
	case "list":
		messageType = "list_req"
	case "delete":
		messageType = "delete_req"
	case "zip":
		messageType = "zip_req"
	case "upload":
		messageType = "upload_req"
	default:
		messageType = method + "_req"
	}

	// Create envelope
	envelope := &Envelope{
		Type:      messageType,
		ReqID:     reqID,
		Timestamp: time.Now().Unix(),
		DeviceID:  deviceID,
		Payload:   payload,
	}

	// Call SendToDevice using reflection-free approach
	// Cast to the actual ws.Manager type
	if wsMgr, ok := wsManager.(interface {
		SendToDevice(deviceID string, envelope interface{}) error
	}); ok {
		return wsMgr.SendToDevice(deviceID, envelope)
	}

	return errors.New("invalid WebSocket manager")
}

// HandleResponse handles an RPC response from a device
func (m *Manager) HandleResponse(reqID string, respData interface{}) error {
	m.mu.Lock()
	req, exists := m.pending[reqID]
	if !exists {
		m.mu.Unlock()
		m.logger.Warn("Received response for unknown request",
			zap.String("req_id", reqID))
		return errors.New("unknown request ID")
	}
	delete(m.pending, reqID)
	m.mu.Unlock()

	// Convert response data to Response struct
	var resp *Response
	switch v := respData.(type) {
	case *Response:
		resp = v
	case map[string]interface{}:
		// Convert map to Response
		resp = &Response{
			ReqID:     reqID,
			Success:   true,
			Timestamp: time.Now(),
		}
		if payload, ok := v["payload"].(json.RawMessage); ok {
			resp.Payload = payload
		}
		if success, ok := v["success"].(bool); ok {
			resp.Success = success
		}
	default:
		// Try to marshal and unmarshal
		data, err := json.Marshal(respData)
		if err != nil {
			return err
		}
		resp = &Response{}
		if err := json.Unmarshal(data, resp); err != nil {
			return err
		}
	}

	// Send response BEFORE cancelling context to avoid race condition
	// where ctx.Done() wins the select race in Call()
	select {
	case req.ResponseCh <- resp:
		// Cancel the timeout after successfully sending response
		if req.CancelFunc != nil {
			req.CancelFunc()
		}
		return nil
	default:
		m.logger.Warn("Response channel full or closed",
			zap.String("req_id", reqID))
		return errors.New("response channel unavailable")
	}
}

// GetPendingRequest retrieves a pending request by ID
func (m *Manager) GetPendingRequest(reqID string) (*PendingRequest, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	req, exists := m.pending[reqID]
	return req, exists
}

// CancelRequest cancels a pending request
func (m *Manager) CancelRequest(reqID string) {
	m.mu.Lock()
	req, exists := m.pending[reqID]
	if exists {
		delete(m.pending, reqID)
	}
	m.mu.Unlock()

	if exists && req.CancelFunc != nil {
		req.CancelFunc()
	}
}

// CleanupExpired removes expired pending requests
func (m *Manager) CleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for reqID, req := range m.pending {
		if now.Sub(req.CreatedAt) > req.Timeout {
			m.logger.Warn("Cleaning up expired RPC request",
				zap.String("req_id", reqID),
				zap.String("device_id", req.DeviceID),
				zap.String("method", req.Method))

			if req.CancelFunc != nil {
				req.CancelFunc()
			}
			delete(m.pending, reqID)
		}
	}
}

// GetPendingCount returns the number of pending requests
func (m *Manager) GetPendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}

// GetPendingRequestsForDevice returns all pending requests for a device
func (m *Manager) GetPendingRequestsForDevice(deviceID string) []*PendingRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var requests []*PendingRequest
	for _, req := range m.pending {
		if req.DeviceID == deviceID {
			requests = append(requests, req)
		}
	}
	return requests
}
