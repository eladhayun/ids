package database

import (
	"fmt"

	"ids/internal/models"
)

// ConversationService handles conversation session storage
type ConversationService struct {
	writeClient *WriteClient
}

// NewConversationService creates a new conversation service
func NewConversationService(writeClient *WriteClient) (*ConversationService, error) {
	if writeClient == nil {
		return nil, fmt.Errorf("write client is required for conversation service")
	}

	service := &ConversationService{
		writeClient: writeClient,
	}

	// Create tables if they don't exist
	if err := service.CreateTables(); err != nil {
		return nil, fmt.Errorf("failed to create conversation tables: %w", err)
	}

	return service, nil
}

// CreateTables creates the conversation tables in the database
func (s *ConversationService) CreateTables() error {
	queries := []string{
		// Chat sessions table
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(36) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			email_sent BOOLEAN DEFAULT FALSE,
			email_html TEXT
		)`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_session_id ON chat_sessions(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_created_at ON chat_sessions(created_at DESC)`,
		// Session messages table
		`CREATE TABLE IF NOT EXISTS session_messages (
			id SERIAL PRIMARY KEY,
			session_id VARCHAR(36) NOT NULL,
			role VARCHAR(20) NOT NULL,
			message TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES chat_sessions(session_id) ON DELETE CASCADE
		)`,
		// Create indexes
		`CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_session_messages_created_at ON session_messages(created_at)`,
	}

	for _, query := range queries {
		if _, err := s.writeClient.ExecuteWriteQuery(query); err != nil {
			// Ignore "already exists" errors
			continue
		}
	}

	return nil
}

// SaveSession creates or updates a session
func (s *ConversationService) SaveSession(sessionID string) error {
	query := `
		INSERT INTO chat_sessions (session_id, created_at, updated_at)
		VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id) DO UPDATE SET
			updated_at = CURRENT_TIMESTAMP
	`
	_, err := s.writeClient.ExecuteWriteQuery(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

// SaveMessage saves a message to a session
func (s *ConversationService) SaveMessage(sessionID string, role, message string) error {
	// Ensure session exists first
	if err := s.SaveSession(sessionID); err != nil {
		return err
	}

	query := `
		INSERT INTO session_messages (session_id, role, message, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
	`
	_, err := s.writeClient.ExecuteWriteQuery(query, sessionID, role, message)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

// UpdateSessionEmail updates a session with email information
func (s *ConversationService) UpdateSessionEmail(sessionID string, emailHTML string) error {
	query := `
		UPDATE chat_sessions
		SET email_sent = TRUE, email_html = $1, updated_at = CURRENT_TIMESTAMP
		WHERE session_id = $2
	`
	_, err := s.writeClient.ExecuteWriteQuery(query, emailHTML, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session email: %w", err)
	}
	return nil
}

// GetSessions retrieves a paginated list of sessions
func (s *ConversationService) GetSessions(limit, offset int) ([]models.ChatSession, error) {
	query := `
		SELECT 
			cs.id,
			cs.session_id,
			cs.created_at,
			cs.updated_at,
			cs.email_sent,
			COUNT(sm.id) as message_count
		FROM chat_sessions cs
		LEFT JOIN session_messages sm ON cs.session_id = sm.session_id
		GROUP BY cs.id, cs.session_id, cs.created_at, cs.updated_at, cs.email_sent
		ORDER BY cs.created_at DESC
		LIMIT $1 OFFSET $2
	`

	var sessions []models.ChatSession
	err := s.writeClient.ExecuteWriteQueryWithResult(&sessions, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Ensure we return an empty slice, not nil
	if sessions == nil {
		sessions = []models.ChatSession{}
	}

	return sessions, nil
}

// GetSessionCount returns the total number of sessions
func (s *ConversationService) GetSessionCount() (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM chat_sessions`
	err := s.writeClient.ExecuteWriteQuerySingle(&count, query)
	if err != nil {
		return 0, fmt.Errorf("failed to get session count: %w", err)
	}
	return count, nil
}

// GetSessionDetails retrieves a session with all its messages
func (s *ConversationService) GetSessionDetails(sessionID string) (*models.ChatSessionDetail, error) {
	// Get session metadata
	var session models.ChatSession
	query := `
		SELECT 
			id,
			session_id,
			created_at,
			updated_at,
			email_sent,
			(SELECT COUNT(*) FROM session_messages WHERE session_id = $1) as message_count
		FROM chat_sessions
		WHERE session_id = $1
	`
	err := s.writeClient.ExecuteWriteQuerySingle(&session, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get messages
	var messages []models.SessionMessage
	msgQuery := `
		SELECT id, session_id, role, message, created_at
		FROM session_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	err = s.writeClient.ExecuteWriteQueryWithResult(&messages, msgQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Get email HTML if available
	var emailHTML *string
	if session.EmailSent {
		emailQuery := `SELECT email_html FROM chat_sessions WHERE session_id = $1`
		err = s.writeClient.ExecuteWriteQuerySingle(&emailHTML, emailQuery, sessionID)
		if err != nil {
			// Log but don't fail - email HTML is optional
			fmt.Printf("[CONVERSATIONS] Warning: Failed to get email HTML: %v\n", err)
		}
	}

	return &models.ChatSessionDetail{
		Session:   session,
		Messages:  messages,
		EmailHTML: emailHTML,
	}, nil
}

// GetSessionEmailHTML retrieves the email HTML for a session
func (s *ConversationService) GetSessionEmailHTML(sessionID string) (*string, error) {
	var emailHTML *string
	query := `SELECT email_html FROM chat_sessions WHERE session_id = $1 AND email_sent = TRUE`
	err := s.writeClient.ExecuteWriteQuerySingle(&emailHTML, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email HTML: %w", err)
	}
	return emailHTML, nil
}
