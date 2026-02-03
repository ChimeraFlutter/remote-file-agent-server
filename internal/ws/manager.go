package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/remote-file-manager/server/internal/devices"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	// Keep at 10MB - large files should use HTTP upload
	maxMessageSize = 10 * 1024 * 1024 // 10MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now (should be restricted in production)
		return true
	},
}

// Connection represents a WebSocket connection
type Connection struct {
	conn     *websocket.Conn
	send     chan []byte
	manager  *Manager
	deviceID string
	mu       sync.Mutex
}

// Manager manages all WebSocket connections
type Manager struct {
	registry       *devices.Registry
	connections    map[string]*Connection // deviceID -> connection
	mu             sync.RWMutex
	enrollToken    string
	rpcManager     interface{}    // Will be set later to avoid circular dependency
	adminNotifier  AdminNotifier  // Admin WebSocket manager for notifications
}

// NewManager creates a new WebSocket manager
func NewManager(registry *devices.Registry, enrollToken string) *Manager {
	return &Manager{
		registry:    registry,
		connections: make(map[string]*Connection),
		enrollToken: enrollToken,
	}
}

// SetAdminNotifier sets the admin WebSocket manager for notifications
func (m *Manager) SetAdminNotifier(notifier AdminNotifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adminNotifier = notifier
}

// notifyAdminDeviceUpdate notifies admin clients about device updates
func (m *Manager) notifyAdminDeviceUpdate(device *devices.Device, event string) {
	log.Printf("DEBUG: notifyAdminDeviceUpdate called - device: %s, event: %s", device.DeviceName, event)
	m.mu.RLock()
	notifier := m.adminNotifier
	m.mu.RUnlock()

	if notifier == nil {
		log.Printf("DEBUG: adminNotifier is nil, skipping notification")
		return
	}

	// Type assert to interface with BroadcastDeviceUpdate method
	type AdminNotifier interface {
		BroadcastDeviceUpdate(device *devices.Device, event string)
	}

	if n, ok := notifier.(AdminNotifier); ok {
		log.Printf("DEBUG: Broadcasting device update to admin clients")
		n.BroadcastDeviceUpdate(device, event)
	} else {
		log.Printf("DEBUG: adminNotifier does not implement AdminNotifier interface")
	}
}

// HandleAgentConnection handles incoming WebSocket connections from agents
func (m *Manager) HandleAgentConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	c := &Connection{
		conn:    conn,
		send:    make(chan []byte, 256),
		manager: m,
	}

	// Start goroutines for reading and writing
	go c.writePump()
	go c.readPump()
}

// readPump pumps messages from the WebSocket connection to the manager
func (c *Connection) readPump() {
	defer func() {
		c.close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse envelope
		var envelope Envelope
		if err := json.Unmarshal(message, &envelope); err != nil {
			log.Printf("Failed to parse message: %v", err)
			c.sendError(ErrInternalError, "Invalid message format", "")
			continue
		}

		// Handle message
		if err := c.handleMessage(&envelope); err != nil {
			log.Printf("Failed to handle message: %v", err)
		}
	}
}

// writePump pumps messages from the manager to the WebSocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (c *Connection) handleMessage(envelope *Envelope) error {
	switch envelope.Type {
	case MsgTypeHello:
		return c.handleHello(envelope)
	case MsgTypeHeartbeat:
		return c.handleHeartbeat(envelope)
	case MsgTypeListResp, MsgTypeDeleteResp, MsgTypeZipResp, MsgTypeCompressResp, MsgTypeUploadResp, MsgTypeFileInfoResp:
		return c.handleRPCResponse(envelope)
	case MsgTypeProgress:
		return c.handleProgress(envelope)
	case MsgTypeError:
		return c.handleErrorResponse(envelope)
	default:
		// Other message types will be handled in future tasks
		log.Printf("Unhandled message type: %s", envelope.Type)
	}
	return nil
}

// handleHello processes hello messages from agents
func (c *Connection) handleHello(envelope *Envelope) error {
	log.Printf("DEBUG: Received hello message from connection")
	var payload HelloPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return c.sendError(ErrInternalError, "Invalid hello payload", envelope.ReqID)
	}
	log.Printf("DEBUG: Hello payload - DeviceID: %s, DeviceName: %s", payload.DeviceID, payload.DeviceName)

	// Verify enroll token
	if payload.EnrollToken != c.manager.enrollToken {
		c.sendError(ErrAgentEnrollTokenInvalid, "Invalid enroll token", envelope.ReqID)
		c.close()
		return fmt.Errorf("invalid enroll token")
	}

	// Check if device already has a different connection
	c.manager.mu.Lock()
	if existingConn, exists := c.manager.connections[payload.DeviceID]; exists && existingConn != c {
		// Close existing connection (different connection from same device)
		existingConn.close()
	}
	c.manager.mu.Unlock()

	// Register device
	device, isNew, err := c.manager.registry.Register(
		payload.DeviceID,
		payload.DeviceName,
		payload.Platform,
		payload.Version,
		payload.AllowedRoots,
		c.conn.RemoteAddr().String(),
	)
	if err != nil {
		return c.sendError(ErrInternalError, fmt.Sprintf("Failed to register device: %v", err), envelope.ReqID)
	}

	// Set device ID and register connection
	c.deviceID = payload.DeviceID
	c.manager.mu.Lock()
	c.manager.connections[payload.DeviceID] = c
	c.manager.mu.Unlock()

	// Update registry connection
	if err := c.manager.registry.SetConnection(payload.DeviceID, payload.DeviceID); err != nil {
		log.Printf("Failed to set connection in registry: %v", err)
	}

	log.Printf("Device registered: %s (%s)", device.DeviceName, device.DeviceID)

	// Notify admin clients about device connection/update
	if isNew {
		c.manager.notifyAdminDeviceUpdate(device, "connected")
	} else {
		c.manager.notifyAdminDeviceUpdate(device, "updated")
	}

	// Send hello_ack
	return c.sendHelloAck(true, "Connected successfully", envelope.ReqID)
}

// handleHeartbeat processes heartbeat messages
func (c *Connection) handleHeartbeat(envelope *Envelope) error {
	if c.deviceID == "" {
		return fmt.Errorf("device not authenticated")
	}

	// Update last seen timestamp
	if err := c.manager.registry.UpdateHeartbeat(c.deviceID); err != nil {
		log.Printf("Failed to update heartbeat: %v", err)
	}

	return nil
}

// handleRPCResponse processes RPC response messages
func (c *Connection) handleRPCResponse(envelope *Envelope) error {
	if c.deviceID == "" {
		return fmt.Errorf("device not authenticated")
	}

	// Get RPC manager
	rpcMgr := c.manager.GetRPCManager()
	if rpcMgr == nil {
		log.Printf("RPC manager not set")
		return fmt.Errorf("rpc manager not available")
	}

	// Type assert to RPC manager interface
	type RPCManager interface {
		HandleResponse(reqID string, resp interface{}) error
	}

	mgr, ok := rpcMgr.(RPCManager)
	if !ok {
		log.Printf("Invalid RPC manager type")
		return fmt.Errorf("invalid rpc manager")
	}

	// Create response object
	resp := map[string]interface{}{
		"req_id":    envelope.ReqID,
		"success":   true,
		"payload":   envelope.Payload,
		"timestamp": time.Now(),
	}

	// Handle response
	return mgr.HandleResponse(envelope.ReqID, resp)
}

// handleErrorResponse processes error messages from agents
func (c *Connection) handleErrorResponse(envelope *Envelope) error {
	if c.deviceID == "" {
		return fmt.Errorf("device not authenticated")
	}

	// Get RPC manager
	rpcMgr := c.manager.GetRPCManager()
	if rpcMgr == nil {
		log.Printf("RPC manager not set")
		return fmt.Errorf("rpc manager not available")
	}

	// Type assert to RPC manager interface
	type RPCManager interface {
		HandleResponse(reqID string, resp interface{}) error
	}

	mgr, ok := rpcMgr.(RPCManager)
	if !ok {
		log.Printf("Invalid RPC manager type")
		return fmt.Errorf("invalid rpc manager")
	}

	// Parse error payload
	var errorPayload ErrorPayload
	if err := json.Unmarshal(envelope.Payload, &errorPayload); err != nil {
		log.Printf("Failed to parse error payload: %v", err)
	}

	// Also parse as generic map to get additional fields like path
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(envelope.Payload, &payloadMap); err == nil {
		if path, ok := payloadMap["path"].(string); ok {
			log.Printf("Received error from device %s: %s - %s (Path: %s)", c.deviceID, errorPayload.Code, errorPayload.Message, path)
		} else {
			log.Printf("Received error from device %s: %s - %s", c.deviceID, errorPayload.Code, errorPayload.Message)
		}
	} else {
		log.Printf("Received error from device %s: %s - %s", c.deviceID, errorPayload.Code, errorPayload.Message)
	}

	// Create error response object
	resp := map[string]interface{}{
		"req_id":    envelope.ReqID,
		"success":   false,
		"error":     errorPayload,
		"payload":   envelope.Payload,
		"timestamp": time.Now(),
	}

	// Handle response
	return mgr.HandleResponse(envelope.ReqID, resp)
}

// handleProgress handles progress messages from agents
func (c *Connection) handleProgress(envelope *Envelope) error {
	// Forward progress message to admin clients
	if c.manager.adminNotifier != nil {
		// Add device_id to the payload for admin clients
		var progressData map[string]interface{}
		if err := json.Unmarshal(envelope.Payload, &progressData); err != nil {
			log.Printf("Failed to parse progress payload: %v", err)
			return nil
		}

		progressData["device_id"] = c.deviceID

		// Re-marshal with device_id
		updatedPayload, err := json.Marshal(progressData)
		if err != nil {
			log.Printf("Failed to marshal progress payload: %v", err)
			return nil
		}

		// Create new envelope with updated payload
		progressEnvelope := Envelope{
			Type:      MsgTypeProgress,
			ReqID:     envelope.ReqID,
			Timestamp: envelope.Timestamp,
			DeviceID:  c.deviceID,
			Payload:   updatedPayload,
		}

		// Broadcast to admin clients
		c.manager.adminNotifier.BroadcastProgress(&progressEnvelope)
	}

	return nil
}

// sendHelloAck sends a hello_ack message
func (c *Connection) sendHelloAck(success bool, message string, reqID string) error {
	payload := HelloAckPayload{
		Success: success,
		Message: message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	envelope := Envelope{
		Type:      MsgTypeHelloAck,
		ReqID:     reqID,
		Timestamp: time.Now().Unix(),
		DeviceID:  c.deviceID,
		Payload:   payloadBytes,
	}

	return c.sendEnvelope(&envelope)
}

// sendError sends an error message
func (c *Connection) sendError(code ErrorCode, message string, reqID string) error {
	payload := ErrorPayload{
		Code:    code,
		Message: message,
		ReqID:   reqID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	envelope := Envelope{
		Type:      MsgTypeError,
		ReqID:     reqID,
		Timestamp: time.Now().Unix(),
		DeviceID:  c.deviceID,
		Payload:   payloadBytes,
	}

	return c.sendEnvelope(&envelope)
}

// sendEnvelope sends an envelope to the connection
func (c *Connection) sendEnvelope(envelope *Envelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// close closes the connection and cleans up
func (c *Connection) close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.deviceID != "" {
		// Remove from manager
		c.manager.mu.Lock()
		delete(c.manager.connections, c.deviceID)
		c.manager.mu.Unlock()

		// Get device info for notification before deleting
		device, _ := c.manager.registry.Get(c.deviceID)

		// Delete device from database (instead of just marking offline)
		if err := c.manager.registry.Delete(c.deviceID); err != nil {
			log.Printf("Failed to delete device from registry: %v", err)
		}

		log.Printf("Device disconnected and deleted: %s", c.deviceID)

		// Notify admin clients about device disconnection
		if device != nil {
			c.manager.notifyAdminDeviceUpdate(device, "disconnected")
		}
	}

	close(c.send)
}

// GetConnection retrieves a connection by device ID
func (m *Manager) GetConnection(deviceID string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, ok := m.connections[deviceID]
	return conn, ok
}

// SendToDevice sends a message to a specific device
func (m *Manager) SendToDevice(deviceID string, envelope interface{}) error {
	conn, ok := m.GetConnection(deviceID)
	if !ok {
		return fmt.Errorf("device not connected: %s", deviceID)
	}

	// Handle both *Envelope and other types (from RPC manager)
	switch env := envelope.(type) {
	case *Envelope:
		return conn.sendEnvelope(env)
	default:
		// Marshal and unmarshal to convert to Envelope
		data, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("failed to marshal envelope: %w", err)
		}
		var wsEnvelope Envelope
		if err := json.Unmarshal(data, &wsEnvelope); err != nil {
			return fmt.Errorf("failed to unmarshal envelope: %w", err)
		}
		return conn.sendEnvelope(&wsEnvelope)
	}
}

// SetRPCManager sets the RPC manager (to avoid circular dependency)
func (m *Manager) SetRPCManager(rpcManager interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rpcManager = rpcManager
}

// GetRPCManager returns the RPC manager
func (m *Manager) GetRPCManager() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rpcManager
}
