package models

import "time"

// Email represents an email message
type Email struct {
	ID         int       `db:"id" json:"id"`
	MessageID  string    `db:"message_id" json:"message_id"`
	Subject    string    `db:"subject" json:"subject"`
	From       string    `db:"from_addr" json:"from"`
	To         string    `db:"to_addr" json:"to"`
	Date       time.Time `db:"date" json:"date"`
	Body       string    `db:"body" json:"body"`
	ThreadID   *string   `db:"thread_id" json:"thread_id,omitempty"`
	InReplyTo  *string   `db:"in_reply_to" json:"in_reply_to,omitempty"`
	References *string   `db:"references" json:"references,omitempty"`
	IsCustomer bool      `db:"is_customer" json:"is_customer"` // true if from customer, false if from support
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// EmailThread represents a conversation thread
type EmailThread struct {
	ThreadID   string    `db:"thread_id" json:"thread_id"`
	Subject    string    `db:"subject" json:"subject"`
	EmailCount int       `db:"email_count" json:"email_count"`
	FirstDate  time.Time `db:"first_date" json:"first_date"`
	LastDate   time.Time `db:"last_date" json:"last_date"`
	Summary    string    `db:"summary" json:"summary"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// EmailEmbedding represents an email or thread with its vector embedding
type EmailEmbedding struct {
	ID        int       `db:"id" json:"id"`
	EmailID   *int      `db:"email_id" json:"email_id,omitempty"`
	ThreadID  *string   `db:"thread_id" json:"thread_id,omitempty"`
	Embedding string    `db:"embedding" json:"embedding"` // JSON array of floats
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// EmailSearchResult represents an email with similarity score
type EmailSearchResult struct {
	Email      Email        `json:"email"`
	Thread     *EmailThread `json:"thread,omitempty"`
	Similarity float64      `json:"similarity"`
	Embedding  []float64    `json:"-"`
}
