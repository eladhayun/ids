package main

import (
	"github.com/rs/zerolog"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/server"
	"os"
	"time"
)

// waitForTunnel waits for the SSH tunnel to be ready
func waitForTunnel(logger *zerolog.Logger) {
	tunnelReadyFile := "/shared/tunnel-ready"
	maxWait := 60 * time.Second
	checkInterval := 1 * time.Second

	logger.Info().Msg("Waiting for SSH tunnel to be ready...")

	start := time.Now()
	for {
		if _, err := os.Stat(tunnelReadyFile); err == nil {
			logger.Info().Msg("SSH tunnel is ready, proceeding with database connection")
			return
		}

		if time.Since(start) > maxWait {
			logger.Warn().Msg("Timed out waiting for SSH tunnel, proceeding anyway")
			return
		}

		time.Sleep(checkInterval)
	}
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Setup logger
	logger := cfg.SetupLogger()

	// Wait for SSH tunnel to be ready before attempting database connection
	waitForTunnel(&logger)

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
