package emails

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/models"
	"ids/internal/vectordb"

	"github.com/sashabaranov/go-openai"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EmailEmbeddingService handles vector embeddings for emails
type EmailEmbeddingService struct {
	client       *openai.Client
	db           *database.WriteClient
	cache        *cache.Cache
	qdrantClient *vectordb.QdrantClient // Qdrant client for dual-write (optional)
}

// NewEmailEmbeddingService creates a new email embedding service
// embeddingCache: Optional cache for query embeddings (can be nil)
func NewEmailEmbeddingService(cfg *config.Config, writeClient *database.WriteClient, embeddingCache ...*cache.Cache) (*EmailEmbeddingService, error) {
	client := openai.NewClient(cfg.OpenAIKey)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{"test"},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OpenAI API: %v", err)
	}

	service := &EmailEmbeddingService{
		client: client,
		db:     writeClient,
	}

	// Set cache if provided
	if len(embeddingCache) > 0 && embeddingCache[0] != nil {
		service.cache = embeddingCache[0]
	}

	return service, nil
}

// SetQdrantClient sets the Qdrant client for dual-write
func (ees *EmailEmbeddingService) SetQdrantClient(client *vectordb.QdrantClient) {
	ees.qdrantClient = client
	if client != nil {
		fmt.Printf("[EMAIL_EMBEDDINGS] Qdrant dual-write enabled for email threads\n")
	}
}

// CreateEmailTables creates the necessary database tables (PostgreSQL-compatible with pgvector)
func (ees *EmailEmbeddingService) CreateEmailTables() error {
	// Enable pgvector extension first
	if _, err := ees.db.ExecuteWriteQuery(`CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		fmt.Printf("Warning: Failed to create vector extension (may already exist): %v\n", err)
	}

	queries := []string{
		// Emails table
		`CREATE TABLE IF NOT EXISTS emails (
			id SERIAL PRIMARY KEY,
			message_id VARCHAR(255) UNIQUE NOT NULL,
			subject TEXT NOT NULL,
			from_addr TEXT NOT NULL,
			to_addr TEXT NOT NULL,
			date TIMESTAMP NOT NULL,
			body TEXT NOT NULL,
			thread_id VARCHAR(255),
			in_reply_to VARCHAR(255),
			"references" TEXT,
			is_customer BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Email threads table
		`CREATE TABLE IF NOT EXISTS email_threads (
			thread_id VARCHAR(255) PRIMARY KEY,
			subject TEXT NOT NULL,
			email_count INT DEFAULT 1,
			first_date TIMESTAMP NOT NULL,
			last_date TIMESTAMP NOT NULL,
			summary TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Email embeddings table - using pgvector for 1536-dimensional embeddings
		`CREATE TABLE IF NOT EXISTS email_embeddings (
			id SERIAL PRIMARY KEY,
			email_id INT,
			thread_id VARCHAR(255),
			embedding vector(1536) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (email_id),
			UNIQUE (thread_id),
			FOREIGN KEY (email_id) REFERENCES emails(id) ON DELETE CASCADE
		)`,
	}

	for _, query := range queries {
		if _, err := ees.db.ExecuteWriteQuery(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create indexes separately
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_emails_message_id ON emails(message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_thread_id ON emails(thread_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(date)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_is_customer ON emails(is_customer)`,
		`CREATE INDEX IF NOT EXISTS idx_email_threads_first_date ON email_threads(first_date)`,
		`CREATE INDEX IF NOT EXISTS idx_email_threads_last_date ON email_threads(last_date)`,
		// HNSW index for fast cosine similarity search with pgvector
		// m=16: number of connections per layer (higher = better recall, more memory)
		// ef_construction=100: size of dynamic candidate list for construction (higher = better index quality, slower build)
		`CREATE INDEX IF NOT EXISTS idx_email_embeddings_hnsw ON email_embeddings USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 100)`,
	}

	for _, query := range indexes {
		if _, err := ees.db.ExecuteWriteQuery(query); err != nil {
			// Ignore errors for index creation (they might already exist)
			fmt.Printf("Warning: Failed to create index: %v\n", err)
		}
	}

	return nil
}

// StoreEmail stores an email in the database
func (ees *EmailEmbeddingService) StoreEmail(email *models.Email) error {
	// Generate thread ID
	threadID := GenerateThreadID(email)
	email.ThreadID = &threadID

	query := `
		INSERT INTO emails (message_id, subject, from_addr, to_addr, date, body, thread_id, in_reply_to, "references", is_customer)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (message_id) DO UPDATE SET
			subject = EXCLUDED.subject,
			from_addr = EXCLUDED.from_addr,
			to_addr = EXCLUDED.to_addr,
			date = EXCLUDED.date,
			body = EXCLUDED.body,
			thread_id = EXCLUDED.thread_id,
			in_reply_to = EXCLUDED.in_reply_to,
			"references" = EXCLUDED."references",
			is_customer = EXCLUDED.is_customer,
			updated_at = CURRENT_TIMESTAMP
	`

	result, err := ees.db.ExecuteWriteQuery(query,
		email.MessageID,
		email.Subject,
		email.From,
		email.To,
		email.Date,
		email.Body,
		email.ThreadID,
		email.InReplyTo,
		email.References,
		email.IsCustomer,
	)

	if err != nil {
		errStr := err.Error()

		// Handle different error types gracefully
		if strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "unique constraint") {
			// This is expected during re-imports - email already exists
			// Silently continue (ON CONFLICT should handle this, but just in case)
			return nil
		}

		if strings.Contains(errStr, "syntax error") {
			// This is a real SQL error - log details for debugging
			fmt.Printf("[EMAIL_STORE] ⚠️  SQL Syntax Error:\n")
			fmt.Printf("  Message-ID: %s\n", email.MessageID)
			fmt.Printf("  Subject: %s\n", email.Subject[:min(50, len(email.Subject))])
			fmt.Printf("  Error: %v\n", err)
			return fmt.Errorf("SQL syntax error: %w", err)
		}

		// Other errors - log and return
		return fmt.Errorf("failed to store email: %w", err)
	}

	// Check if this was an insert or update
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Email already exists and unchanged
		return nil
	}

	// Update thread information
	return ees.updateThread(threadID, email)
}

// updateThread updates or creates a thread entry
func (ees *EmailEmbeddingService) updateThread(threadID string, email *models.Email) error {
	// Check if thread exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM email_threads WHERE thread_id = ?)`
	rows, err := ees.db.GetDB().Query(checkQuery, threadID)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: Error closing rows: %v\n", err)
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return fmt.Errorf("failed to scan exists check: %w", err)
		}
	}

	if exists {
		// Update existing thread
		updateQuery := `
			UPDATE email_threads 
			SET email_count = email_count + 1,
			    last_date = GREATEST(last_date, ?),
			    first_date = LEAST(first_date, ?),
			    updated_at = NOW()
			WHERE thread_id = ?
		`
		_, err = ees.db.ExecuteWriteQuery(updateQuery, email.Date, email.Date, threadID)
	} else {
		// Create new thread
		insertQuery := `
			INSERT INTO email_threads (thread_id, subject, email_count, first_date, last_date)
			VALUES ($1, $2, 1, $3, $4)
			ON CONFLICT (thread_id) DO UPDATE SET
				email_count = email_threads.email_count + 1,
				last_date = CASE WHEN EXCLUDED.last_date > email_threads.last_date THEN EXCLUDED.last_date ELSE email_threads.last_date END,
				first_date = CASE WHEN EXCLUDED.first_date < email_threads.first_date THEN EXCLUDED.first_date ELSE email_threads.first_date END,
				updated_at = CURRENT_TIMESTAMP
		`
		_, err = ees.db.ExecuteWriteQuery(insertQuery, threadID, email.Subject, email.Date, email.Date)
	}

	return err
}

// EmailEmbeddingStats contains statistics about email embedding generation
type EmailEmbeddingStats struct {
	EmailsProcessed  int
	ThreadsProcessed int
	Success          bool
}

// GenerateEmailEmbeddings generates embeddings for all emails without embeddings
func (ees *EmailEmbeddingService) GenerateEmailEmbeddings() error {
	_, err := ees.GenerateEmailEmbeddingsWithStats()
	return err
}

// GenerateEmailEmbeddingsWithStats generates embeddings and returns statistics
func (ees *EmailEmbeddingService) GenerateEmailEmbeddingsWithStats() (*EmailEmbeddingStats, error) {
	stats := &EmailEmbeddingStats{}
	fmt.Println("[EMAIL_EMBEDDINGS] Starting email embedding generation...")

	// Get emails without embeddings
	query := `
		SELECT e.id, e.message_id, e.subject, e.from_addr, e.to_addr, e.date, 
		       e.body, e.thread_id, e.in_reply_to, e."references", e.is_customer
		FROM emails e
		LEFT JOIN email_embeddings ee ON ee.email_id = e.id
		WHERE ee.id IS NULL
		ORDER BY e.date DESC
	`

	rows, err := ees.db.GetDB().Query(query)
	if err != nil {
		return stats, fmt.Errorf("failed to fetch emails: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: Error closing rows: %v\n", err)
		}
	}()

	var emails []models.Email
	for rows.Next() {
		var email models.Email
		var threadID, inReplyTo, references *string
		// PostgreSQL returns timestamps directly as time.Time
		err := rows.Scan(
			&email.ID,
			&email.MessageID,
			&email.Subject,
			&email.From,
			&email.To,
			&email.Date,
			&email.Body,
			&threadID,
			&inReplyTo,
			&references,
			&email.IsCustomer,
		)
		if err != nil {
			fmt.Printf("[EMAIL_EMBEDDINGS] Warning: Failed to scan email: %v\n", err)
			continue
		}

		email.ThreadID = threadID
		email.InReplyTo = inReplyTo
		email.References = references
		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return stats, fmt.Errorf("failed to iterate emails: %w", err)
	}

	fmt.Printf("[EMAIL_EMBEDDINGS] Found %d emails to process\n", len(emails))
	stats.EmailsProcessed = len(emails)

	// Process in batches
	batchSize := 50
	for i := 0; i < len(emails); i += batchSize {
		end := i + batchSize
		if end > len(emails) {
			end = len(emails)
		}

		batch := emails[i:end]
		fmt.Printf("[EMAIL_EMBEDDINGS] Processing batch %d-%d...\n", i+1, end)

		if err := ees.processEmailBatch(batch); err != nil {
			fmt.Printf("[EMAIL_EMBEDDINGS] Error processing batch: %v\n", err)
			// Continue with next batch
		}
	}

	fmt.Println("[EMAIL_EMBEDDINGS] Email embedding generation complete")
	stats.Success = true
	return stats, nil
}

// processEmailBatch processes a batch of emails and generates embeddings
func (ees *EmailEmbeddingService) processEmailBatch(emails []models.Email) error {
	// Build texts for embedding
	texts := make([]string, len(emails))
	for i, email := range emails {
		texts[i] = ees.buildEmailText(email)
	}

	// Generate embeddings
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := ees.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Store embeddings
	for i, embeddingData := range resp.Data {
		email := emails[i]
		embedding := make([]float64, len(embeddingData.Embedding))
		for j, v := range embeddingData.Embedding {
			embedding[j] = float64(v)
		}

		if err := ees.storeEmailEmbedding(email.ID, nil, embedding); err != nil {
			fmt.Printf("[EMAIL_EMBEDDINGS] Failed to store embedding for email %d: %v\n", email.ID, err)
		}
	}

	return nil
}

// GenerateThreadEmbeddings generates embeddings for email threads
func (ees *EmailEmbeddingService) GenerateThreadEmbeddings() error {
	_, err := ees.GenerateThreadEmbeddingsWithStats()
	return err
}

// GenerateThreadEmbeddingsWithStats generates thread embeddings and returns statistics
func (ees *EmailEmbeddingService) GenerateThreadEmbeddingsWithStats() (int, error) {
	fmt.Println("[THREAD_EMBEDDINGS] Starting thread embedding generation...")

	// Get threads without thread-level embeddings
	// Note: email_embeddings stores both individual emails (email_id set) and thread embeddings (email_id NULL)
	query := `
		SELECT et.thread_id, et.subject, et.email_count, et.first_date, et.last_date
		FROM email_threads et
		LEFT JOIN email_embeddings ee ON ee.thread_id = et.thread_id AND ee.email_id IS NULL
		WHERE ee.id IS NULL AND et.email_count >= 2
		ORDER BY et.last_date DESC
	`

	rows, err := ees.db.GetDB().Query(query)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch threads: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: Error closing rows: %v\n", err)
		}
	}()

	var threads []models.EmailThread
	for rows.Next() {
		var thread models.EmailThread
		// PostgreSQL returns timestamps directly as time.Time, no parsing needed
		err := rows.Scan(
			&thread.ThreadID,
			&thread.Subject,
			&thread.EmailCount,
			&thread.FirstDate,
			&thread.LastDate,
		)
		if err != nil {
			fmt.Printf("[THREAD_EMBEDDINGS] Warning: Failed to scan thread: %v\n", err)
			continue
		}

		threads = append(threads, thread)
	}

	if err = rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to iterate threads: %w", err)
	}

	fmt.Printf("[THREAD_EMBEDDINGS] Found %d threads to process\n", len(threads))

	// Process threads
	for _, thread := range threads {
		if err := ees.generateThreadEmbedding(thread.ThreadID); err != nil {
			fmt.Printf("[THREAD_EMBEDDINGS] Error processing thread %s: %v\n", thread.ThreadID, err)
		}
	}

	fmt.Println("[THREAD_EMBEDDINGS] Thread embedding generation complete")
	return len(threads), nil
}

// generateThreadEmbedding generates an embedding for a complete thread
func (ees *EmailEmbeddingService) generateThreadEmbedding(threadID string) error {
	// Get all emails in thread
	query := `
		SELECT id, message_id, subject, from_addr, to_addr, date, body, thread_id, 
		       in_reply_to, "references", is_customer
		FROM emails
		WHERE thread_id = $1
		ORDER BY date ASC
	`

	rows, err := ees.db.GetDB().Query(query, threadID)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: Error closing rows: %v\n", err)
		}
	}()

	var emails []models.Email
	for rows.Next() {
		var email models.Email
		var threadIDPtr, inReplyTo, references *string
		// PostgreSQL returns timestamps directly as time.Time
		err := rows.Scan(
			&email.ID,
			&email.MessageID,
			&email.Subject,
			&email.From,
			&email.To,
			&email.Date,
			&email.Body,
			&threadIDPtr,
			&inReplyTo,
			&references,
			&email.IsCustomer,
		)
		if err != nil {
			return err
		}

		email.ThreadID = threadIDPtr
		email.InReplyTo = inReplyTo
		email.References = references
		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	if len(emails) == 0 {
		return nil
	}

	// Build thread text (conversation flow)
	text := ees.buildThreadText(emails)

	// Generate embedding
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := ees.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return err
	}

	embedding := make([]float64, len(resp.Data[0].Embedding))
	for j, v := range resp.Data[0].Embedding {
		embedding[j] = float64(v)
	}

	return ees.storeEmailEmbedding(0, &threadID, embedding)
}

// buildEmailText creates text representation for a single email
func (ees *EmailEmbeddingService) buildEmailText(email models.Email) string {
	var parts []string

	parts = append(parts, "Subject: "+email.Subject)

	if email.IsCustomer {
		parts = append(parts, "From: Customer")
	} else {
		parts = append(parts, "From: Support")
	}

	// Clean and truncate body
	body := email.Body
	body = strings.TrimSpace(body)
	if len(body) > 2000 {
		body = body[:2000] + "..."
	}
	parts = append(parts, "Message: "+body)

	return strings.Join(parts, " | ")
}

// buildThreadText creates text representation for an entire thread
func (ees *EmailEmbeddingService) buildThreadText(emails []models.Email) string {
	var parts []string

	parts = append(parts, "Thread: "+emails[0].Subject)

	for _, email := range emails {
		var role string
		if email.IsCustomer {
			role = "Customer"
		} else {
			role = "Support"
		}

		body := strings.TrimSpace(email.Body)
		if len(body) > 500 {
			body = body[:500] + "..."
		}

		parts = append(parts, fmt.Sprintf("%s: %s", role, body))
	}

	return strings.Join(parts, " | ")
}

// storeEmailEmbedding stores an embedding for an email or thread using pgvector
// Also writes thread embeddings to Qdrant if dual-write is enabled
func (ees *EmailEmbeddingService) storeEmailEmbedding(emailID int, threadID *string, embedding []float64) error {
	// Convert embedding to pgvector format
	embeddingStr := formatVectorForPgvector(embedding)

	var query string
	var args []interface{}

	if threadID != nil {
		query = `
			INSERT INTO email_embeddings (thread_id, embedding)
			VALUES ($1, $2::vector)
			ON CONFLICT (thread_id) DO UPDATE SET
				embedding = EXCLUDED.embedding,
				updated_at = CURRENT_TIMESTAMP
		`
		args = []interface{}{*threadID, embeddingStr}
	} else {
		query = `
			INSERT INTO email_embeddings (email_id, embedding)
			VALUES ($1, $2::vector)
			ON CONFLICT (email_id) DO UPDATE SET
				embedding = EXCLUDED.embedding,
				updated_at = CURRENT_TIMESTAMP
		`
		args = []interface{}{emailID, embeddingStr}
	}

	_, err := ees.db.ExecuteWriteQuery(query, args...)
	if err != nil {
		return fmt.Errorf("failed to store embedding in PostgreSQL: %w", err)
	}

	// Dual-write thread embeddings to Qdrant if enabled
	if ees.qdrantClient != nil && threadID != nil {
		// Convert float64 to float32 for Qdrant
		embedding32 := make([]float32, len(embedding))
		for i, v := range embedding {
			embedding32[i] = float32(v)
		}

		// Get thread info from database for payload
		payload := vectordb.EmailPayload{
			ThreadID: *threadID,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := ees.qdrantClient.UpsertEmailThread(ctx, *threadID, embedding32, payload); err != nil {
			// Log error but don't fail - PostgreSQL is the primary store
			fmt.Printf("[EMAIL_EMBEDDINGS] Warning: Failed to write thread to Qdrant: %v\n", err)
		}
	}

	return nil
}

// formatVectorForPgvector converts a float64 slice to pgvector string format
func formatVectorForPgvector(embedding []float64) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// formatFloat32VectorForPgvector converts a float32 slice to pgvector string format
func formatFloat32VectorForPgvector(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// SearchSimilarEmails finds emails or threads similar to a query using pgvector
func (ees *EmailEmbeddingService) SearchSimilarEmails(query string, limit int, searchThreads bool) ([]models.EmailSearchResult, error) {
	searchType := "individual emails"
	if searchThreads {
		searchType = "email threads"
	}
	fmt.Printf("[EMAIL_EMBEDDINGS] 🔍 Querying EMAIL EMBEDDINGS datasource - Query: '%s', Limit: %d, Type: %s\n", query, limit, searchType)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get embedding from cache first
	var queryEmbedding []float32
	if ees.cache != nil {
		if cachedEmbedding, found := ees.cache.GetEmbedding(query); found {
			fmt.Printf("[EMAIL_EMBEDDINGS] ✓ Cache HIT - using cached query embedding\n")
			queryEmbedding = cachedEmbedding
		}
	}

	// Generate embedding if not in cache
	if queryEmbedding == nil {
		fmt.Printf("[EMAIL_EMBEDDINGS] Generating query embedding...\n")
		resp, err := ees.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: []string{query},
			Model: openai.SmallEmbedding3,
		})
		if err != nil {
			fmt.Printf("[EMAIL_EMBEDDINGS] ❌ ERROR: Failed to generate query embedding: %v\n", err)
			return nil, err
		}
		queryEmbedding = resp.Data[0].Embedding

		// Store in cache for future requests
		if ees.cache != nil {
			ees.cache.SetEmbedding(query, queryEmbedding)
			fmt.Printf("[EMAIL_EMBEDDINGS] ✓ Cached query embedding for future use\n")
		}
	}

	fmt.Printf("[EMAIL_EMBEDDINGS] Query embedding ready (dimensions: %d)\n", len(queryEmbedding))

	// Convert query embedding to pgvector format
	queryVectorStr := formatFloat32VectorForPgvector(queryEmbedding)

	// Use pgvector for similarity search - database calculates similarity
	// CTE-based queries for better performance with HNSW index
	var dbQuery string
	if searchThreads {
		// Use CTE to first get top similar threads, then join with metadata
		// This leverages the HNSW index before doing expensive JOINs
		dbQuery = `
			WITH ranked_threads AS (
				SELECT thread_id,
				       1 - (embedding <=> $1::vector) AS similarity
				FROM email_embeddings
				WHERE thread_id IS NOT NULL
				ORDER BY embedding <=> $1::vector
				LIMIT $2
			)
			SELECT '' as embedding_str, e.id, e.message_id, e.subject, e.from_addr, e.to_addr,
			       e.date, e.body, e.thread_id, e.is_customer,
			       et.thread_id, et.subject, et.email_count, et.first_date, et.last_date,
			       rt.similarity
			FROM ranked_threads rt
			JOIN email_threads et ON et.thread_id = rt.thread_id
			JOIN LATERAL (
				SELECT * FROM emails 
				WHERE emails.thread_id = rt.thread_id 
				ORDER BY date DESC 
				LIMIT 1
			) e ON true
			ORDER BY rt.similarity DESC
		`
	} else {
		dbQuery = `
			SELECT ee.embedding::text, e.id, e.message_id, e.subject, e.from_addr, e.to_addr,
			       e.date, e.body, e.thread_id, e.is_customer,
			       1 - (ee.embedding <=> $1::vector) AS similarity
			FROM email_embeddings ee
			JOIN emails e ON e.id = ee.email_id
			WHERE ee.email_id IS NOT NULL
			ORDER BY ee.embedding <=> $1::vector
			LIMIT $2
		`
	}

	var rows interface{ Close() error }
	var scanErr error

	var results []models.EmailSearchResult

	if searchThreads {
		// Thread search with CTE - uses limit parameter
		rowsResult, err := ees.db.GetDB().Query(dbQuery, queryVectorStr, limit)
		if err != nil {
			return nil, err
		}
		rows = rowsResult
		defer func() {
			if err := rows.Close(); err != nil {
				fmt.Printf("Warning: Error closing rows: %v\n", err)
			}
		}()

		for rowsResult.Next() {
			var embeddingStr string
			var email models.Email
			var threadID, threadSubject *string
			var emailCount *int
			var firstDate, lastDate *time.Time
			var similarity float64

			scanErr = rowsResult.Scan(
				&embeddingStr,
				&email.ID, &email.MessageID, &email.Subject, &email.From, &email.To,
				&email.Date, &email.Body, &email.ThreadID, &email.IsCustomer,
				&threadID, &threadSubject, &emailCount, &firstDate, &lastDate,
				&similarity,
			)

			if scanErr != nil {
				fmt.Printf("[EMAIL_EMBEDDINGS] Warning: Failed to scan row: %v\n", scanErr)
				continue
			}

			result := models.EmailSearchResult{
				Email:      email,
				Similarity: similarity,
				Embedding:  nil, // Don't need to store embedding in results
			}

			if threadID != nil {
				result.Thread = &models.EmailThread{
					ThreadID:   *threadID,
					Subject:    *threadSubject,
					EmailCount: *emailCount,
					FirstDate:  *firstDate,
					LastDate:   *lastDate,
				}
			}

			results = append(results, result)
		}
		// Results are already sorted by similarity and limited by the CTE query
	} else {
		// Individual email search with pgvector ORDER BY
		rowsResult, err := ees.db.GetDB().Query(dbQuery, queryVectorStr, limit)
		if err != nil {
			return nil, err
		}
		rows = rowsResult
		defer func() {
			if err := rows.Close(); err != nil {
				fmt.Printf("Warning: Error closing rows: %v\n", err)
			}
		}()

		for rowsResult.Next() {
			var embeddingStr string
			var email models.Email
			var similarity float64

			scanErr = rowsResult.Scan(
				&embeddingStr,
				&email.ID, &email.MessageID, &email.Subject, &email.From, &email.To,
				&email.Date, &email.Body, &email.ThreadID, &email.IsCustomer,
				&similarity,
			)

			if scanErr != nil {
				fmt.Printf("[EMAIL_EMBEDDINGS] Warning: Failed to scan row: %v\n", scanErr)
				continue
			}

			result := models.EmailSearchResult{
				Email:      email,
				Similarity: similarity,
				Embedding:  nil, // Don't need to store embedding in results
			}

			results = append(results, result)
		}
	}

	fmt.Printf("[EMAIL_EMBEDDINGS] ✅ EMAIL EMBEDDINGS query complete - Returning %d %s\n", len(results), searchType)
	if len(results) > 0 {
		fmt.Printf("[EMAIL_EMBEDDINGS] Top result similarity: %.3f\n", results[0].Similarity)
	}

	return results, nil
}
