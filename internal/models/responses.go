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
	Conversation []ConversationMessage `json:"conversation"` // Array of conversation messages
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
	Conversation  []ConversationMessage `json:"conversation"`   // Full conversation history
	CustomerEmail string                `json:"customer_email"` // Customer email address
}

// SupportResponse represents the response from the support escalation endpoint
// @Description Support escalation response payload
type SupportResponse struct {
	Success bool   `json:"success" example:"true"`                    // Whether the email was sent successfully
	Message string `json:"message" example:"Email sent successfully"` // Response message
	Error   string `json:"error,omitempty" example:""`                // Error message if any
}
