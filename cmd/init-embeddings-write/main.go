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
	fmt.Println("=== EMBEDDING INIT CONTAINER ===")
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
