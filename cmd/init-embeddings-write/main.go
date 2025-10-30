package main

import (
	"fmt"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/embeddings"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Println("=== EMBEDDING DAILY SERVICE ===")
	fmt.Printf("Starting at: %s\n", time.Now().Format(time.RFC3339))

	// Load configuration
	cfg := config.Load()

	// Wait for SSH tunnel if WAIT_FOR_TUNNEL is set
	if os.Getenv("WAIT_FOR_TUNNEL") == "true" {
		fmt.Println("Waiting for SSH tunnel to be ready...")
		tunnelReadyPath := "/shared/tunnel-ready"
		for i := 0; i < 60; i++ {
			if _, err := os.Stat(tunnelReadyPath); err == nil {
				fmt.Println("SSH tunnel is ready!")
				break
			}
			if i == 59 {
				log.Fatal("SSH tunnel did not become ready after 60 seconds")
			}
			fmt.Printf("Attempt %d/60: Tunnel not ready yet, waiting...\n", i+1)
			time.Sleep(1 * time.Second)
		}
	}

	// Initialize write-enabled database connection
	fmt.Println("Connecting to database with write access...")
	writeClient, err := database.NewWriteClient(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database with write access:", err)
	}
	defer writeClient.Close()

	// Create write-enabled embedding service
	fmt.Println("Initializing embedding service...")
	embeddingService, err := embeddings.NewWriteEmbeddingService(cfg, writeClient)
	if err != nil {
		log.Fatal("Failed to create embedding service:", err)
	}

	// Create embeddings table if it doesn't exist
	fmt.Println("Creating embeddings table...")
	if err := embeddingService.CreateEmbeddingsTable(); err != nil {
		log.Fatal("Failed to create embeddings table:", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run initial embedding generation
	fmt.Println("Running initial embedding generation...")
	if err := runEmbeddingGeneration(embeddingService); err != nil {
		log.Printf("ERROR: Initial embedding generation failed: %v", err)
	} else {
		fmt.Println("Initial embedding generation completed successfully")
	}

	// Set up daily execution
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	fmt.Println("Embedding service is now running. Will regenerate embeddings daily.")
	fmt.Println("Press Ctrl+C to stop the service.")

	// Main loop
	for {
		select {
		case <-ticker.C:
			fmt.Printf("\n=== DAILY EMBEDDING GENERATION TRIGGERED ===\n")
			fmt.Printf("Starting at: %s\n", time.Now().Format(time.RFC3339))

			if err := runEmbeddingGeneration(embeddingService); err != nil {
				log.Printf("ERROR: Daily embedding generation failed: %v", err)
				// Continue running even if one generation fails
			} else {
				fmt.Printf("Daily embedding generation completed successfully at: %s\n", time.Now().Format(time.RFC3339))
			}

		case sig := <-sigChan:
			fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)
			return
		}
	}
}

// runEmbeddingGeneration runs the embedding generation process
func runEmbeddingGeneration(embeddingService *embeddings.WriteEmbeddingService) error {
	start := time.Now()

	if err := embeddingService.GenerateProductEmbeddings(); err != nil {
		return fmt.Errorf("failed to generate product embeddings: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("Successfully generated embeddings in %v\n", duration)
	return nil
}
