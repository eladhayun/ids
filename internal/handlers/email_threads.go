package handlers

import (
	"fmt"
	"net/http"

	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/emails"

	"github.com/labstack/echo/v4"
)

// GenerateThreadEmbeddingsResponse represents the response
type GenerateThreadEmbeddingsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// GenerateThreadEmbeddingsHandler triggers thread embedding generation
// @Summary Generate email thread embeddings
// @Description Creates embeddings for email conversation threads
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} GenerateThreadEmbeddingsResponse
// @Failure 500 {object} GenerateThreadEmbeddingsResponse
// @Router /api/admin/generate-thread-embeddings [post]
func GenerateThreadEmbeddingsHandler(c echo.Context) error {
	fmt.Println("[THREAD_GEN] Starting thread embedding generation...")

	// Load configuration
	cfg := config.Load()

	// Create write database client
	writeClient, err := database.NewWriteClient(cfg.EmbeddingsDatabaseURL)
	if err != nil {
		fmt.Printf("[THREAD_GEN] Failed to create database client: %v\n", err)
		return c.JSON(http.StatusInternalServerError, GenerateThreadEmbeddingsResponse{
			Success: false,
			Message: "Failed to connect to database",
			Error:   err.Error(),
		})
	}
	defer writeClient.Close()

	// Create email embedding service
	emailService, err := emails.NewEmailEmbeddingService(cfg, writeClient)
	if err != nil {
		fmt.Printf("[THREAD_GEN] Failed to create email service: %v\n", err)
		return c.JSON(http.StatusInternalServerError, GenerateThreadEmbeddingsResponse{
			Success: false,
			Message: "Failed to initialize email service",
			Error:   err.Error(),
		})
	}

	// Generate thread embeddings
	fmt.Println("[THREAD_GEN] Generating thread embeddings...")
	if err := emailService.GenerateThreadEmbeddings(); err != nil {
		fmt.Printf("[THREAD_GEN] Failed to generate thread embeddings: %v\n", err)
		return c.JSON(http.StatusInternalServerError, GenerateThreadEmbeddingsResponse{
			Success: false,
			Message: "Failed to generate thread embeddings",
			Error:   err.Error(),
		})
	}

	fmt.Println("[THREAD_GEN] ✅ Thread embeddings generated successfully")

	return c.JSON(http.StatusOK, GenerateThreadEmbeddingsResponse{
		Success: true,
		Message: "Thread embeddings generated successfully",
	})
}
