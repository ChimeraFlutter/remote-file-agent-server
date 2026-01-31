package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Metrics holds application metrics
type Metrics struct {
	mu sync.RWMutex

	// Request metrics
	TotalRequests     int64
	SuccessRequests   int64
	FailedRequests    int64
	ActiveConnections int64

	// Upload/Download metrics
	TotalUploads    int64
	TotalDownloads  int64
	BytesUploaded   int64
	BytesDownloaded int64

	// Device metrics
	TotalDevices  int64
	OnlineDevices int64

	// RPC metrics
	TotalRPCCalls  int64
	FailedRPCCalls int64
	RPCTimeouts    int64

	// Start time
	StartTime time.Time
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime: time.Now(),
	}
}

// IncrementRequests increments total requests
func (m *Metrics) IncrementRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRequests++
}

// IncrementSuccessRequests increments successful requests
func (m *Metrics) IncrementSuccessRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuccessRequests++
}

// IncrementFailedRequests increments failed requests
func (m *Metrics) IncrementFailedRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedRequests++
}

// IncrementActiveConnections increments active connections
func (m *Metrics) IncrementActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections++
}

// DecrementActiveConnections decrements active connections
func (m *Metrics) DecrementActiveConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveConnections--
}

// IncrementUploads increments total uploads
func (m *Metrics) IncrementUploads(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalUploads++
	m.BytesUploaded += bytes
}

// IncrementDownloads increments total downloads
func (m *Metrics) IncrementDownloads(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalDownloads++
	m.BytesDownloaded += bytes
}

// SetDeviceMetrics sets device metrics
func (m *Metrics) SetDeviceMetrics(total, online int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalDevices = total
	m.OnlineDevices = online
}

// IncrementRPCCalls increments total RPC calls
func (m *Metrics) IncrementRPCCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalRPCCalls++
}

// IncrementFailedRPCCalls increments failed RPC calls
func (m *Metrics) IncrementFailedRPCCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedRPCCalls++
}

// IncrementRPCTimeouts increments RPC timeouts
func (m *Metrics) IncrementRPCTimeouts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RPCTimeouts++
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.StartTime)

	return map[string]interface{}{
		"uptime_seconds":     uptime.Seconds(),
		"total_requests":     m.TotalRequests,
		"success_requests":   m.SuccessRequests,
		"failed_requests":    m.FailedRequests,
		"active_connections": m.ActiveConnections,
		"total_uploads":      m.TotalUploads,
		"total_downloads":    m.TotalDownloads,
		"bytes_uploaded":     m.BytesUploaded,
		"bytes_downloaded":   m.BytesDownloaded,
		"total_devices":      m.TotalDevices,
		"online_devices":     m.OnlineDevices,
		"total_rpc_calls":    m.TotalRPCCalls,
		"failed_rpc_calls":   m.FailedRPCCalls,
		"rpc_timeouts":       m.RPCTimeouts,
	}
}

// Handler returns an HTTP handler for metrics endpoint
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := m.GetSnapshot()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON encoding
		w.Write([]byte("{"))
		first := true
		for k, v := range snapshot {
			if !first {
				w.Write([]byte(","))
			}
			first = false
			w.Write([]byte("\"" + k + "\":"))
			switch val := v.(type) {
			case int64:
				w.Write([]byte(fmt.Sprintf("%d", val)))
			case float64:
				w.Write([]byte(fmt.Sprintf("%.2f", val)))
			default:
				w.Write([]byte(fmt.Sprintf("%v", val)))
			}
		}
		w.Write([]byte("}"))
	}
}
