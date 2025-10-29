package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

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
// @Router /healthz [get]
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
// @Router /healthz/db [get]
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

		// Test a simple query to ensure database is readable
		var count int
		err = db.Get(&count, "SELECT 1")
		if err != nil {
			response.Status = statusUnhealthy
			response.Error = fmt.Sprintf("Database query failed: %v", err)
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
// @Router / [get]
func RootHandler(version string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"service": "IDS API",
			"version": version,
			"status":  "running",
		})
	}
}

// ProductsHandler handles requests to get products with pagination
// @Summary Get products
// @Description Get paginated list of products from the database
// @Tags products
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Success 200 {object} models.ProductsResponse
// @Failure 503 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /products [get]
func ProductsHandler(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Handle case where database connection is not available
		if db == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Database connection not available",
			})
		}

		// Parse pagination parameters
		page := 1
		if p := c.QueryParam("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}

		limit := 100 // Fixed limit as requested
		offset := (page - 1) * limit

		// Count total products first
		countQuery := `
			SELECT COUNT(DISTINCT p.ID)
			FROM wpjr_wc_product_meta_lookup l
			JOIN wpjr_posts p ON p.ID = l.product_id
			WHERE p.post_type = 'product'
			  AND p.post_status IN ('publish','private')
		`

		var totalCount int
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := db.GetContext(ctx, &totalCount, countQuery)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to count products: %v", err),
			})
		}

		// Main query with pagination
		query := `
			SELECT
			  p.ID,
			  p.post_title,
			  p.post_content   AS description,
			  p.post_excerpt   AS short_description,
			  l.sku,
			  l.min_price,
			  l.max_price,
			  l.stock_status,
			  l.stock_quantity,
			  GROUP_CONCAT(DISTINCT t.name ORDER BY t.name SEPARATOR ', ') AS tags
			FROM wpjr_wc_product_meta_lookup l
			JOIN wpjr_posts p ON p.ID = l.product_id
			LEFT JOIN wpjr_term_relationships tr
			  ON tr.object_id = p.ID
			LEFT JOIN wpjr_term_taxonomy tt
			  ON tt.term_taxonomy_id = tr.term_taxonomy_id
			  AND tt.taxonomy = 'product_tag'
			LEFT JOIN wpjr_terms t
			  ON t.term_id = tt.term_id
			WHERE p.post_type = 'product'
			  AND p.post_status IN ('publish','private')
			GROUP BY
			  p.ID, p.post_title, p.post_content, p.post_excerpt,
			  l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
			ORDER BY p.ID
			LIMIT ? OFFSET ?
		`

		var products []models.Product
		err = db.SelectContext(ctx, &products, query, limit, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to fetch products: %v", err),
			})
		}

		// Calculate pagination metadata
		totalPages := int(math.Ceil(float64(totalCount) / float64(limit)))
		hasNext := page < totalPages
		hasPrev := page > 1

		response := models.ProductsResponse{
			Data: products,
			Pagination: models.PaginationMeta{
				Page:       page,
				Limit:      limit,
				TotalCount: totalCount,
				TotalPages: totalPages,
				HasNext:    hasNext,
				HasPrev:    hasPrev,
			},
		}

		return c.JSON(http.StatusOK, response)
	}
}
