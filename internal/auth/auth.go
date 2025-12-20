package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"ids/internal/config"

	"github.com/labstack/echo/v4"
)

// Manager handles authentication for admin routes
type Manager struct {
	config      *config.Config
	tokens      map[string]time.Time
	mu          sync.RWMutex
	tokenExpiry time.Duration
}

// NewManager creates a new authentication manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:      cfg,
		tokens:      make(map[string]time.Time),
		tokenExpiry: 24 * time.Hour, // Tokens expire after 24 hours
	}
}

// Authenticate validates username and password and returns a token
func (am *Manager) Authenticate(username, password string) (string, error) {
	if username != am.config.AdminUsername || password != am.config.AdminPassword {
		return "", fmt.Errorf("invalid credentials")
	}

	// Generate a secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store token with expiration
	am.mu.Lock()
	am.tokens[token] = time.Now().Add(am.tokenExpiry)
	am.mu.Unlock()

	// Clean up expired tokens in background
	go am.cleanupExpiredTokens()

	return token, nil
}

// ValidateToken checks if a token is valid
func (am *Manager) ValidateToken(token string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	expiry, exists := am.tokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Token expired, remove it
		am.mu.RUnlock()
		am.mu.Lock()
		delete(am.tokens, token)
		am.mu.Unlock()
		am.mu.RLock()
		return false
	}

	return true
}

// cleanupExpiredTokens removes expired tokens
func (am *Manager) cleanupExpiredTokens() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for token, expiry := range am.tokens {
		if now.After(expiry) {
			delete(am.tokens, token)
		}
	}
}

// Middleware creates middleware for admin route authentication
func Middleware(authManager *Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get token from Authorization header or query parameter
			token := c.Request().Header.Get("Authorization")
			if token != "" {
				// Remove "Bearer " prefix if present
				if len(token) > 7 && token[:7] == "Bearer " {
					token = token[7:]
				}
			} else {
				// Try query parameter
				token = c.QueryParam("token")
			}

			if token == "" || !authManager.ValidateToken(token) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Unauthorized. Please login first.",
				})
			}

			// Store token in context for handlers to use
			c.Set("auth_token", token)

			return next(c)
		}
	}
}
