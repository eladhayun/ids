package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/models"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/sashabaranov/go-openai"
)

// ChatHandler handles chat requests and processes them with OpenAI
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

		// Get product data from database to provide context (with caching)
		productData, err := getProductDataForContext(db, cache, cfg.ProductCacheTTL)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("Failed to fetch product data: %v", err),
			})
		}

		// Create OpenAI client
		client := openai.NewClient(cfg.OpenAIKey)

		// Build conversation messages for OpenAI
		messages := buildOpenAIMessages(req.Conversation, productData)

		// Create chat completion request
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:       openai.GPT4o, // Using GPT-4o as GPT-5 is not available yet
				Messages:    messages,
				MaxTokens:   1000,
				Temperature: 0.7,
			},
		)

		if err != nil {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("OpenAI API error: %v", err),
			})
		}

		if len(resp.Choices) == 0 {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "No response from OpenAI",
			})
		}

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response: resp.Choices[0].Message.Content,
		})
	}
}

// getProductDataForContext fetches product data to provide context to the LLM (with caching)
func getProductDataForContext(db *sqlx.DB, cache *cache.Cache, cacheTTLMinutes int) (string, error) {
	const cacheKey = "product_context_data"

	// Try to get from cache first
	if cachedData, found := cache.Get(cacheKey); found {
		if productData, ok := cachedData.(string); ok {
			return productData, nil
		}
	}

	// Cache miss or invalid data, fetch from database
	query := `
		SELECT
		  p.ID,
		  p.post_title,
		  p.post_content AS description,
		  p.post_excerpt AS short_description,
		  l.sku,
		  l.min_price,
		  l.max_price,
		  l.stock_status,
		  l.stock_quantity,
		  GROUP_CONCAT(DISTINCT t.name ORDER BY t.name SEPARATOR ', ') AS tags
		FROM wpjr_wc_product_meta_lookup l
		JOIN wpjr_posts p ON p.ID = l.product_id
		LEFT JOIN wpjr_term_relationships tr
		  ON tr.object_id = p.ID
		LEFT JOIN wpjr_term_taxonomy tt
		  ON tt.term_taxonomy_id = tr.term_taxonomy_id
		  AND tt.taxonomy = 'product_tag'
		LEFT JOIN wpjr_terms t
		  ON t.term_id = tt.term_id
		WHERE p.post_type = 'product'
		  AND p.post_status IN ('publish','private')
		GROUP BY
		  p.ID, p.post_title, p.post_content, p.post_excerpt,
		  l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
		ORDER BY p.ID
		LIMIT 50
	`

	var products []models.Product
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := db.SelectContext(ctx, &products, query)
	if err != nil {
		return "", err
	}

	// Format product data as context string
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Product Database Context:\n")
	contextBuilder.WriteString("You are a helpful assistant that can answer questions about products in our database. ")
	contextBuilder.WriteString("Here are some sample products to help you understand what's available:\n\n")

	for i, product := range products {
		if i >= 10 { // Limit to first 10 products for context
			break
		}
		contextBuilder.WriteString(fmt.Sprintf("Product ID: %d\n", product.ID))
		contextBuilder.WriteString(fmt.Sprintf("Title: %s\n", product.PostTitle))
		if product.ShortDescription != nil {
			contextBuilder.WriteString(fmt.Sprintf("Description: %s\n", *product.ShortDescription))
		}
		if product.SKU != nil {
			contextBuilder.WriteString(fmt.Sprintf("SKU: %s\n", *product.SKU))
		}
		if product.MinPrice != nil && product.MaxPrice != nil {
			contextBuilder.WriteString(fmt.Sprintf("Price Range: %s - %s\n", *product.MinPrice, *product.MaxPrice))
		}
		if product.StockStatus != nil {
			contextBuilder.WriteString(fmt.Sprintf("Stock Status: %s\n", *product.StockStatus))
		}
		if product.Tags != nil {
			contextBuilder.WriteString(fmt.Sprintf("Tags: %s\n", *product.Tags))
		}
		contextBuilder.WriteString("\n")
	}

	contextBuilder.WriteString("Please provide helpful, accurate answers about our products based on this information. ")
	contextBuilder.WriteString("If you need more specific product details, let the user know that you can help them find specific products.\n")

	productData := contextBuilder.String()

	// Cache the result for the specified TTL
	cache.Set(cacheKey, productData, time.Duration(cacheTTLMinutes)*time.Minute)

	return productData, nil
}

// buildOpenAIMessages converts conversation messages to OpenAI format
func buildOpenAIMessages(conversation []models.ConversationMessage, productContext string) []openai.ChatCompletionMessage {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: productContext,
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
