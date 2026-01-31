package devices

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Create temporary database file
	tmpFile, err := os.CreateTemp("", "test_devices_*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	dbPath := tmpFile.Name()

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("failed to open database: %v", err)
	}

	// Create schema
	schema := `
		CREATE TABLE devices (
			device_id TEXT PRIMARY KEY,
			device_name TEXT NOT NULL,
			platform TEXT NOT NULL,
			version TEXT NOT NULL,
			ip TEXT,
			last_seen INTEGER NOT NULL,
			status TEXT NOT NULL,
			allowed_roots TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX idx_devices_status ON devices(status);
		CREATE INDEX idx_devices_last_seen ON devices(last_seen);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.Remove(dbPath)
		t.Fatalf("failed to create schema: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func TestRegistry_Register(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Test registering a new device
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
		{RootID: "desktop", Name: "Desktop", AbsPath: "/Users/test/Desktop"},
	}

	device, err := registry.Register("device-1", "Test Device", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	if device.DeviceID != "device-1" {
		t.Errorf("expected device_id 'device-1', got '%s'", device.DeviceID)
	}
	if device.DeviceName != "Test Device" {
		t.Errorf("expected device_name 'Test Device', got '%s'", device.DeviceName)
	}
	if device.Status != DeviceStatusOnline {
		t.Errorf("expected status 'online', got '%s'", device.Status)
	}
	if len(device.AllowedRoots) != 2 {
		t.Errorf("expected 2 allowed roots, got %d", len(device.AllowedRoots))
	}

	// Test updating an existing device
	updatedRoots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	device2, err := registry.Register("device-1", "Updated Device", PlatformMacOS, "1.1.0", updatedRoots, "192.168.1.101")
	if err != nil {
		t.Fatalf("failed to update device: %v", err)
	}

	if device2.DeviceName != "Updated Device" {
		t.Errorf("expected device_name 'Updated Device', got '%s'", device2.DeviceName)
	}
	if device2.Version != "1.1.0" {
		t.Errorf("expected version '1.1.0', got '%s'", device2.Version)
	}
	if len(device2.AllowedRoots) != 1 {
		t.Errorf("expected 1 allowed root, got %d", len(device2.AllowedRoots))
	}
}

func TestRegistry_Get(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register a device
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Test Device", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	// Get the device
	device, err := registry.Get("device-1")
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	if device.DeviceID != "device-1" {
		t.Errorf("expected device_id 'device-1', got '%s'", device.DeviceID)
	}

	// Try to get non-existent device
	_, err = registry.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent device, got nil")
	}
}

func TestRegistry_List(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register multiple devices
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Device 1", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device-1: %v", err)
	}

	_, err = registry.Register("device-2", "Device 2", PlatformWindows, "1.0.0", roots, "192.168.1.101")
	if err != nil {
		t.Fatalf("failed to register device-2: %v", err)
	}

	// List all devices
	devices, err := registry.List()
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}
}

func TestRegistry_SetConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register a device
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Test Device", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	// Set connection
	err = registry.SetConnection("device-1", "conn-123")
	if err != nil {
		t.Fatalf("failed to set connection: %v", err)
	}

	// Check if device is online
	if !registry.IsOnline("device-1") {
		t.Error("expected device to be online")
	}

	// Get connection ID
	connID, ok := registry.GetConnectionID("device-1")
	if !ok {
		t.Error("expected to find connection ID")
	}
	if connID != "conn-123" {
		t.Errorf("expected connection ID 'conn-123', got '%s'", connID)
	}

	// Remove connection
	err = registry.RemoveConnection("device-1")
	if err != nil {
		t.Fatalf("failed to remove connection: %v", err)
	}

	// Check if device is offline
	if registry.IsOnline("device-1") {
		t.Error("expected device to be offline")
	}
}

func TestRegistry_ListOnline(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register multiple devices
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Device 1", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device-1: %v", err)
	}

	_, err = registry.Register("device-2", "Device 2", PlatformWindows, "1.0.0", roots, "192.168.1.101")
	if err != nil {
		t.Fatalf("failed to register device-2: %v", err)
	}

	// Set device-1 offline
	err = registry.RemoveConnection("device-1")
	if err != nil {
		t.Fatalf("failed to remove connection: %v", err)
	}

	// List online devices
	devices, err := registry.ListOnline()
	if err != nil {
		t.Fatalf("failed to list online devices: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("expected 1 online device, got %d", len(devices))
	}

	if devices[0].DeviceID != "device-2" {
		t.Errorf("expected device-2 to be online, got %s", devices[0].DeviceID)
	}
}

func TestRegistry_CheckStaleDevices(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register a device
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Test Device", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	// Manually update last_seen to be old
	store := NewStore(db)
	oldTime := time.Now().Add(-10 * time.Minute)
	err = store.UpdateStatus("device-1", DeviceStatusOnline, oldTime)
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	// Check stale devices with 5 minute timeout
	err = registry.CheckStaleDevices(5 * time.Minute)
	if err != nil {
		t.Fatalf("failed to check stale devices: %v", err)
	}

	// Device should now be offline
	device, err := registry.Get("device-1")
	if err != nil {
		t.Fatalf("failed to get device: %v", err)
	}

	if device.Status != DeviceStatusOffline {
		t.Errorf("expected device to be offline, got %s", device.Status)
	}
}

func TestRegistry_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	registry := NewRegistry(db)

	// Register a device
	roots := []Root{
		{RootID: "logs", Name: "Logs", AbsPath: "/var/logs"},
	}

	_, err := registry.Register("device-1", "Test Device", PlatformMacOS, "1.0.0", roots, "192.168.1.100")
	if err != nil {
		t.Fatalf("failed to register device: %v", err)
	}

	// Set connection
	err = registry.SetConnection("device-1", "conn-123")
	if err != nil {
		t.Fatalf("failed to set connection: %v", err)
	}

	// Delete device
	err = registry.Delete("device-1")
	if err != nil {
		t.Fatalf("failed to delete device: %v", err)
	}

	// Device should not exist
	_, err = registry.Get("device-1")
	if err == nil {
		t.Error("expected error for deleted device, got nil")
	}

	// Connection should be removed
	if registry.IsOnline("device-1") {
		t.Error("expected device connection to be removed")
	}
}
