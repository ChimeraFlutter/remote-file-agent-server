package devices

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// Registry manages device registration and lifecycle
type Registry struct {
	store       *Store
	mu          sync.RWMutex
	connections map[string]string // deviceID -> connID mapping
}

// NewRegistry creates a new device registry
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		store:       NewStore(db),
		connections: make(map[string]string),
	}
}

// Register registers a new device or updates an existing one
// Returns the device and a boolean indicating if it's a new device
func (r *Registry) Register(deviceID, deviceName string, platform Platform, version string, allowedRoots []Root, ip string) (*Device, bool, error) {
	now := time.Now()

	// Check if device already exists
	existingDevice, err := r.store.Get(deviceID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check existing device: %w", err)
	}

	if existingDevice != nil {
		// Update existing device
		existingDevice.DeviceName = deviceName
		existingDevice.Platform = platform
		existingDevice.Version = version
		existingDevice.IP = ip
		existingDevice.LastSeen = now
		existingDevice.Status = DeviceStatusOnline
		existingDevice.AllowedRoots = allowedRoots
		existingDevice.UpdatedAt = now

		if err := r.store.Update(existingDevice); err != nil {
			return nil, false, fmt.Errorf("failed to update device: %w", err)
		}

		return existingDevice, false, nil
	}

	// Create new device
	device := &Device{
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		Platform:     platform,
		Version:      version,
		IP:           ip,
		LastSeen:     now,
		Status:       DeviceStatusOnline,
		AllowedRoots: allowedRoots,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := r.store.Create(device); err != nil {
		return nil, false, fmt.Errorf("failed to create device: %w", err)
	}

	return device, true, nil
}

// Get retrieves a device by ID
func (r *Registry) Get(deviceID string) (*Device, error) {
	device, err := r.store.Get(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	if device == nil {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	// Update ConnID from in-memory map
	r.mu.RLock()
	if connID, ok := r.connections[deviceID]; ok {
		device.ConnID = connID
	}
	r.mu.RUnlock()

	return device, nil
}

// List retrieves all devices
func (r *Registry) List() ([]*Device, error) {
	devices, err := r.store.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// Update ConnID from in-memory map
	r.mu.RLock()
	for _, device := range devices {
		if connID, ok := r.connections[device.DeviceID]; ok {
			device.ConnID = connID
		}
	}
	r.mu.RUnlock()

	return devices, nil
}

// ListOnline retrieves all online devices
func (r *Registry) ListOnline() ([]*Device, error) {
	devices, err := r.store.ListByStatus(DeviceStatusOnline)
	if err != nil {
		return nil, fmt.Errorf("failed to list online devices: %w", err)
	}

	// Update ConnID from in-memory map
	r.mu.RLock()
	for _, device := range devices {
		if connID, ok := r.connections[device.DeviceID]; ok {
			device.ConnID = connID
		}
	}
	r.mu.RUnlock()

	return devices, nil
}

// SetConnection associates a WebSocket connection ID with a device
func (r *Registry) SetConnection(deviceID, connID string) error {
	r.mu.Lock()
	r.connections[deviceID] = connID
	r.mu.Unlock()

	// Update device status to online
	if err := r.store.UpdateStatus(deviceID, DeviceStatusOnline, time.Now()); err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	return nil
}

// RemoveConnection removes the WebSocket connection association
func (r *Registry) RemoveConnection(deviceID string) error {
	r.mu.Lock()
	delete(r.connections, deviceID)
	r.mu.Unlock()

	// Update device status to offline
	if err := r.store.UpdateStatus(deviceID, DeviceStatusOffline, time.Now()); err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	return nil
}

// GetConnectionID retrieves the connection ID for a device
func (r *Registry) GetConnectionID(deviceID string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	connID, ok := r.connections[deviceID]
	return connID, ok
}

// IsOnline checks if a device is currently online
func (r *Registry) IsOnline(deviceID string) bool {
	r.mu.RLock()
	_, ok := r.connections[deviceID]
	r.mu.RUnlock()
	return ok
}

// UpdateHeartbeat updates the last_seen timestamp for a device
func (r *Registry) UpdateHeartbeat(deviceID string) error {
	if err := r.store.UpdateStatus(deviceID, DeviceStatusOnline, time.Now()); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}
	return nil
}

// CheckStaleDevices marks devices as offline if they haven't been seen recently
func (r *Registry) CheckStaleDevices(timeout time.Duration) error {
	devices, err := r.store.ListByStatus(DeviceStatusOnline)
	if err != nil {
		return fmt.Errorf("failed to list online devices: %w", err)
	}

	now := time.Now()
	for _, device := range devices {
		// Check if device has no active connection and hasn't been seen recently
		r.mu.RLock()
		_, hasConnection := r.connections[device.DeviceID]
		r.mu.RUnlock()

		if !hasConnection && now.Sub(device.LastSeen) > timeout {
			if err := r.store.UpdateStatus(device.DeviceID, DeviceStatusOffline, device.LastSeen); err != nil {
				return fmt.Errorf("failed to mark device offline: %w", err)
			}
		}
	}

	return nil
}

// Delete removes a device from the registry
func (r *Registry) Delete(deviceID string) error {
	// Remove connection if exists
	r.mu.Lock()
	delete(r.connections, deviceID)
	r.mu.Unlock()

	// Delete from database
	if err := r.store.Delete(deviceID); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	return nil
}

// CleanupAllDevices removes all devices from the database (called on startup)
func (r *Registry) CleanupAllDevices() (int64, error) {
	r.mu.Lock()
	r.connections = make(map[string]string)
	r.mu.Unlock()

	return r.store.DeleteAll()
}
