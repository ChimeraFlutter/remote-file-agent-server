package devices

import "time"

// Platform represents the device platform type
type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "macos"
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// DeviceStatus represents the online status of a device
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
)

// Root represents a whitelisted root directory
type Root struct {
	RootID  string `json:"root_id"`  // Root directory ID (e.g., "logs", "desktop")
	Name    string `json:"name"`     // Display name (e.g., "Logs", "Desktop")
	AbsPath string `json:"abs_path"` // Absolute path (e.g., "C:\Work\logs")
}

// Device represents a registered device
type Device struct {
	DeviceID     string       `json:"device_id"`     // Stable unique device identifier
	DeviceName   string       `json:"device_name"`   // Device name (user-visible)
	Platform     Platform     `json:"platform"`      // Platform type
	Version      string       `json:"version"`       // Agent version
	IP           string       `json:"ip"`            // Device IP address
	LastSeen     time.Time    `json:"last_seen"`     // Last active time
	Status       DeviceStatus `json:"status"`        // Online status
	AllowedRoots []Root       `json:"allowed_roots"` // Whitelist directory list
	ConnID       string       `json:"-"`             // WebSocket connection ID (internal use)
	CreatedAt    time.Time    `json:"created_at"`    // First registration time
	UpdatedAt    time.Time    `json:"updated_at"`    // Last update time
}
