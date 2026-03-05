package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/remote-file-manager/server/internal/devices"
	"github.com/remote-file-manager/server/internal/ws"
)

var adminUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// AdminConnection represents an admin WebSocket connection
type AdminConnection struct {
	conn    *websocket.Conn
	send    chan []byte
	manager *AdminWSManager
	mu      sync.Mutex
}

// AdminWSManager manages admin WebSocket connections for real-time updates
type AdminWSManager struct {
	registry    *devices.Registry
	connections map[*AdminConnection]bool
	mu          sync.RWMutex
	broadcast   chan []byte
}

// NewAdminWSManager creates a new admin WebSocket manager
func NewAdminWSManager(registry *devices.Registry) *AdminWSManager {
	m := &AdminWSManager{
		registry:    registry,
		connections: make(map[*AdminConnection]bool),
		broadcast:   make(chan []byte, 256),
	}
	go m.run()
	return m
}

// run handles broadcasting messages to all connections
func (m *AdminWSManager) run() {
	for {
		msg := <-m.broadcast
		m.mu.RLock()
		for conn := range m.connections {
			select {
			case conn.send <- msg:
			default:
				close(conn.send)
				delete(m.connections, conn)
			}
		}
		m.mu.RUnlock()
	}
}

// HandleConnection handles new admin WebSocket connections
func (m *AdminWSManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := adminUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade admin connection: %v", err)
		return
	}

	adminConn := &AdminConnection{
		conn:    conn,
		send:    make(chan []byte, 256),
		manager: m,
	}

	m.mu.Lock()
	m.connections[adminConn] = true
	m.mu.Unlock()

	// Send initial device list
	m.sendDeviceList(adminConn)

	go adminConn.writePump()
	go adminConn.readPump()
}

// sendDeviceList sends the current device list to a connection
func (m *AdminWSManager) sendDeviceList(conn *AdminConnection) {
	deviceList, err := m.registry.ListOnline()
	if err != nil {
		log.Printf("Failed to get device list: %v", err)
		return
	}

	msg := map[string]interface{}{
		"type":    "device_list",
		"devices": deviceList,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal device list: %v", err)
		return
	}

	select {
	case conn.send <- data:
	default:
	}
}

// BroadcastDeviceUpdate broadcasts a device update to all admin connections
func (m *AdminWSManager) BroadcastDeviceUpdate(device *devices.Device, event string) {
	m.mu.RLock()
	connCount := len(m.connections)
	m.mu.RUnlock()
	log.Printf("DEBUG: BroadcastDeviceUpdate - %d admin clients connected", connCount)

	msg := map[string]interface{}{
		"type":   "device_update",
		"event":  event, // "connected", "disconnected", "updated"
		"device": device,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal device update: %v", err)
		return
	}

	select {
	case m.broadcast <- data:
		log.Printf("DEBUG: Message sent to broadcast channel")
	default:
		log.Printf("DEBUG: Broadcast channel full, message dropped")
	}
}

// BroadcastDeviceList broadcasts the full device list to all admin connections
func (m *AdminWSManager) BroadcastDeviceList() {
	deviceList, err := m.registry.ListOnline()
	if err != nil {
		log.Printf("Failed to get device list: %v", err)
		return
	}

	msg := map[string]interface{}{
		"type":    "device_list",
		"devices": deviceList,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal device list: %v", err)
		return
	}

	select {
	case m.broadcast <- data:
	default:
	}
}

// BroadcastProgress broadcasts upload progress to all admin connections
func (m *AdminWSManager) BroadcastProgress(envelope *ws.Envelope) {
	msg := map[string]interface{}{
		"type":     "upload_progress",
		"progress": envelope,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal progress: %v", err)
		return
	}

	select {
	case m.broadcast <- data:
	default:
	}
}

// GetConnectionCount returns the number of active admin connections
func (m *AdminWSManager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// readPump reads messages from the connection
func (c *AdminConnection) readPump() {
	defer func() {
		c.manager.mu.Lock()
		delete(c.manager.connections, c)
		c.manager.mu.Unlock()
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Admin WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump writes messages to the connection
func (c *AdminConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
