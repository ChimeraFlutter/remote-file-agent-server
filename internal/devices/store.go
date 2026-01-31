package devices

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Store handles database operations for devices
type Store struct {
	db *sql.DB
}

// NewStore creates a new device store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new device into the database
func (s *Store) Create(device *Device) error {
	allowedRootsJSON, err := json.Marshal(device.AllowedRoots)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed_roots: %w", err)
	}

	query := `
		INSERT INTO devices (
			device_id, device_name, platform, version, ip,
			last_seen, status, allowed_roots, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(
		query,
		device.DeviceID,
		device.DeviceName,
		device.Platform,
		device.Version,
		device.IP,
		device.LastSeen.Unix(),
		device.Status,
		string(allowedRootsJSON),
		device.CreatedAt.Unix(),
		device.UpdatedAt.Unix(),
	)

	if err != nil {
		return fmt.Errorf("failed to insert device: %w", err)
	}

	return nil
}

// Get retrieves a device by ID
func (s *Store) Get(deviceID string) (*Device, error) {
	query := `
		SELECT device_id, device_name, platform, version, ip,
		       last_seen, status, allowed_roots, created_at, updated_at
		FROM devices
		WHERE device_id = ?
	`

	var device Device
	var allowedRootsJSON string
	var lastSeenUnix, createdAtUnix, updatedAtUnix int64

	err := s.db.QueryRow(query, deviceID).Scan(
		&device.DeviceID,
		&device.DeviceName,
		&device.Platform,
		&device.Version,
		&device.IP,
		&lastSeenUnix,
		&device.Status,
		&allowedRootsJSON,
		&createdAtUnix,
		&updatedAtUnix,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query device: %w", err)
	}

	// Parse allowed_roots JSON
	if err := json.Unmarshal([]byte(allowedRootsJSON), &device.AllowedRoots); err != nil {
		return nil, fmt.Errorf("failed to unmarshal allowed_roots: %w", err)
	}

	// Convert Unix timestamps to time.Time
	device.LastSeen = time.Unix(lastSeenUnix, 0)
	device.CreatedAt = time.Unix(createdAtUnix, 0)
	device.UpdatedAt = time.Unix(updatedAtUnix, 0)

	return &device, nil
}

// List retrieves all devices
func (s *Store) List() ([]*Device, error) {
	query := `
		SELECT device_id, device_name, platform, version, ip,
		       last_seen, status, allowed_roots, created_at, updated_at
		FROM devices
		ORDER BY last_seen DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var allowedRootsJSON string
		var lastSeenUnix, createdAtUnix, updatedAtUnix int64

		err := rows.Scan(
			&device.DeviceID,
			&device.DeviceName,
			&device.Platform,
			&device.Version,
			&device.IP,
			&lastSeenUnix,
			&device.Status,
			&allowedRootsJSON,
			&createdAtUnix,
			&updatedAtUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}

		// Parse allowed_roots JSON
		if err := json.Unmarshal([]byte(allowedRootsJSON), &device.AllowedRoots); err != nil {
			return nil, fmt.Errorf("failed to unmarshal allowed_roots: %w", err)
		}

		// Convert Unix timestamps to time.Time
		device.LastSeen = time.Unix(lastSeenUnix, 0)
		device.CreatedAt = time.Unix(createdAtUnix, 0)
		device.UpdatedAt = time.Unix(updatedAtUnix, 0)

		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating device rows: %w", err)
	}

	return devices, nil
}

// Update updates an existing device
func (s *Store) Update(device *Device) error {
	allowedRootsJSON, err := json.Marshal(device.AllowedRoots)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed_roots: %w", err)
	}

	query := `
		UPDATE devices
		SET device_name = ?, platform = ?, version = ?, ip = ?,
		    last_seen = ?, status = ?, allowed_roots = ?, updated_at = ?
		WHERE device_id = ?
	`

	result, err := s.db.Exec(
		query,
		device.DeviceName,
		device.Platform,
		device.Version,
		device.IP,
		device.LastSeen.Unix(),
		device.Status,
		string(allowedRootsJSON),
		device.UpdatedAt.Unix(),
		device.DeviceID,
	)

	if err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found: %s", device.DeviceID)
	}

	return nil
}

// UpdateStatus updates the status and last_seen of a device
func (s *Store) UpdateStatus(deviceID string, status DeviceStatus, lastSeen time.Time) error {
	query := `
		UPDATE devices
		SET status = ?, last_seen = ?, updated_at = ?
		WHERE device_id = ?
	`

	result, err := s.db.Exec(
		query,
		status,
		lastSeen.Unix(),
		time.Now().Unix(),
		deviceID,
	)

	if err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	return nil
}

// Delete removes a device from the database
func (s *Store) Delete(deviceID string) error {
	query := `DELETE FROM devices WHERE device_id = ?`

	result, err := s.db.Exec(query, deviceID)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("device not found: %s", deviceID)
	}

	return nil
}

// DeleteOffline removes all offline devices from the database
func (s *Store) DeleteOffline() (int64, error) {
	query := `DELETE FROM devices WHERE status = ?`

	result, err := s.db.Exec(query, DeviceStatusOffline)
	if err != nil {
		return 0, fmt.Errorf("failed to delete offline devices: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// DeleteAll removes all devices from the database (for cleanup on startup)
func (s *Store) DeleteAll() (int64, error) {
	query := `DELETE FROM devices`

	result, err := s.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete all devices: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// ListByStatus retrieves devices by status
func (s *Store) ListByStatus(status DeviceStatus) ([]*Device, error) {
	query := `
		SELECT device_id, device_name, platform, version, ip,
		       last_seen, status, allowed_roots, created_at, updated_at
		FROM devices
		WHERE status = ?
		ORDER BY last_seen DESC
	`

	rows, err := s.db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices by status: %w", err)
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var allowedRootsJSON string
		var lastSeenUnix, createdAtUnix, updatedAtUnix int64

		err := rows.Scan(
			&device.DeviceID,
			&device.DeviceName,
			&device.Platform,
			&device.Version,
			&device.IP,
			&lastSeenUnix,
			&device.Status,
			&allowedRootsJSON,
			&createdAtUnix,
			&updatedAtUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}

		// Parse allowed_roots JSON
		if err := json.Unmarshal([]byte(allowedRootsJSON), &device.AllowedRoots); err != nil {
			return nil, fmt.Errorf("failed to unmarshal allowed_roots: %w", err)
		}

		// Convert Unix timestamps to time.Time
		device.LastSeen = time.Unix(lastSeenUnix, 0)
		device.CreatedAt = time.Unix(createdAtUnix, 0)
		device.UpdatedAt = time.Unix(updatedAtUnix, 0)

		devices = append(devices, &device)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating device rows: %w", err)
	}

	return devices, nil
}
