package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ids/internal/auth"
	"ids/internal/database"
	"ids/internal/models"

	"github.com/labstack/echo/v4"
)

// Israel timezone
var israelTZ *time.Location

func init() {
	var err error
	israelTZ, err = time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		// Fallback to UTC if timezone loading fails
		israelTZ = time.UTC
	}
}

// AdminLoginHandler handles admin authentication
// @Summary Admin login
// @Description Authenticate admin user and receive auth token
// @Tags admin
// @Accept json
// @Produce json
// @Param request body models.AdminAuthRequest true "Login credentials"
// @Success 200 {object} models.AdminAuthResponse
// @Failure 401 {object} models.AdminAuthResponse
// @Router /api/admin/login [post]
func AdminLoginHandler(authManager *auth.Manager) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req models.AdminAuthRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, models.AdminAuthResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		token, err := authManager.Authenticate(req.Username, req.Password)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, models.AdminAuthResponse{
				Success: false,
				Error:   "Invalid username or password",
			})
		}

		return c.JSON(http.StatusOK, models.AdminAuthResponse{
			Success: true,
			Token:   token,
		})
	}
}

// ListSessionsHandler handles listing chat sessions
// @Summary List chat sessions
// @Description Get a paginated list of chat sessions
// @Tags admin
// @Accept json
// @Produce json
// @Param limit query int false "Number of sessions per page" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} models.SessionListResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/admin/sessions [get]
func ListSessionsHandler(conversationService *database.ConversationService) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get pagination parameters
		limit := 20 // default
		offset := 0 // default

		if limitStr := c.QueryParam("limit"); limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		if offsetStr := c.QueryParam("offset"); offsetStr != "" {
			if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		// Get sessions
		sessions, err := conversationService.GetSessions(limit, offset)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to get sessions: %v", err),
			})
		}

		// Ensure sessions is never nil
		if sessions == nil {
			sessions = []models.ChatSession{}
		}

		// Get total count
		total, err := conversationService.GetSessionCount()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to get session count: %v", err),
			})
		}

		// Convert timestamps to Israel timezone
		for i := range sessions {
			sessions[i].CreatedAt = sessions[i].CreatedAt.In(israelTZ)
			sessions[i].UpdatedAt = sessions[i].UpdatedAt.In(israelTZ)
		}

		hasMore := offset+limit < total

		return c.JSON(http.StatusOK, models.SessionListResponse{
			Sessions: sessions,
			Total:    total,
			Limit:    limit,
			Offset:   offset,
			HasMore:  hasMore,
		})
	}
}

// GetSessionHandler handles getting a single session with all messages
// @Summary Get session details
// @Description Get full details of a chat session including all messages
// @Tags admin
// @Accept json
// @Produce json
// @Param sessionId path string true "Session ID (UUID)"
// @Success 200 {object} models.ChatSessionDetail
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/admin/sessions/{sessionId} [get]
func GetSessionHandler(conversationService *database.ConversationService) echo.HandlerFunc {
	return func(c echo.Context) error {
		sessionID := c.Param("sessionId")
		if sessionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Session ID is required",
			})
		}

		// Get session details
		sessionDetail, err := conversationService.GetSessionDetails(sessionID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("Session not found: %v", err),
			})
		}

		// Convert timestamps to Israel timezone
		sessionDetail.Session.CreatedAt = sessionDetail.Session.CreatedAt.In(israelTZ)
		sessionDetail.Session.UpdatedAt = sessionDetail.Session.UpdatedAt.In(israelTZ)
		for i := range sessionDetail.Messages {
			sessionDetail.Messages[i].CreatedAt = sessionDetail.Messages[i].CreatedAt.In(israelTZ)
		}

		return c.JSON(http.StatusOK, sessionDetail)
	}
}

// GetSessionEmailHandler handles getting email HTML for a session
// @Summary Get session email HTML
// @Description Get the email HTML content for a session (if email was sent)
// @Tags admin
// @Accept json
// @Produce html
// @Param sessionId path string true "Session ID (UUID)"
// @Success 200 {string} string "HTML content"
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/admin/sessions/{sessionId}/email [get]
func GetSessionEmailHandler(conversationService *database.ConversationService) echo.HandlerFunc {
	return func(c echo.Context) error {
		sessionID := c.Param("sessionId")
		if sessionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Session ID is required",
			})
		}

		// Get email HTML
		emailHTML, err := conversationService.GetSessionEmailHTML(sessionID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("Email not found for session: %v", err),
			})
		}

		if emailHTML == nil {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "No email found for this session",
			})
		}

		// Return HTML content
		c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
		return c.String(http.StatusOK, *emailHTML)
	}
}
