package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/emails"
	"ids/internal/embeddings"
	"ids/internal/models"
	"ids/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/sashabaranov/go-openai"
)

const stockStatusInStock = "instock"

// ChatHandler handles chat requests with both product and email context
// @Summary Chat with AI using enhanced vector search (products + email history)
// @Description Send a conversation to the AI chatbot and get a response with product recommendations enhanced by similar past conversations
// @Tags chat
// @Accept json
// @Produce json
// @Param request body models.ChatRequest true "Chat request"
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} models.ChatResponse
// @Failure 500 {object} models.ChatResponse
// @Failure 503 {object} models.ChatResponse
// @Router /api/chat [post]
func ChatHandler(db *sqlx.DB, cfg *config.Config, cache *cache.Cache, embeddingService *embeddings.EmbeddingService, writeClient *database.WriteClient) echo.HandlerFunc {
	// Create email embedding service
	emailService, err := emails.NewEmailEmbeddingService(cfg, writeClient)
	if err != nil {
		fmt.Printf("[CHAT] Warning: Failed to create email service: %v\n", err)
		emailService = nil // Will skip email search if not available
	}

	return func(c echo.Context) error {
		fmt.Printf("[CHAT] ===== NEW CHAT REQUEST =====\n")

		// Handle case where database connection is not available
		if db == nil {
			fmt.Printf("[CHAT] ERROR: Database connection not available\n")
			return c.JSON(http.StatusServiceUnavailable, models.ChatResponse{
				Error: "Database connection not available",
			})
		}

		// Check if OpenAI API key is configured
		if cfg.OpenAIKey == "" {
			fmt.Printf("[CHAT] ERROR: OpenAI API key not configured\n")
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "OpenAI API key not configured",
			})
		}

		// Parse request body
		var req models.ChatRequest
		if err := c.Bind(&req); err != nil {
			fmt.Printf("[CHAT] ERROR: Invalid request body: %v\n", err)
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		fmt.Printf("[CHAT] Received conversation with %d messages\n", len(req.Conversation))

		// Validate conversation is not empty
		if len(req.Conversation) == 0 {
			fmt.Printf("[CHAT] ERROR: Empty conversation\n")
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: "Conversation cannot be empty",
			})
		}

		// Get the last user message
		var userQuery string
		for i := len(req.Conversation) - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(req.Conversation[i].Role), "user") {
				userQuery = req.Conversation[i].Message
				break
			}
		}

		if userQuery == "" {
			fmt.Printf("[CHAT] ERROR: No user message found in conversation\n")
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: "No user message found in conversation",
			})
		}

		fmt.Printf("[CHAT] Extracted user query: '%s'\n", userQuery)

		// Check for shipping inquiry
		if isShipping, country := IsShippingInquiry(userQuery); isShipping {
			fmt.Printf("[CHAT] Detected shipping inquiry for country: %s\n", country)
			response := GetShippingResponse(country)
			return c.JSON(http.StatusOK, models.ChatResponse{
				Response: response,
				Products: make(map[string]string),
			})
		}

		// Search for similar products
		fmt.Printf("[CHAT] 🔍 DATASOURCE: Starting PRODUCT EMBEDDINGS search for query: '%s'\n", userQuery)
		searchStart := time.Now()
		similarProducts, fallbackToSimilarity, err := embeddingService.SearchSimilarProducts(userQuery, 20)
		searchDuration := time.Since(searchStart)
		if err != nil {
			fmt.Printf("[CHAT] ❌ ERROR: Product embeddings search failed: %v (took %v)\n", err, searchDuration)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("Failed to search products: %v", err),
			})
		}

		fmt.Printf("[CHAT] ✅ DATASOURCE: PRODUCT EMBEDDINGS search completed - Found %d products (took %v, fallback=%t)\n", len(similarProducts), searchDuration, fallbackToSimilarity)

		// Filter to in-stock products
		var inStockProducts []embeddings.ProductEmbedding
		for _, product := range similarProducts {
			if product.Product.StockStatus != nil && *product.Product.StockStatus == stockStatusInStock {
				inStockProducts = append(inStockProducts, product)
			}
		}

		if len(inStockProducts) == 0 {
			inStockProducts = similarProducts
		}

		fmt.Printf("[CHAT] %d in-stock products\n", len(inStockProducts))

		// Search for similar email conversations (if enabled)
		var similarEmails []models.EmailSearchResult
		if cfg.EnableEmailContext && emailService != nil {
			fmt.Printf("[CHAT] 🔍 DATASOURCE: Starting EMAIL EMBEDDINGS search for query: '%s'\n", userQuery)
			emailSearchStart := time.Now()
			similarEmails, err = emailService.SearchSimilarEmails(userQuery, 5, true) // Search threads
			emailSearchDuration := time.Since(emailSearchStart)
			if err != nil {
				fmt.Printf("[CHAT] ❌ ERROR: Email embeddings search failed: %v (took %v)\n", err, emailSearchDuration)
			} else {
				fmt.Printf("[CHAT] ✅ DATASOURCE: EMAIL EMBEDDINGS search completed - Found %d similar email threads (took %v)\n", len(similarEmails), emailSearchDuration)
			}
		} else if !cfg.EnableEmailContext {
			fmt.Printf("[CHAT] ⚠️  DATASOURCE: EMAIL EMBEDDINGS search skipped - Email context disabled in config\n")
		} else if emailService == nil {
			fmt.Printf("[CHAT] ⚠️  DATASOURCE: EMAIL EMBEDDINGS search skipped - Email service not available\n")
		}

		// Create product metadata for frontend
		productMetadata := make(map[string]string)
		for _, product := range inStockProducts {
			if product.Product.PostName != nil && *product.Product.PostName != "" {
				productMetadata[product.Product.PostTitle] = *product.Product.PostName
			} else if product.Product.SKU != nil && *product.Product.SKU != "" {
				productMetadata[product.Product.PostTitle] = *product.Product.SKU
			} else {
				productMetadata[product.Product.PostTitle] = fmt.Sprintf("product-%d", product.Product.ID)
			}
		}

		// Build OpenAI messages with enhanced context
		messages := buildOpenAIMessages(
			req.Conversation,
			inStockProducts,
			similarEmails,
			utils.Language{Code: utils.LangEnglish, Name: "English", Confidence: 1.0},
			fallbackToSimilarity,
		)

		// Create OpenAI client and get response
		client := openai.NewClient(cfg.OpenAIKey)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenAITimeout)*time.Second)
		defer cancel()

		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:       openai.GPT4oMini,
			Messages:    messages,
			MaxTokens:   1500, // Increased for richer context
			Temperature: 0.7,
		})

		if err != nil {
			fmt.Printf("[CHAT] ERROR: OpenAI API error: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("OpenAI API error: %v", err),
			})
		}

		if len(resp.Choices) == 0 {
			fmt.Printf("[CHAT] ERROR: No response from OpenAI\n")
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "No response from OpenAI",
			})
		}

		response := resp.Choices[0].Message.Content
		if len(inStockProducts) > 0 {
			response += fmt.Sprintf("\n\n**Found %d relevant products**", len(inStockProducts))
		}

		fmt.Printf("[CHAT] 📊 DATASOURCE SUMMARY: Used %d product embeddings, %d email embeddings\n", len(inStockProducts), len(similarEmails))
		fmt.Printf("[CHAT] ===== REQUEST COMPLETE =====\n\n")

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response: response,
			Products: productMetadata,
		})
	}
}

// buildOpenAIMessages creates OpenAI messages with product and email context
func buildOpenAIMessages(
	conversation []models.ConversationMessage,
	products []embeddings.ProductEmbedding,
	emailThreads []models.EmailSearchResult,
	detectedLang utils.Language,
	fallbackToSimilarity bool,
) []openai.ChatCompletionMessage {

	systemPrompt := `You are an expert sales rep for Israel Defense Store (israeldefensestore.com) specializing in tactical gear.

ROLE: Help customers find tactical gear products.`

	if len(emailThreads) > 0 {
		systemPrompt += ` You have access to:
1. A list of relevant products found using advanced vector search
2. Similar past customer conversations that might help you answer better`
	} else {
		systemPrompt += ` You have access to:
1. A list of relevant products found using advanced vector search`
	}

	systemPrompt += `

DETECT USER INTENT:
When user asks questions like:
- "what [product] do you have" or "what [product] for [model] do you have"
- "do you have [product]" or "do you have any [product]"
- "show me [product]" or "show [product]"
- "what [product] are in stock" or "what [product] do you have in stock"
- "list [product]" or "available [product]"

These are PRODUCT LISTING REQUESTS - show 5-10 available products.

COMPATIBILITY CHECK:
- For product listing requests, show available products with compatibility info from tags
- If product tags explicitly list the user's model → "Compatible with [Model]"
- If product tags list similar models but not exact match → "⚠️ May be compatible - verify with tags: [tags]"
- If product tags don't mention compatibility → "⚠️ Compatibility uncertain - please verify"

STRICT MODE (Only for "Is X compatible with Y?" questions):
- If user specifically asks "Is X compatible with Y?" without asking to see products
- AND product tags don't explicitly mention their exact model variant
- THEN state: "I couldn't find an exact match for [Model] in our current inventory."
- But still mention: "I found [N] related products if you'd like to see them."

RULES:
- Only recommend products from the provided list
- Use product tags for compatibility verification
- Check stock status before recommending
- Provide pricing and availability details
- Format responses with **bold** for product names
- Show the most relevant products first (ranked by similarity)
- When a customer asks for a link, use: https://israeldefensestore.com/product/<slug>

APPAREL & SIZING:
- For clothing, provide sizing from product tags (XS, S, M, L, XL, XXL)
- Mention special features (water-resistant, insulated, etc.)`

	if len(emailThreads) > 0 {
		systemPrompt += `

PAST CONVERSATIONS:
- Use insights from similar past conversations to enhance your response
- Learn from how issues were resolved previously
- Don't mention that you're using past conversations - just naturally incorporate the knowledge`
	}

	systemPrompt += `

RESPONSE FORMAT:
- For confirmed compatibility: **[Product Name]** - [Price] - [Stock] - Compatible with [Model]
- For uncertain compatibility: **[Product Name]** - [Price] - [Stock] - ⚠️ Compatibility uncertain - please verify`

	if fallbackToSimilarity {
		systemPrompt += `

IMPORTANT:
- Exact matches were not found for this query
- Present listed products as closest available alternatives
- Clearly explain any differences when relevant`
	}

	// Add language instruction
	languageInstruction := utils.GetLanguageInstruction(detectedLang)

	// Build product context
	var productContext strings.Builder
	productContext.WriteString("\n\n=== RELEVANT PRODUCTS ===\n")
	for i, product := range products {
		if i >= 15 {
			productContext.WriteString(fmt.Sprintf("\n... and %d more products available", len(products)-15))
			break
		}

		productContext.WriteString(fmt.Sprintf("\n**%s**", product.Product.PostTitle))

		if product.Product.MinPrice != nil && product.Product.MaxPrice != nil {
			if *product.Product.MinPrice == *product.Product.MaxPrice {
				productContext.WriteString(fmt.Sprintf(" - $%s", *product.Product.MinPrice))
			} else {
				productContext.WriteString(fmt.Sprintf(" - $%s-$%s", *product.Product.MinPrice, *product.Product.MaxPrice))
			}
		}

		if product.Product.StockStatus != nil {
			if *product.Product.StockStatus == stockStatusInStock {
				productContext.WriteString(" - In Stock")
			} else {
				productContext.WriteString(" - Out of Stock")
			}
		}

		productContext.WriteString(fmt.Sprintf(" - Similarity: %.2f", product.Similarity))

		if product.Product.Tags != nil && *product.Product.Tags != "" {
			productContext.WriteString(fmt.Sprintf(" - Tags: %s", *product.Product.Tags))
		}

		if product.Product.PostName != nil && *product.Product.PostName != "" {
			productContext.WriteString(fmt.Sprintf(" - URL: https://israeldefensestore.com/product/%s", *product.Product.PostName))
		} else {
			productContext.WriteString(fmt.Sprintf(" - URL: https://israeldefensestore.com/?p=%d", product.Product.ID))
		}
	}

	// Build email context if available
	var emailContext strings.Builder
	if len(emailThreads) > 0 {
		emailContext.WriteString("\n\n=== SIMILAR PAST CONVERSATIONS (for context) ===\n")
		emailContext.WriteString("Learn from these similar customer interactions:\n")

		for i, result := range emailThreads {
			if i >= 3 { // Limit to top 3 threads
				break
			}

			if result.Thread != nil {
				emailContext.WriteString(fmt.Sprintf("\n--- Thread: %s (Similarity: %.2f) ---\n", result.Thread.Subject, result.Similarity))

				// Fetch thread emails
				threadEmails, err := getThreadEmails(result.Thread.ThreadID)
				if err == nil && len(threadEmails) > 0 {
					for j, email := range threadEmails {
						if j >= 5 { // Limit to 5 emails per thread
							break
						}

						role := "Customer"
						if !email.IsCustomer {
							role = "Support"
						}

						body := strings.TrimSpace(email.Body)
						if len(body) > 300 {
							body = body[:300] + "..."
						}

						emailContext.WriteString(fmt.Sprintf("%s: %s\n", role, body))
					}
				}
			}
		}

		emailContext.WriteString("\n(Use these conversations to understand common questions and effective responses)")
	}

	// Combine all context
	enhancedContext := systemPrompt + productContext.String() + emailContext.String() + "\n\n" + languageInstruction

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: enhancedContext,
		},
	}

	// Add conversation messages
	for _, msg := range conversation {
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

// getThreadEmails retrieves all emails in a thread (helper function)
func getThreadEmails(threadID string) ([]models.Email, error) {
	// This is a simplified version - in production, you'd inject the DB connection
	// For now, we'll return an error to use the summary instead
	return nil, fmt.Errorf("thread detail retrieval not available in this context")
}
