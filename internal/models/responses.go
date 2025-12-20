package models

import "time"

// HealthResponse represents a basic health check response
// @Description Health check response
type HealthResponse struct {
	Status    string    `json:"status" example:"healthy"`                 // Health status
	Timestamp time.Time `json:"timestamp" example:"2023-01-01T00:00:00Z"` // Timestamp of the check
	Version   string    `json:"version" example:"1.0.0"`                  // Application version
}

// DBHealthResponse represents a database health check response
// @Description Database health check response
type DBHealthResponse struct {
	Status    string        `json:"status" example:"healthy"`                   // Health status
	Timestamp time.Time     `json:"timestamp" example:"2023-01-01T00:00:00Z"`   // Timestamp of the check
	Connected bool          `json:"connected" example:"true"`                   // Database connection status
	Latency   time.Duration `json:"latency" swaggertype:"string" example:"1ms"` // Database ping latency
	Error     string        `json:"error,omitempty" example:""`                 // Error message if any
}

// Product represents a product from the database (minimal version for embeddings)
// @Description Product information for embeddings
type Product struct {
	ID               int      `json:"id" db:"ID" example:"1"`                                        // Product ID
	PostTitle        string   `json:"post_title" db:"post_title" example:"Sample Product"`           // Product title
	PostName         *string  `json:"post_name" db:"post_name" example:"sample-product"`             // Product URL slug
	Description      *string  `json:"description" db:"description" example:"Product description"`    // Product description
	ShortDescription *string  `json:"short_description" db:"short_description" example:"Short desc"` // Short description
	SKU              *string  `json:"sku" db:"sku" example:"SKU123"`                                 // Product SKU
	MinPrice         *string  `json:"min_price" db:"min_price" example:"10.00"`                      // Minimum price
	MaxPrice         *string  `json:"max_price" db:"max_price" example:"20.00"`                      // Maximum price
	StockStatus      *string  `json:"stock_status" db:"stock_status" example:"instock"`              // Stock status
	StockQuantity    *float64 `json:"stock_quantity" db:"stock_quantity" example:"100"`              // Stock quantity
	Tags             *string  `json:"tags" db:"tags" example:"electronics,gadgets"`                  // Product tags
}

// ConversationMessage represents a single message in a conversation
// @Description Single message in a conversation
type ConversationMessage struct {
	Role    string `json:"role" example:"user"`                      // Message role (user, assistant, system)
	Message string `json:"message" example:"Hello, how can I help?"` // Message content
}

// ChatRequest represents the request body for the chat endpoint
// @Description Chat request payload
type ChatRequest struct {
	Conversation []ConversationMessage `json:"conversation"`         // Array of conversation messages
	SessionID    string                `json:"session_id,omitempty"` // Session ID (UUID from frontend)
}

// ChatResponse represents the response from the chat endpoint
// @Description Chat response payload
type ChatResponse struct {
	Response       string            `json:"response" example:"Hello! How can I help you today?"` // AI response message
	Error          string            `json:"error,omitempty" example:""`                          // Error message if any
	Products       map[string]string `json:"products,omitempty"`                                  // Product name to SKU mapping for link generation
	RequestSupport bool              `json:"request_support,omitempty" example:"false"`           // Whether to request customer email for support escalation
}

// SupportRequest represents a request to escalate conversation to support
// @Description Support escalation request payload
type SupportRequest struct {
	Conversation  []ConversationMessage `json:"conversation"`         // Full conversation history
	CustomerEmail string                `json:"customer_email"`       // Customer email address
	SessionID     string                `json:"session_id,omitempty"` // Session ID (UUID from frontend)
}

// SupportResponse represents the response from the support escalation endpoint
// @Description Support escalation response payload
type SupportResponse struct {
	Success bool   `json:"success" example:"true"`                    // Whether the email was sent successfully
	Message string `json:"message" example:"Email sent successfully"` // Response message
	Error   string `json:"error,omitempty" example:""`                // Error message if any
}

// ChatSession represents a chat session
// @Description Chat session metadata
type ChatSession struct {
	ID           int       `json:"id" db:"id" example:"1"`                                                    // Session database ID
	SessionID    string    `json:"session_id" db:"session_id" example:"550e8400-e29b-41d4-a716-446655440000"` // Session UUID
	CreatedAt    time.Time `json:"created_at" db:"created_at" example:"2023-01-01T00:00:00Z"`                 // Session creation time
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at" example:"2023-01-01T00:00:00Z"`                 // Last message time
	EmailSent    bool      `json:"email_sent" db:"email_sent" example:"false"`                                // Whether support email was sent
	MessageCount int       `json:"message_count" db:"message_count" example:"10"`                             // Number of messages in session
}

// SessionMessage represents a single message in a session
// @Description Individual message in a session
type SessionMessage struct {
	ID        int       `json:"id" db:"id" example:"1"`                                                    // Message database ID
	SessionID string    `json:"session_id" db:"session_id" example:"550e8400-e29b-41d4-a716-446655440000"` // Session UUID
	Role      string    `json:"role" db:"role" example:"user"`                                             // Message role (user or assistant)
	Message   string    `json:"message" db:"message" example:"Hello, how can I help?"`                     // Message content
	CreatedAt time.Time `json:"created_at" db:"created_at" example:"2023-01-01T00:00:00Z"`                 // Message timestamp
}

// ChatSessionDetail represents a full session with all messages
// @Description Complete session details including messages
type ChatSessionDetail struct {
	Session   ChatSession      `json:"session"`              // Session metadata
	Messages  []SessionMessage `json:"messages"`             // All messages in the session
	EmailHTML *string          `json:"email_html,omitempty"` // Email HTML if email was sent
}

// SessionListResponse represents a paginated list of sessions
// @Description Paginated session list response
type SessionListResponse struct {
	Sessions []ChatSession `json:"sessions"`                // List of sessions
	Total    int           `json:"total" example:"100"`     // Total number of sessions
	Limit    int           `json:"limit" example:"20"`      // Page size
	Offset   int           `json:"offset" example:"0"`      // Current offset
	HasMore  bool          `json:"has_more" example:"true"` // Whether there are more sessions
}

// AdminAuthRequest represents admin login request
// @Description Admin authentication request
type AdminAuthRequest struct {
	Username string `json:"username" example:"admin"`    // Admin username
	Password string `json:"password" example:"password"` // Admin password
}

// AdminAuthResponse represents admin login response
// @Description Admin authentication response
type AdminAuthResponse struct {
	Success bool   `json:"success" example:"true"`           // Whether authentication succeeded
	Token   string `json:"token,omitempty" example:"abc123"` // Auth token (if successful)
	Error   string `json:"error,omitempty" example:""`       // Error message if any
}
