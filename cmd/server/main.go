package main

import (
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/server"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logger := cfg.SetupLogger()

	// Initialize database connection
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		logger.Warn().Err(err).Msg("Database connection failed")
		logger.Info().Msg("Starting server without database connection")
	} else {
		logger.Info().Msg("Database connection established successfully")
	}

	// Create and initialize server
	srv := server.New(cfg, db, logger)
	srv.Initialize()

	// Start server
	if err := srv.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Server failed to start")
	}
}
