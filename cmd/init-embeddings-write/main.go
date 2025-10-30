package main

import (
	"fmt"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/embeddings"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("=== EMBEDDING INIT CONTAINER ===")
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

	// Generate embeddings for all products
	fmt.Println("Generating embeddings for all products...")
	start := time.Now()

	if err := embeddingService.GenerateProductEmbeddings(); err != nil {
		log.Fatal("Failed to generate product embeddings:", err)
	}

	duration := time.Since(start)
	fmt.Printf("Successfully generated embeddings in %v\n", duration)
	fmt.Printf("Completed at: %s\n", time.Now().Format(time.RFC3339))

	// Exit with success
	os.Exit(0)
}
