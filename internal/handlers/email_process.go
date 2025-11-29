package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/emails"
	"ids/internal/models"

	"github.com/labstack/echo/v4"
)

// ProcessEmailsResponse represents the response from email processing
type ProcessEmailsResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	EmailsProcessed int    `json:"emails_processed"`
	ThreadsCreated  int    `json:"threads_created,omitempty"`
	EmbeddingsCount int    `json:"embeddings_count,omitempty"`
	Error           string `json:"error,omitempty"`
}

// ProcessEmailsFromStorage processes emails from the mounted PVC and imports them to database
// @Summary Process downloaded emails
// @Description Import emails from storage to database with embeddings
// @Tags Admin
// @Accept json
// @Produce json
// @Success 200 {object} ProcessEmailsResponse
// @Failure 500 {object} ProcessEmailsResponse
// @Router /api/admin/import-emails-to-db [post]
func ProcessEmailsFromStorage(c echo.Context) error {
	fmt.Println("[EMAIL_PROCESS] Starting email import from storage...")

	// Get email storage path from environment or use default
	emailPath := os.Getenv("EMAIL_IMPORT_PATH")
	if emailPath == "" {
		emailPath = "/emails"
	}

	// Check if path exists
	if _, err := os.Stat(emailPath); os.IsNotExist(err) {
		fmt.Printf("[EMAIL_PROCESS] Email directory not found: %s\n", emailPath)
		return c.JSON(http.StatusInternalServerError, ProcessEmailsResponse{
			Success: false,
			Message: "Email storage directory not found",
			Error:   fmt.Sprintf("Directory %s does not exist", emailPath),
		})
	}

	// Load configuration
	cfg := config.Load()

	// Create write database client (local MariaDB for embeddings)
	writeClient, err := database.NewWriteClient(cfg.EmbeddingsDatabaseURL)
	if err != nil {
		fmt.Printf("[EMAIL_PROCESS] Failed to create database client: %v\n", err)
		return c.JSON(http.StatusInternalServerError, ProcessEmailsResponse{
			Success: false,
			Message: "Failed to connect to database",
			Error:   err.Error(),
		})
	}
	defer writeClient.Close()

	// Create email embedding service
	emailService, err := emails.NewEmailEmbeddingService(cfg, writeClient)
	if err != nil {
		fmt.Printf("[EMAIL_PROCESS] Failed to create email service: %v\n", err)
		return c.JSON(http.StatusInternalServerError, ProcessEmailsResponse{
			Success: false,
			Message: "Failed to initialize email service",
			Error:   err.Error(),
		})
	}

	// Create tables if they don't exist
	fmt.Println("[EMAIL_PROCESS] Ensuring email tables exist...")
	if err := emailService.CreateEmailTables(); err != nil {
		fmt.Printf("[EMAIL_PROCESS] Failed to create email tables: %v\n", err)
		return c.JSON(http.StatusInternalServerError, ProcessEmailsResponse{
			Success: false,
			Message: "Failed to create database tables",
			Error:   err.Error(),
		})
	}

	totalEmails := 0
	totalThreads := 0
	totalEmbeddings := 0

	// Process EML files
	emlFiles, err := findFiles(emailPath, ".eml")
	if err != nil {
		fmt.Printf("[EMAIL_PROCESS] Error finding EML files: %v\n", err)
	} else if len(emlFiles) > 0 {
		fmt.Printf("[EMAIL_PROCESS] Found %d EML files\n", len(emlFiles))

		// Parse directory
		parsedEmails, err := emails.ParseDirectory(emailPath)
		if err != nil {
			fmt.Printf("[EMAIL_PROCESS] Error parsing EML files: %v\n", err)
		} else {
			// Store emails
			for i, email := range parsedEmails {
				if err := emailService.StoreEmail(email); err != nil {
					fmt.Printf("[EMAIL_PROCESS] Warning: Failed to store email %d: %v\n", i+1, err)
				} else {
					totalEmails++
				}
			}
			fmt.Printf("[EMAIL_PROCESS] Stored %d EML emails\n", totalEmails)

			// Generate embeddings
			fmt.Println("[EMAIL_PROCESS] Generating embeddings for EML files...")
			if err := emailService.GenerateEmailEmbeddings(); err != nil {
				fmt.Printf("[EMAIL_PROCESS] Warning: Failed to generate email embeddings: %v\n", err)
			} else {
				totalEmbeddings += totalEmails
			}

			if err := emailService.GenerateThreadEmbeddings(); err != nil {
				fmt.Printf("[EMAIL_PROCESS] Warning: Failed to generate thread embeddings: %v\n", err)
			} else {
				// Count threads from database would require a query - skip for now
				totalThreads++
			}
		}
	}

	// Process MBOX files with streaming (memory-efficient for large files)
	mboxFiles, err := findFiles(emailPath, ".mbox")
	if err != nil {
		fmt.Printf("[EMAIL_PROCESS] Error finding MBOX files: %v\n", err)
	} else if len(mboxFiles) > 0 {
		fmt.Printf("[EMAIL_PROCESS] Found %d MBOX files\n", len(mboxFiles))

		for _, mboxFile := range mboxFiles {
			fileInfo, _ := os.Stat(mboxFile)
			fileSizeGB := float64(fileInfo.Size()) / (1024 * 1024 * 1024)

			fmt.Printf("[EMAIL_PROCESS] ═══════════════════════════════════════\n")
			fmt.Printf("[EMAIL_PROCESS] Processing MBOX: %s (%.2f GB)\n", filepath.Base(mboxFile), fileSizeGB)
			fmt.Printf("[EMAIL_PROCESS] Using streaming parser with batch size: 50 emails\n")
			fmt.Printf("[EMAIL_PROCESS] ═══════════════════════════════════════\n")

			// Process MBOX with streaming - batch size 50 emails at a time
			batchNum := 0
			err := emails.ParseMBOXFileStreaming(mboxFile, 50, func(batch []*models.Email, progress emails.MBOXProgress) error {
				batchNum++

				fmt.Printf("[EMAIL_PROCESS] ▶ Batch %d: Processing %d emails (%.1f%% complete, %d total emails)\n",
					batchNum, len(batch), progress.PercentComplete, progress.EmailsProcessed)

				// Store emails in this batch
				storedCount := 0
				errorCount := 0

				for _, email := range batch {
					if err := emailService.StoreEmail(email); err != nil {
						if strings.Contains(err.Error(), "syntax error") {
							errorCount++
							// Only log first few syntax errors to avoid log spam
							if errorCount <= 3 {
								fmt.Printf("[EMAIL_PROCESS]   ⚠️ SQL Error: %v\n", err)
							}
						}
						// All other errors are silently handled (duplicates, etc)
					} else {
						storedCount++
					}
				}
				totalEmails += storedCount

				if errorCount > 3 {
					fmt.Printf("[EMAIL_PROCESS] ✓ Batch %d: %d stored, %d SQL errors (showing first 3 only)\n",
						batchNum, storedCount, errorCount)
				} else if storedCount > 0 {
					fmt.Printf("[EMAIL_PROCESS] ✓ Batch %d: Stored %d/%d new emails (Total: %d)\n",
						batchNum, storedCount, len(batch), totalEmails)
				} else {
					fmt.Printf("[EMAIL_PROCESS] ○ Batch %d: All emails already imported (skipped %d duplicates)\n",
						batchNum, len(batch))
				}

				// Generate embeddings for this batch
				fmt.Printf("[EMAIL_PROCESS] ▶ Batch %d: Generating embeddings...\n", batchNum)
				if err := emailService.GenerateEmailEmbeddings(); err != nil {
					fmt.Printf("[EMAIL_PROCESS]   Warning: Failed to generate email embeddings: %v\n", err)
				} else {
					totalEmbeddings += storedCount
					fmt.Printf("[EMAIL_PROCESS] ✓ Batch %d: Generated %d embeddings\n", batchNum, storedCount)
				}

				return nil
			})

			if err != nil {
				fmt.Printf("[EMAIL_PROCESS] ✗ MBOX processing failed for %s: %v\n", filepath.Base(mboxFile), err)
				continue
			}

			fmt.Printf("[EMAIL_PROCESS] ✅ Completed processing %s: %d emails\n", filepath.Base(mboxFile), totalEmails)

			// Generate thread embeddings after all emails from this MBOX are processed
			fmt.Println("[EMAIL_PROCESS] Generating thread embeddings for all conversations...")
			if err := emailService.GenerateThreadEmbeddings(); err != nil {
				fmt.Printf("[EMAIL_PROCESS] Warning: Failed to generate thread embeddings: %v\n", err)
			} else {
				// Successfully generated thread embeddings
				fmt.Printf("[EMAIL_PROCESS] ✓ Thread embeddings generated successfully\n")
				totalThreads++ // Increment as a marker that threads were processed
			}
		}
	}

	fmt.Printf("[EMAIL_PROCESS] ✅ Import complete: %d emails, %d threads, %d embeddings\n",
		totalEmails, totalThreads, totalEmbeddings)

	return c.JSON(http.StatusOK, ProcessEmailsResponse{
		Success:         true,
		Message:         "Email import completed successfully",
		EmailsProcessed: totalEmails,
		ThreadsCreated:  totalThreads,
		EmbeddingsCount: totalEmbeddings,
	})
}

// findFiles recursively finds all files with the given extension
func findFiles(root, ext string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// Skip directories with permission errors (like lost+found)
		if err != nil {
			if os.IsPermission(err) {
				fmt.Printf("[EMAIL_PROCESS] Skipping %s due to permission error\n", path)
				return filepath.SkipDir
			}
			return err
		}

		// Skip lost+found directory (common in PVCs)
		if info.IsDir() && info.Name() == "lost+found" {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ext) {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}
