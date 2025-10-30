package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"ids/internal/database"
	"ids/internal/models"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

const (
	statusHealthy   = "healthy"
	statusUnhealthy = "unhealthy"
)

// HealthHandler handles basic health check requests
// @Summary Health check
// @Description Get basic health status of the application
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.HealthResponse
// @Router /api/healthz [get]
func HealthHandler(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		response := models.HealthResponse{
			Status:    statusHealthy,
			Timestamp: time.Now().UTC(),
			Version:   version,
		}

		return c.JSON(http.StatusOK, response)
	}
}

// DBHealthHandler handles database health check requests
// @Summary Database health check
// @Description Get database connectivity status and latency
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.DBHealthResponse
// @Failure 503 {object} models.DBHealthResponse
// @Router /api/healthz/db [get]
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
			response.Status = statusUnhealthy
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
			response.Status = statusUnhealthy
			response.Error = err.Error()
			return c.JSON(http.StatusServiceUnavailable, response)
		}

		// Test a simple query to ensure database is readable using read-only transaction
		var count int
		err = database.ExecuteReadOnlyQuerySingle(ctx, db, &count, "SELECT 1")
		if err != nil {
			response.Status = statusUnhealthy
			response.Error = fmt.Sprintf("Database read-only query failed: %v", err)
			return c.JSON(http.StatusServiceUnavailable, response)
		}

		response.Status = statusHealthy
		response.Connected = true

		return c.JSON(http.StatusOK, response)
	}
}

// RootHandler handles requests to the root endpoint
// @Summary Root endpoint
// @Description Get basic service information
// @Tags general
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/ [get]
func RootHandler(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": "IDS API",
			"version": version,
			"status":  "running",
		})
	}
}
