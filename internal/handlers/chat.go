package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/models"
	"ids/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/sashabaranov/go-openai"
)

// ChatHandler handles chat requests and processes them with OpenAI
// @Summary Chat with AI
// @Description Send a conversation to the AI chatbot and get a response
// @Tags chat
// @Accept json
// @Produce json
// @Param request body models.ChatRequest true "Chat request"
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} models.ChatResponse
// @Failure 500 {object} models.ChatResponse
// @Failure 503 {object} models.ChatResponse
// @Router /api/chat [post]
func ChatHandler(db *sqlx.DB, cfg *config.Config, cache *cache.Cache) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Handle case where database connection is not available
		if db == nil {
			return c.JSON(http.StatusServiceUnavailable, models.ChatResponse{
				Error: "Database connection not available",
			})
		}

		// Check if OpenAI API key is configured
		if cfg.OpenAIKey == "" {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "OpenAI API key not configured",
			})
		}

		// Parse request body
		var req models.ChatRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		// Validate conversation is not empty
		if len(req.Conversation) == 0 {
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: "Conversation cannot be empty",
			})
		}

		// Truncate conversation to prevent token limit issues
		// Keep only the last user message and truncate its content
		if len(req.Conversation) > 1 {
			// Find the last user message
			for i := len(req.Conversation) - 1; i >= 0; i-- {
				if strings.Contains(strings.ToLower(req.Conversation[i].Role), "user") {
					// Truncate the message content to 100 characters max
					message := req.Conversation[i].Message
					if len(message) > 100 {
						message = message[:100] + "..."
					}
					req.Conversation = []models.ConversationMessage{
						{
							Role:    req.Conversation[i].Role,
							Message: message,
						},
					}
					break
				}
			}
		}

		// Create OpenAI client
		client := openai.NewClient(cfg.OpenAIKey)

		// Build conversation messages for OpenAI
		messages := buildOpenAIMessages(req.Conversation, utils.Language{Code: utils.LangEnglish, Name: "English", Confidence: 1.0})

		// Create chat completion request with retry logic
		var resp openai.ChatCompletionResponse
		var apiErr error

		maxRetries := 3
		baseDelay := 1 * time.Second

		for attempt := 0; attempt < maxRetries; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenAITimeout)*time.Second)

			resp, apiErr = client.CreateChatCompletion(
				ctx,
				openai.ChatCompletionRequest{
					Model:       openai.GPT4oMini,
					Messages:    messages,
					MaxTokens:   2000,
					Temperature: 0.7,
				},
			)

			cancel()

			// If successful, break out of retry loop
			if apiErr == nil {
				break
			}

			// If this is the last attempt, return the error
			if attempt == maxRetries-1 {
				return c.JSON(http.StatusInternalServerError, models.ChatResponse{
					Error: fmt.Sprintf("OpenAI API error after %d attempts: %v", maxRetries, apiErr),
				})
			}

			// Calculate exponential backoff delay
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			time.Sleep(delay)
		}

		if apiErr != nil {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("OpenAI API error: %v", apiErr),
			})
		}

		if len(resp.Choices) == 0 {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "No response from OpenAI",
			})
		}

		// Create response
		response := resp.Choices[0].Message.Content

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response: response,
		})
	}
}

// getDatabaseMetadata returns the database metadata for AI context
func getDatabaseMetadata() string {
	return `STORE INFO: Israel Defense Store - tactical gear & military equipment
MAIN CATEGORIES: Gun Holsters, Fobus, DPM, FAB Defense, ORPAZ Defense, Conversion Kits
KEY TAGS: Glock, Right/Left Hand, Black/Od Green, OWB/IWB, Polymer/Leather, Duty Holster
PRICE RANGE: $1.25-$2,999.99 | 90% in stock
RULES: Only recommend in-stock products from provided data. Use tags for compatibility.`
}

// buildOpenAIMessages converts conversation messages to OpenAI format
func buildOpenAIMessages(conversation []models.ConversationMessage, detectedLang utils.Language) []openai.ChatCompletionMessage {
	// Create the main system prompt for tactical gear sales
	systemPrompt := `You are a sales rep for Israel Defense Store (israeldefensestore.com) specializing in tactical gear.

ROLE: Help customers find tactical gear products. Be professional and knowledgeable.

RULES:
- Provide general information about tactical gear and military equipment
- Be helpful with product recommendations and compatibility questions
- Format responses with **bold** for product names
- Use bullet points for lists
- Be informative about different types of tactical gear

RESPONSE FORMAT: **[Product Name]** - [Description]`

	// Add language instruction to the system prompt
	languageInstruction := utils.GetLanguageInstruction(detectedLang)

	// Get database metadata for AI context
	databaseMetadata := getDatabaseMetadata()

	// Combine system prompt, database metadata, and language instruction
	enhancedContext := systemPrompt + "\n\n" + databaseMetadata + "\n\n" + languageInstruction

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: enhancedContext,
		},
	}

	// Add conversation messages
	for _, msg := range conversation {
		// Determine role based on the role field
		role := openai.ChatMessageRoleUser
		if strings.Contains(strings.ToLower(msg.Role), "assistant") ||
			strings.Contains(strings.ToLower(msg.Role), "bot") ||
			strings.Contains(strings.ToLower(msg.Role), "ai") {
			role = openai.ChatMessageRoleAssistant
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Message,
		})
	}

	return messages
}
