package ws

import (
	"log"
	"time"
)

// StartStaleDeviceChecker starts a background goroutine to check for stale devices
func (m *Manager) StartStaleDeviceChecker(interval time.Duration, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := m.registry.CheckStaleDevices(timeout); err != nil {
				log.Printf("Failed to check stale devices: %v", err)
			}
		}
	}()
}

// BroadcastToAll sends a message to all connected devices
func (m *Manager) BroadcastToAll(envelope *Envelope) {
	m.mu.RLock()
	connections := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, conn)
	}
	m.mu.RUnlock()

	for _, conn := range connections {
		if err := conn.sendEnvelope(envelope); err != nil {
			log.Printf("Failed to send broadcast to device %s: %v", conn.deviceID, err)
		}
	}
}

// GetConnectedDeviceCount returns the number of currently connected devices
func (m *Manager) GetConnectedDeviceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// GetConnectedDeviceIDs returns a list of all connected device IDs
func (m *Manager) GetConnectedDeviceIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.connections))
	for deviceID := range m.connections {
		ids = append(ids, deviceID)
	}
	return ids
}
