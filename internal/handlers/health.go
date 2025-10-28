package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"ids/internal/models"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// HealthHandler handles basic health check requests
func HealthHandler(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		response := models.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().UTC(),
			Version:   version,
		}

		return c.JSON(http.StatusOK, response)
	}
}

// DBHealthHandler handles database health check requests
func DBHealthHandler(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		response := models.DBHealthResponse{
			Status:    "unknown",
			Timestamp: time.Now().UTC(),
			Connected: false,
			Latency:   0,
		}

		// Check if database connection exists
		if db == nil {
			response.Status = "unhealthy"
			response.Error = "Database connection not initialized"
			return c.JSON(http.StatusServiceUnavailable, response)
		}

		// Measure database ping latency
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := db.PingContext(ctx)
		latency := time.Since(start)
		response.Latency = latency

		if err != nil {
			response.Status = "unhealthy"
			response.Error = err.Error()
			return c.JSON(http.StatusServiceUnavailable, response)
		}

		// Test a simple query to ensure database is readable
		var count int
		err = db.Get(&count, "SELECT 1")
		if err != nil {
			response.Status = "unhealthy"
			response.Error = fmt.Sprintf("Database query failed: %v", err)
			return c.JSON(http.StatusServiceUnavailable, response)
		}

		response.Status = "healthy"
		response.Connected = true

		return c.JSON(http.StatusOK, response)
	}
}

// RootHandler handles requests to the root endpoint
func RootHandler(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": "IDS API",
			"version": version,
			"status":  "running",
		})
	}
}
