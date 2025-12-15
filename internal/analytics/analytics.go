package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ids/internal/database"
	"ids/internal/models"
)

// EventType constants for tracking different events
const (
	EventConversation         = "conversation"
	EventProductSuggestion    = "product_suggestion"
	EventEmailImport          = "email_import"
	EventSupportEscalation    = "support_escalation"
	EventOpenAICall           = "openai_call"
	EventSendGridCall         = "sendgrid_call"
	EventProductEmbeddings    = "product_embeddings"
	EventEmailEmbeddings      = "email_embeddings"
	EventThreadEmbeddings     = "thread_embeddings"
	EventQueryEmbedding       = "query_embedding"       // Per-search embedding generation (billable)
	EventSupportSummarization = "support_summarization" // GPT call for support summary (billable)
)

// Period constants for analytics queries
const (
	PeriodToday      = "today"
	PeriodYesterday  = "yesterday"
	PeriodLast7Days  = "last_7_days"
	PeriodLast30Days = "last_30_days"
)

// Service handles analytics tracking and retrieval
type Service struct {
	writeClient *database.WriteClient
	mu          sync.Mutex
}

// NewService creates a new analytics service
func NewService(writeClient *database.WriteClient) (*Service, error) {
	if writeClient == nil {
		return nil, fmt.Errorf("write client is required for analytics service")
	}

	service := &Service{
		writeClient: writeClient,
	}

	// Create analytics tables if they don't exist
	if err := service.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create analytics tables: %w", err)
	}

	return service, nil
}

// createTables creates the analytics tables in the database
func (s *Service) createTables() error {
	queries := []string{
		// Analytics events table
		`CREATE TABLE IF NOT EXISTS analytics_events (
			id SERIAL PRIMARY KEY,
			event_type VARCHAR(50) NOT NULL,
			count INT DEFAULT 1,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_analytics_event_type ON analytics_events(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_created_at ON analytics_events(created_at)`,
		// Daily aggregates table for faster queries
		`CREATE TABLE IF NOT EXISTS analytics_daily (
			id SERIAL PRIMARY KEY,
			date DATE NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			total_count INT DEFAULT 0,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(date, event_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_daily_date ON analytics_daily(date)`,
	}

	for _, query := range queries {
		if _, err := s.writeClient.ExecuteWriteQuery(query); err != nil {
			// Ignore "already exists" errors
			continue
		}
	}

	return nil
}

// TrackEvent records an analytics event
func (s *Service) TrackEvent(eventType string, count int, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var metadataJSON *string
	if metadata != nil {
		jsonBytes, err := json.Marshal(metadata)
		if err == nil {
			str := string(jsonBytes)
			metadataJSON = &str
		}
	}

	// Insert event
	query := `INSERT INTO analytics_events (event_type, count, metadata) VALUES ($1, $2, $3)`
	_, err := s.writeClient.ExecuteWriteQuery(query, eventType, count, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to track event: %w", err)
	}

	// Update daily aggregate
	today := time.Now().UTC().Format("2006-01-02")
	aggregateQuery := `
		INSERT INTO analytics_daily (date, event_type, total_count, metadata)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (date, event_type) DO UPDATE SET
			total_count = analytics_daily.total_count + EXCLUDED.total_count,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = s.writeClient.ExecuteWriteQuery(aggregateQuery, today, eventType, count, metadataJSON)
	if err != nil {
		fmt.Printf("[ANALYTICS] Warning: Failed to update daily aggregate: %v\n", err)
	}

	return nil
}

// TrackConversation records a chat conversation event
func (s *Service) TrackConversation(productCount int, emailCount int, openAITokens int, model string) error {
	metadata := map[string]interface{}{
		"products_suggested": productCount,
		"emails_used":        emailCount,
		"openai_tokens":      openAITokens,
		"model":              model,
	}

	// Track conversation
	if err := s.TrackEvent(EventConversation, 1, metadata); err != nil {
		return err
	}

	// Track product suggestions
	if productCount > 0 {
		if err := s.TrackEvent(EventProductSuggestion, productCount, nil); err != nil {
			return err
		}
	}

	// Track OpenAI call
	if openAITokens > 0 {
		openAIMetadata := map[string]interface{}{
			"tokens": openAITokens,
			"model":  model,
		}
		if err := s.TrackEvent(EventOpenAICall, 1, openAIMetadata); err != nil {
			return err
		}
	}

	return nil
}

// TrackSupportEscalation records a support escalation event
func (s *Service) TrackSupportEscalation(customerEmail string) error {
	metadata := map[string]interface{}{
		"customer_email_hash": hashEmail(customerEmail),
	}
	return s.TrackEvent(EventSupportEscalation, 1, metadata)
}

// TrackSendGridEmail records a SendGrid email sent
func (s *Service) TrackSendGridEmail(emailType string, recipient string) error {
	metadata := map[string]interface{}{
		"type":           emailType,
		"recipient_hash": hashEmail(recipient),
	}
	return s.TrackEvent(EventSendGridCall, 1, metadata)
}

// TrackEmailImport records email import events
func (s *Service) TrackEmailImport(emailCount int, threadCount int) error {
	metadata := map[string]interface{}{
		"emails":  emailCount,
		"threads": threadCount,
	}
	return s.TrackEvent(EventEmailImport, emailCount, metadata)
}

// TrackProductEmbeddings records product embeddings generation
func (s *Service) TrackProductEmbeddings(totalProducts int, changedProducts int, success bool) error {
	metadata := map[string]interface{}{
		"total_products":   totalProducts,
		"changed_products": changedProducts,
		"success":          success,
	}
	return s.TrackEvent(EventProductEmbeddings, changedProducts, metadata)
}

// TrackEmailEmbeddings records email embeddings generation
func (s *Service) TrackEmailEmbeddings(emailCount int, success bool) error {
	metadata := map[string]interface{}{
		"emails":  emailCount,
		"success": success,
	}
	return s.TrackEvent(EventEmailEmbeddings, emailCount, metadata)
}

// TrackThreadEmbeddings records thread embeddings generation
func (s *Service) TrackThreadEmbeddings(threadCount int, success bool) error {
	metadata := map[string]interface{}{
		"threads": threadCount,
		"success": success,
	}
	return s.TrackEvent(EventThreadEmbeddings, threadCount, metadata)
}

// TrackQueryEmbedding records per-search embedding generation (billable)
func (s *Service) TrackQueryEmbedding(queryType string, model string) error {
	metadata := map[string]interface{}{
		"query_type": queryType, // "product_search" or "email_search"
		"model":      model,
	}
	return s.TrackEvent(EventQueryEmbedding, 1, metadata)
}

// TrackSupportSummarization records GPT calls for support summarization (billable)
func (s *Service) TrackSupportSummarization(tokens int, model string) error {
	metadata := map[string]interface{}{
		"tokens": tokens,
		"model":  model,
	}
	return s.TrackEvent(EventSupportSummarization, 1, metadata)
}

// GetSummary retrieves analytics summary for a time period
func (s *Service) GetSummary(period string) (*models.AnalyticsSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now().UTC()
	var startDate, endDate time.Time

	switch period {
	case PeriodToday:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		endDate = now
	case PeriodYesterday:
		yesterday := now.AddDate(0, 0, -1)
		startDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	case PeriodLast7Days:
		startDate = now.AddDate(0, 0, -7)
		endDate = now
	case PeriodLast30Days:
		startDate = now.AddDate(0, 0, -30)
		endDate = now
	default:
		period = PeriodToday
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		endDate = now
	}

	summary := &models.AnalyticsSummary{
		Period:    period,
		StartDate: startDate,
		EndDate:   endDate,
	}

	// Get event counts from daily aggregates
	query := `
		SELECT event_type, COALESCE(SUM(total_count), 0) as total
		FROM analytics_daily
		WHERE date >= $1 AND date <= $2
		GROUP BY event_type
	`

	rows, err := s.writeClient.GetDB().QueryContext(ctx, query, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var eventType string
		var total int
		if err := rows.Scan(&eventType, &total); err != nil {
			continue
		}

		switch eventType {
		case EventConversation:
			summary.TotalConversations = total
		case EventProductSuggestion:
			summary.ProductSuggestions = total
		case EventSupportEscalation:
			summary.SupportEscalations = total
		case EventOpenAICall:
			summary.OpenAICalls = total
		case EventSendGridCall:
			summary.SendGridEmailsSent = total
		case EventEmailImport:
			summary.TotalEmails = total
		case EventProductEmbeddings:
			summary.ProductEmbeddingsRan = true
			summary.ProductEmbeddingsCount = total
		case EventEmailEmbeddings:
			summary.EmailEmbeddingsRan = true
			summary.EmailEmbeddingsCount = total
		case EventThreadEmbeddings:
			summary.ThreadEmbeddingsCount = total
		case EventQueryEmbedding:
			summary.QueryEmbeddings = total
		case EventSupportSummarization:
			summary.SupportSummarizations = total
		}
	}

	// Get OpenAI token usage (from chat completions)
	tokenQuery := `
		SELECT COALESCE(SUM((metadata->>'tokens')::int), 0) as total_tokens
		FROM analytics_events
		WHERE event_type = $1 AND created_at >= $2 AND created_at <= $3
		AND metadata->>'tokens' IS NOT NULL
	`
	var totalTokens int
	err = s.writeClient.GetDB().QueryRowContext(ctx, tokenQuery, EventOpenAICall, startDate, endDate).Scan(&totalTokens)
	if err == nil {
		summary.OpenAITokensUsed = totalTokens
	}

	// Get support summarization token usage
	var supportTokens int
	err = s.writeClient.GetDB().QueryRowContext(ctx, tokenQuery, EventSupportSummarization, startDate, endDate).Scan(&supportTokens)
	if err == nil {
		summary.SupportSummaryTokens = supportTokens
		summary.OpenAITokensUsed += supportTokens // Add to total tokens
	}

	// Get email and thread counts from actual tables
	emailCountQuery := `SELECT COUNT(*) FROM emails WHERE created_at >= $1 AND created_at <= $2`
	err = s.writeClient.GetDB().QueryRowContext(ctx, emailCountQuery, startDate, endDate).Scan(&summary.TotalEmails)
	if err != nil {
		// Try getting total count if date filter fails
		totalEmailQuery := `SELECT COUNT(*) FROM emails`
		_ = s.writeClient.GetDB().QueryRowContext(ctx, totalEmailQuery).Scan(&summary.TotalEmails)
	}

	threadCountQuery := `SELECT COUNT(*) FROM email_threads`
	_ = s.writeClient.GetDB().QueryRowContext(ctx, threadCountQuery).Scan(&summary.EmailThreads)

	// Get total embeddings counts
	productEmbeddingsQuery := `SELECT COUNT(*) FROM product_embeddings`
	_ = s.writeClient.GetDB().QueryRowContext(ctx, productEmbeddingsQuery).Scan(&summary.TotalProductEmbeddings)

	emailEmbeddingsQuery := `SELECT COUNT(*) FROM email_embeddings`
	_ = s.writeClient.GetDB().QueryRowContext(ctx, emailEmbeddingsQuery).Scan(&summary.TotalEmailEmbeddings)

	return summary, nil
}

// GetDailyReport generates a report suitable for Slack notifications
func (s *Service) GetDailyReport() (*models.AnalyticsSummary, error) {
	// Get yesterday's data (complete day)
	return s.GetSummary(PeriodYesterday)
}

// hashEmail creates a simple hash of an email for privacy
func hashEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	return email[:2] + "***" + email[len(email)-3:]
}
