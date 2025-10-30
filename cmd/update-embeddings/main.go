package main

import (
	"fmt"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/embeddings"
	"log"
	"os"
	"time"
)

func main() {
	fmt.Println("=== EMBEDDING UPDATE CRON JOB ===")
	fmt.Printf("Starting at: %s\n", time.Now().Format(time.RFC3339))

	// Load configuration
	cfg := config.Load()

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

	// Check if embeddings table exists
	fmt.Println("Checking embeddings table...")
	if err := embeddingService.CreateEmbeddingsTable(); err != nil {
		log.Fatal("Failed to create/verify embeddings table:", err)
	}

	// Update embeddings for all products
	fmt.Println("Updating embeddings for all products...")
	start := time.Now()

	if err := embeddingService.GenerateProductEmbeddings(); err != nil {
		log.Fatal("Failed to update product embeddings:", err)
	}

	duration := time.Since(start)
	fmt.Printf("Successfully updated embeddings in %v\n", duration)
	fmt.Printf("Completed at: %s\n", time.Now().Format(time.RFC3339))

	// Exit with success
	os.Exit(0)
}
