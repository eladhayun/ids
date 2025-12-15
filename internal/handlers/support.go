package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ids/internal/config"
	"ids/internal/email"
	"ids/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/sashabaranov/go-openai"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const (
	roleUser      = "User"
	roleAssistant = "Assistant"
)

// SupportRequestHandler handles support escalation requests
// @Summary Request support escalation
// @Description Escalate conversation to support team with customer email
// @Tags support
// @Accept json
// @Produce json
// @Param request body models.SupportRequest true "Support request"
// @Success 200 {object} models.SupportResponse
// @Failure 400 {object} models.SupportResponse
// @Failure 500 {object} models.SupportResponse
// @Router /api/chat/request-support [post]
func SupportRequestHandler(cfg *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {
		fmt.Printf("[SUPPORT] ===== NEW SUPPORT REQUEST =====\n")

		// Parse request body
		var req models.SupportRequest
		if err := c.Bind(&req); err != nil {
			fmt.Printf("[SUPPORT] ERROR: Invalid request body: %v\n", err)
			return c.JSON(http.StatusBadRequest, models.SupportResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		// Validate email format
		if !emailRegex.MatchString(req.CustomerEmail) {
			fmt.Printf("[SUPPORT] ERROR: Invalid email format: %s\n", req.CustomerEmail)
			return c.JSON(http.StatusBadRequest, models.SupportResponse{
				Success: false,
				Error:   "Invalid email format. Please provide a valid email address.",
			})
		}

		// Validate conversation is not empty
		if len(req.Conversation) == 0 {
			fmt.Printf("[SUPPORT] ERROR: Empty conversation\n")
			return c.JSON(http.StatusBadRequest, models.SupportResponse{
				Success: false,
				Error:   "Conversation cannot be empty",
			})
		}

		// Check if OpenAI API key is configured
		if cfg.OpenAIKey == "" {
			fmt.Printf("[SUPPORT] ERROR: OpenAI API key not configured\n")
			return c.JSON(http.StatusInternalServerError, models.SupportResponse{
				Success: false,
				Error:   "OpenAI API key not configured",
			})
		}

		// Summarize conversation using OpenAI
		summary, err := summarizeConversation(cfg.OpenAIKey, req.Conversation)
		if err != nil {
			fmt.Printf("[SUPPORT] ERROR: Failed to summarize conversation: %v\n", err)
			// Continue with basic summary if AI summarization fails
			summary = generateBasicSummary(req.Conversation)
		}

		// Format full conversation
		fullConversation := formatConversation(req.Conversation)

		// Send email via email service
		emailService := email.NewEmailService(cfg.SendGridAPIKey, cfg.SupportEmail)
		if err := emailService.SendSupportEscalationEmail(req.CustomerEmail, summary, fullConversation); err != nil {
			fmt.Printf("[SUPPORT] ERROR: Failed to send email: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.SupportResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to send email: %v", err),
			})
		}

		fmt.Printf("[SUPPORT] ✅ Support escalation email sent successfully to %s\n", req.CustomerEmail)
		fmt.Printf("[SUPPORT] ===== REQUEST COMPLETE =====\n\n")

		return c.JSON(http.StatusOK, models.SupportResponse{
			Success: true,
			Message: "Your conversation has been sent to our support team. We'll get back to you soon!",
		})
	}
}

// summarizeConversation uses OpenAI to generate a summary of the conversation
func summarizeConversation(openAIKey string, conversation []models.ConversationMessage) (string, error) {
	client := openai.NewClient(openAIKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build conversation text
	var conversationText strings.Builder
	for _, msg := range conversation {
		role := roleUser
		if strings.Contains(strings.ToLower(msg.Role), "assistant") ||
			strings.Contains(strings.ToLower(msg.Role), "bot") ||
			strings.Contains(strings.ToLower(msg.Role), "ai") {
			role = roleAssistant
		}
		conversationText.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Message))
	}

	// Create summary prompt
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: "You are a support assistant. Summarize the following customer conversation, highlighting:\n" +
				"1. The customer's main question or issue\n" +
				"2. What the assistant tried to help with\n" +
				"3. Why the customer might be dissatisfied\n" +
				"4. Key details that support should know\n\n" +
				"Keep the summary concise (2-3 paragraphs).",
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: conversationText.String(),
		},
	}

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       openai.GPT4oMini,
		Messages:    messages,
		MaxTokens:   500,
		Temperature: 0.7,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// generateBasicSummary creates a simple summary without AI
func generateBasicSummary(conversation []models.ConversationMessage) string {
	var summary strings.Builder
	summary.WriteString("Customer Conversation Summary:\n\n")

	userMessages := 0
	for _, msg := range conversation {
		if strings.Contains(strings.ToLower(msg.Role), "user") {
			userMessages++
			if userMessages <= 3 {
				summary.WriteString(fmt.Sprintf("- %s\n", msg.Message))
			}
		}
	}

	if userMessages > 3 {
		summary.WriteString(fmt.Sprintf("\n... and %d more messages\n", userMessages-3))
	}

	return summary.String()
}

// formatConversation formats the conversation for email
func formatConversation(conversation []models.ConversationMessage) string {
	var formatted strings.Builder
	formatted.WriteString("Full Conversation:\n")
	formatted.WriteString(strings.Repeat("=", 50) + "\n\n")

	for i, msg := range conversation {
		role := roleUser
		if strings.Contains(strings.ToLower(msg.Role), "assistant") ||
			strings.Contains(strings.ToLower(msg.Role), "bot") ||
			strings.Contains(strings.ToLower(msg.Role), "ai") {
			role = roleAssistant
		}

		formatted.WriteString(fmt.Sprintf("[Message %d] %s:\n", i+1, role))
		formatted.WriteString(msg.Message)
		formatted.WriteString("\n\n")
	}

	return formatted.String()
}
