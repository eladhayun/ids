package models

import "time"

// AnalyticsEvent represents a tracked event
type AnalyticsEvent struct {
	ID        int       `db:"id" json:"id"`
	EventType string    `db:"event_type" json:"event_type"` // conversation, product_suggestion, email_import, support_escalation, openai_call, sendgrid_call
	Count     int       `db:"count" json:"count"`
	Metadata  *string   `db:"metadata" json:"metadata,omitempty"` // JSON metadata (tokens used, model, etc.)
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// AnalyticsSummary represents aggregated analytics for a time period
type AnalyticsSummary struct {
	Period               string    `json:"period"`                 // "today", "yesterday", "last_7_days", "last_30_days"
	TotalConversations   int       `json:"total_conversations"`    // Total chat conversations
	ProductSuggestions   int       `json:"product_suggestions"`    // Total product suggestions made
	TotalEmails          int       `json:"total_emails"`           // Total emails in database
	EmailThreads         int       `json:"email_threads"`          // Total email threads
	SupportEscalations   int       `json:"support_escalations"`    // Support requests sent
	OpenAICalls          int       `json:"openai_calls"`           // Total OpenAI API calls
	OpenAITokensUsed     int       `json:"openai_tokens_used"`     // Total tokens consumed
	SendGridEmailsSent   int       `json:"sendgrid_emails_sent"`   // Emails sent via SendGrid
	StartDate            time.Time `json:"start_date"`             // Period start
	EndDate              time.Time `json:"end_date"`               // Period end
	UniqueProductsViewed int       `json:"unique_products_viewed"` // Unique products suggested
	// Embeddings info
	ProductEmbeddingsRan   bool `json:"product_embeddings_ran"`   // Whether product embeddings ran in period
	ProductEmbeddingsCount int  `json:"product_embeddings_count"` // Products processed for embeddings
	EmailEmbeddingsRan     bool `json:"email_embeddings_ran"`     // Whether email embeddings ran in period
	EmailEmbeddingsCount   int  `json:"email_embeddings_count"`   // Emails processed for embeddings
	ThreadEmbeddingsCount  int  `json:"thread_embeddings_count"`  // Threads processed for embeddings
	TotalProductEmbeddings int  `json:"total_product_embeddings"` // Total product embeddings in DB
	TotalEmailEmbeddings   int  `json:"total_email_embeddings"`   // Total email embeddings in DB
	// Additional billing-relevant metrics
	QueryEmbeddings       int `json:"query_embeddings"`       // Per-search embedding generations (billable)
	SupportSummarizations int `json:"support_summarizations"` // GPT calls for support summaries (billable)
	SupportSummaryTokens  int `json:"support_summary_tokens"` // Tokens used for support summarizations
}

// AnalyticsResponse represents the API response for analytics
// @Description Analytics response payload
type AnalyticsResponse struct {
	Success bool              `json:"success" example:"true"`
	Summary *AnalyticsSummary `json:"summary,omitempty"`
	Error   string            `json:"error,omitempty" example:""`
}

// OpenAIUsage represents OpenAI API usage details
type OpenAIUsage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model"`
}
