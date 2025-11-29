package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/emails"
	"ids/internal/models"
)

func main() {
	// Parse command line flags
	emlPath := flag.String("eml", "", "Path to EML file or directory containing EML files")
	mboxPath := flag.String("mbox", "", "Path to MBOX file")
	generateEmbeddings := flag.Bool("embeddings", true, "Generate embeddings after import")
	flag.Parse()

	if *emlPath == "" && *mboxPath == "" {
		fmt.Println("Usage:")
		fmt.Println("  Import EML files:  import-emails -eml /path/to/file.eml")
		fmt.Println("  Import directory:  import-emails -eml /path/to/directory")
		fmt.Println("  Import MBOX:       import-emails -mbox /path/to/file.mbox")
		fmt.Println("  Skip embeddings:   import-emails -eml /path -embeddings=false")
		os.Exit(1)
	}

	// Load configuration
	cfg := config.Load()

	// Create write database client (local MariaDB for embeddings)
	writeClient, err := database.NewWriteClient(cfg.EmbeddingsDatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}
	defer writeClient.Close()

	// Create email embedding service
	emailService, err := emails.NewEmailEmbeddingService(cfg, writeClient)
	if err != nil {
		log.Fatalf("Failed to create email service: %v", err)
	}

	// Create tables if they don't exist
	fmt.Println("Creating email tables...")
	if err := emailService.CreateEmailTables(); err != nil {
		log.Fatalf("Failed to create email tables: %v", err)
	}

	var parsedEmails []*models.Email
	var parseErr error

	// Parse emails based on input type
	if *emlPath != "" {
		fmt.Printf("Parsing EML from: %s\n", *emlPath)

		// Check if it's a file or directory
		info, err := os.Stat(*emlPath)
		if err != nil {
			log.Fatalf("Failed to access path: %v", err)
		}

		if info.IsDir() {
			fmt.Println("Scanning directory for EML files...")
			parsedEmails, parseErr = emails.ParseDirectory(*emlPath)
		} else if strings.HasSuffix(strings.ToLower(*emlPath), ".eml") {
			email, err := emails.ParseEMLFile(*emlPath)
			if err != nil {
				log.Fatalf("Failed to parse EML file: %v", err)
			}
			parsedEmails = []*models.Email{email}
		} else {
			log.Fatalf("Invalid file type. Expected .eml file or directory")
		}
	} else if *mboxPath != "" {
		fmt.Printf("Parsing MBOX file: %s\n", *mboxPath)
		parsedEmails, parseErr = emails.ParseMBOXFile(*mboxPath)
	}

	if parseErr != nil {
		log.Fatalf("Failed to parse emails: %v", parseErr)
	}

	fmt.Printf("Successfully parsed %d emails\n", len(parsedEmails))

	// Store emails in database
	fmt.Println("Storing emails in database...")
	successCount := 0
	errorCount := 0

	for i, email := range parsedEmails {
		if err := emailService.StoreEmail(email); err != nil {
			fmt.Printf("Warning: Failed to store email %d: %v\n", i+1, err)
			errorCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("Stored %d emails successfully (%d errors)\n", successCount, errorCount)

	// Generate embeddings if requested
	if *generateEmbeddings {
		fmt.Println("\nGenerating embeddings for individual emails...")
		if err := emailService.GenerateEmailEmbeddings(); err != nil {
			log.Printf("Warning: Failed to generate email embeddings: %v", err)
		}

		fmt.Println("\nGenerating embeddings for email threads...")
		if err := emailService.GenerateThreadEmbeddings(); err != nil {
			log.Printf("Warning: Failed to generate thread embeddings: %v", err)
		}

		fmt.Println("Embedding generation complete!")
	}

	fmt.Println("\n✓ Email import complete!")
	fmt.Printf("  - Parsed: %d emails\n", len(parsedEmails))
	fmt.Printf("  - Stored: %d emails\n", successCount)
	if *generateEmbeddings {
		fmt.Println("  - Embeddings: Generated")
	}
}
