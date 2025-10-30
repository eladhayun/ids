package main

import (
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/embeddings"
	"time"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logger := cfg.SetupLogger()

	// Initialize database connection
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Create embedding service
	embeddingService, err := embeddings.NewEmbeddingService(cfg, db)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create embedding service")
	}

	// Create embeddings table
	logger.Info().Msg("Creating embeddings table...")
	if err := embeddingService.CreateEmbeddingsTable(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create embeddings table")
	}

	// Generate embeddings for all products
	logger.Info().Msg("Generating embeddings for all products...")
	start := time.Now()
	
	if err := embeddingService.GenerateProductEmbeddings(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate product embeddings")
	}

	duration := time.Since(start)
	logger.Info().Dur("duration", duration).Msg("Successfully generated embeddings for all products")
}
