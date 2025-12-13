package main

import (
	"flag"
	"fmt"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/embeddings"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
)

func main() {
	// Parse command-line flags
	runOnce := flag.Bool("once", false, "Run embeddings generation once and exit (default: false, runs continuously)")
	flag.Parse()

	printStartupMessage(*runOnce)

	// Load configuration
	cfg := config.Load()
	scheduleInterval := time.Duration(cfg.EmbeddingScheduleHours) * time.Hour
	scheduleDescription := formatScheduleDescription(cfg.EmbeddingScheduleHours)

	// Wait for SSH tunnel if needed
	waitForSSHTunnel()

	// Initialize databases
	readDB, writeClient := initializeDatabases(cfg)
	defer closeDatabases(readDB, writeClient)

	// Initialize embedding service
	embeddingService := initializeEmbeddingService(cfg, readDB, writeClient, *runOnce)
	if embeddingService != nil {
		createEmbeddingsTable(embeddingService)
	}

	// Set up signal handling
	sigChan := setupSignalHandling()

	// Run initial embedding generation if service is available
	if embeddingService != nil {
		handleInitialGeneration(embeddingService, *runOnce)
	}

	// If running once, exit cleanly
	if *runOnce {
		fmt.Println("One-time run completed. Exiting.")
		return
	}

	// Run scheduled mode
	runScheduledMode(cfg, scheduleInterval, scheduleDescription, readDB, writeClient, embeddingService, sigChan)
}

// printStartupMessage prints the startup message based on run mode
func printStartupMessage(runOnce bool) {
	if runOnce {
		fmt.Println("=== EMBEDDING GENERATION (ONE-TIME RUN) ===")
	} else {
		fmt.Println("=== EMBEDDING SCHEDULED SERVICE ===")
	}
	fmt.Printf("Starting at: %s\n", time.Now().Format(time.RFC3339))
}

// waitForSSHTunnel waits for SSH tunnel to be ready if WAIT_FOR_TUNNEL is set
func waitForSSHTunnel() {
	if os.Getenv("WAIT_FOR_TUNNEL") != "true" {
		return
	}

	fmt.Println("Waiting for SSH tunnel to be ready...")
	tunnelReadyPath := "/shared/tunnel-ready"
	for i := 0; i < 60; i++ {
		if _, err := os.Stat(tunnelReadyPath); err == nil {
			fmt.Println("SSH tunnel is ready!")
			return
		}
		if i == 59 {
			log.Fatal("SSH tunnel did not become ready after 60 seconds")
		}
		fmt.Printf("Attempt %d/60: Tunnel not ready yet, waiting...\n", i+1)
		time.Sleep(1 * time.Second)
	}
}

// initializeDatabases initializes both read and write database connections
func initializeDatabases(cfg *config.Config) (*sqlx.DB, *database.WriteClient) {
	fmt.Println("Connecting to remote database for product reads...")
	readDB, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to remote database:", err)
	}

	fmt.Println("Connecting to embeddings database with write access...")
	writeClient, err := database.NewWriteClient(cfg.EmbeddingsDatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to embeddings database with write access:", err)
	}

	return readDB, writeClient
}

// closeDatabases closes database connections
func closeDatabases(readDB *sqlx.DB, writeClient *database.WriteClient) {
	if err := readDB.Close(); err != nil {
		log.Printf("Error closing read database: %v", err)
	}
	if err := writeClient.Close(); err != nil {
		log.Printf("Error closing write client: %v", err)
	}
}

// isQuotaError checks if an error is related to OpenAI quota
func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") || strings.Contains(errStr, "quota")
}

// initializeEmbeddingService initializes the embedding service with quota error handling
func initializeEmbeddingService(cfg *config.Config, readDB *sqlx.DB, writeClient *database.WriteClient, runOnce bool) *embeddings.WriteEmbeddingService {
	fmt.Println("Initializing embedding service...")
	embeddingService, err := embeddings.NewWriteEmbeddingService(cfg, readDB.DB, writeClient)
	if err != nil {
		if isQuotaError(err) {
			log.Printf("WARNING: OpenAI API quota exceeded. Embedding generation skipped. Error: %v", err)
			if runOnce {
				fmt.Println("One-time run completed with warnings (quota exceeded). Exiting.")
				os.Exit(0)
			}
			log.Printf("Will retry on next scheduled run. Current time: %s", time.Now().Format(time.RFC3339))
			return nil
		}
		log.Fatal("Failed to create embedding service:", err)
	}
	return embeddingService
}

// createEmbeddingsTable creates the embeddings table if it doesn't exist
func createEmbeddingsTable(embeddingService *embeddings.WriteEmbeddingService) {
	fmt.Println("Creating embeddings table...")
	if err := embeddingService.CreateEmbeddingsTable(); err != nil {
		log.Printf("WARNING: Failed to create embeddings table: %v", err)
		// Don't exit - table might already exist
	}
}

// setupSignalHandling sets up signal handling for graceful shutdown
func setupSignalHandling() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	return sigChan
}

// handleInitialGeneration runs the initial embedding generation
func handleInitialGeneration(embeddingService *embeddings.WriteEmbeddingService, runOnce bool) {
	fmt.Println("Running embedding generation...")
	if err := runEmbeddingGeneration(embeddingService); err != nil {
		if isQuotaError(err) {
			log.Printf("WARNING: Embedding generation skipped due to OpenAI quota: %v", err)
			if runOnce {
				fmt.Println("One-time run completed with warnings (quota exceeded). Exiting.")
				os.Exit(0)
			}
		} else {
			log.Printf("ERROR: Embedding generation failed: %v", err)
			if runOnce {
				os.Exit(1)
			}
		}
	} else {
		fmt.Println("Embedding generation completed successfully")
	}
}

// runScheduledMode runs the scheduled embedding generation loop
func runScheduledMode(cfg *config.Config, scheduleInterval time.Duration, scheduleDescription string,
	readDB *sqlx.DB, writeClient *database.WriteClient,
	embeddingService *embeddings.WriteEmbeddingService, sigChan chan os.Signal) {
	ticker := time.NewTicker(scheduleInterval)
	defer ticker.Stop()

	fmt.Printf("\nEmbedding service is now running in scheduled mode.\n")
	fmt.Printf("Will regenerate embeddings %s.\n", scheduleDescription)
	fmt.Printf("Schedule interval: %d hours (%v)\n", cfg.EmbeddingScheduleHours, scheduleInterval)
	fmt.Println("Press Ctrl+C to stop the service.")

	for {
		select {
		case <-ticker.C:
			handleScheduledGeneration(cfg, readDB, writeClient, &embeddingService)
		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)
			return
		}
	}
}

// handleScheduledGeneration handles a scheduled embedding generation run
func handleScheduledGeneration(cfg *config.Config, readDB *sqlx.DB, writeClient *database.WriteClient,
	embeddingService **embeddings.WriteEmbeddingService) {
	fmt.Printf("\n=== SCHEDULED EMBEDDING GENERATION TRIGGERED ===\n")
	fmt.Printf("Starting at: %s\n", time.Now().Format(time.RFC3339))

	// Re-initialize embedding service if it was nil
	if *embeddingService == nil {
		*embeddingService = reinitializeEmbeddingService(cfg, readDB, writeClient)
		if *embeddingService == nil {
			return
		}
	}

	// Run embedding generation
	if err := runEmbeddingGeneration(*embeddingService); err != nil {
		handleScheduledGenerationError(err, embeddingService)
	} else {
		fmt.Printf("Scheduled embedding generation completed successfully at: %s\n", time.Now().Format(time.RFC3339))
	}
}

// reinitializeEmbeddingService re-initializes the embedding service
func reinitializeEmbeddingService(cfg *config.Config, readDB *sqlx.DB, writeClient *database.WriteClient) *embeddings.WriteEmbeddingService {
	fmt.Println("Re-initializing embedding service...")
	embeddingService, err := embeddings.NewWriteEmbeddingService(cfg, readDB.DB, writeClient)
	if err != nil {
		if isQuotaError(err) {
			log.Printf("WARNING: OpenAI API quota still exceeded. Skipping this run. Error: %v", err)
			return nil
		}
		log.Printf("ERROR: Failed to re-initialize embedding service: %v", err)
		return nil
	}
	fmt.Println("Embedding service re-initialized successfully")
	return embeddingService
}

// handleScheduledGenerationError handles errors during scheduled generation
func handleScheduledGenerationError(err error, embeddingService **embeddings.WriteEmbeddingService) {
	if isQuotaError(err) {
		log.Printf("WARNING: Scheduled embedding generation skipped due to OpenAI quota: %v", err)
		// Set service to nil so we retry initialization next time
		*embeddingService = nil
	} else {
		log.Printf("ERROR: Scheduled embedding generation failed: %v", err)
		// Continue running even if one generation fails
	}
}

// runEmbeddingGeneration runs the embedding generation process
func runEmbeddingGeneration(embeddingService *embeddings.WriteEmbeddingService) error {
	if embeddingService == nil {
		return fmt.Errorf("embedding service is not initialized")
	}

	start := time.Now()

	if err := embeddingService.GenerateProductEmbeddings(); err != nil {
		if isQuotaError(err) {
			return fmt.Errorf("OpenAI quota exceeded: %v", err)
		}
		return fmt.Errorf("failed to generate product embeddings: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("Successfully generated embeddings in %v\n", duration)
	return nil
}

// formatScheduleDescription returns a human-readable description of the schedule
func formatScheduleDescription(hours int) string {
	switch {
	case hours < 24:
		return fmt.Sprintf("every %d hour(s)", hours)
	case hours == 24:
		return "daily"
	case hours == 168:
		return "weekly"
	case hours == 336:
		return "bi-weekly"
	case hours == 720:
		return "monthly"
	default:
		days := float64(hours) / 24.0
		if days == float64(int(days)) {
			return fmt.Sprintf("every %d day(s)", int(days))
		}
		return fmt.Sprintf("every %d hours", hours)
	}
}
