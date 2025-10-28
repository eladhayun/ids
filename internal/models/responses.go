package models

import "time"

// HealthResponse represents a basic health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// DBHealthResponse represents a database health check response
type DBHealthResponse struct {
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Connected bool          `json:"connected"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
}
