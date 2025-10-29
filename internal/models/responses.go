package models

import "time"

// HealthResponse represents a basic health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// DBHealthResponse represents a database health check response
type DBHealthResponse struct {
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Connected bool          `json:"connected"`
	Latency   time.Duration `json:"latency"`
	Error     string        `json:"error,omitempty"`
}

// Product represents a product from the database
type Product struct {
	ID               int     `json:"id" db:"ID"`
	PostTitle        string  `json:"post_title" db:"post_title"`
	Description      *string `json:"description" db:"description"`
	ShortDescription *string `json:"short_description" db:"short_description"`
	SKU              *string `json:"sku" db:"sku"`
	MinPrice         *string `json:"min_price" db:"min_price"`
	MaxPrice         *string `json:"max_price" db:"max_price"`
	StockStatus      *string `json:"stock_status" db:"stock_status"`
	StockQuantity    *int    `json:"stock_quantity" db:"stock_quantity"`
	Tags             *string `json:"tags" db:"tags"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalCount int  `json:"total_count"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ProductsResponse represents the paginated products response
type ProductsResponse struct {
	Data       []Product      `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

// ChatRequest represents the request body for the chat endpoint
type ChatRequest struct {
	Conversation []ConversationMessage `json:"conversation"`
}

// ChatResponse represents the response from the chat endpoint
type ChatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}
