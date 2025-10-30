package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/embeddings"
	"ids/internal/models"
	"ids/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/sashabaranov/go-openai"
)

// ChatVectorHandler handles chat requests using vector embeddings for product search
// @Summary Chat with AI using vector search
// @Description Send a conversation to the AI chatbot and get a response with vector-based product recommendations
// @Tags chat
// @Accept json
// @Produce json
// @Param request body models.ChatRequest true "Chat request"
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} models.ChatResponse
// @Failure 500 {object} models.ChatResponse
// @Failure 503 {object} models.ChatResponse
// @Router /api/chat [post]
func ChatVectorHandler(db *sqlx.DB, cfg *config.Config, cache *cache.Cache, embeddingService *embeddings.EmbeddingService) echo.HandlerFunc {
	return func(c echo.Context) error {
		fmt.Printf("[CHAT_VECTOR] ===== NEW CHAT REQUEST =====\n")
		
		// Handle case where database connection is not available
		if db == nil {
			fmt.Printf("[CHAT_VECTOR] ERROR: Database connection not available\n")
			return c.JSON(http.StatusServiceUnavailable, models.ChatResponse{
				Error: "Database connection not available",
			})
		}

		// Check if OpenAI API key is configured
		if cfg.OpenAIKey == "" {
			fmt.Printf("[CHAT_VECTOR] ERROR: OpenAI API key not configured\n")
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "OpenAI API key not configured",
			})
		}

		// Parse request body
		var req models.ChatRequest
		if err := c.Bind(&req); err != nil {
			fmt.Printf("[CHAT_VECTOR] ERROR: Invalid request body: %v\n", err)
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: fmt.Sprintf("Invalid request body: %v", err),
			})
		}

		fmt.Printf("[CHAT_VECTOR] Received conversation with %d messages\n", len(req.Conversation))

		// Validate conversation is not empty
		if len(req.Conversation) == 0 {
			fmt.Printf("[CHAT_VECTOR] ERROR: Empty conversation\n")
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: "Conversation cannot be empty",
			})
		}

		// Get the last user message for product search
		var userQuery string
		for i := len(req.Conversation) - 1; i >= 0; i-- {
			if strings.Contains(strings.ToLower(req.Conversation[i].Role), "user") {
				userQuery = req.Conversation[i].Message
				break
			}
		}

		if userQuery == "" {
			fmt.Printf("[CHAT_VECTOR] ERROR: No user message found in conversation\n")
			return c.JSON(http.StatusBadRequest, models.ChatResponse{
				Error: "No user message found in conversation",
			})
		}

		fmt.Printf("[CHAT_VECTOR] Extracted user query: '%s'\n", userQuery)

		// Search for similar products using vector embeddings
		fmt.Printf("[CHAT_VECTOR] Starting vector search for products...\n")
		similarProducts, err := embeddingService.SearchSimilarProducts(userQuery, 20) // Get top 20 most similar products
		if err != nil {
			fmt.Printf("[CHAT_VECTOR] ERROR: Vector search failed: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("Failed to search products: %v", err),
			})
		}

		fmt.Printf("[CHAT_VECTOR] Vector search returned %d products\n", len(similarProducts))

		// Filter to only in-stock products
		fmt.Printf("[CHAT_VECTOR] Filtering for in-stock products...\n")
		var inStockProducts []embeddings.ProductEmbedding
		for _, product := range similarProducts {
			if product.Product.StockStatus != nil && *product.Product.StockStatus == "instock" {
				inStockProducts = append(inStockProducts, product)
			}
		}

		fmt.Printf("[CHAT_VECTOR] Found %d in-stock products out of %d total\n", len(inStockProducts), len(similarProducts))

		// If no in-stock products found, use all products
		if len(inStockProducts) == 0 {
			fmt.Printf("[CHAT_VECTOR] No in-stock products found, using all products\n")
			inStockProducts = similarProducts
		}


		// Create product metadata map for frontend
		fmt.Printf("[CHAT_VECTOR] Creating product metadata for frontend...\n")
		productMetadata := make(map[string]string)
		metadataCount := 0
		for _, product := range inStockProducts {
			// Use PostName if available, otherwise use SKU, otherwise use product ID
			if product.Product.PostName != nil && *product.Product.PostName != "" {
				productMetadata[product.Product.PostTitle] = *product.Product.PostName
				metadataCount++
			} else if product.Product.SKU != nil && *product.Product.SKU != "" {
				productMetadata[product.Product.PostTitle] = *product.Product.SKU
				metadataCount++
			} else {
				// Fallback to product ID
				productMetadata[product.Product.PostTitle] = fmt.Sprintf("product-%d", product.Product.ID)
				metadataCount++
			}
		}

		fmt.Printf("[CHAT_VECTOR] Created metadata for %d products\n", metadataCount)


		// Build conversation messages for OpenAI
		fmt.Printf("[CHAT_VECTOR] Building OpenAI messages...\n")
		messages := buildVectorOpenAIMessages(req.Conversation, inStockProducts, utils.Language{Code: utils.LangEnglish, Name: "English", Confidence: 1.0})
		fmt.Printf("[CHAT_VECTOR] Built %d messages for OpenAI\n", len(messages))

		// Create OpenAI client
		fmt.Printf("[CHAT_VECTOR] Creating OpenAI client...\n")
		client := openai.NewClient(cfg.OpenAIKey)

		// Create chat completion request
		fmt.Printf("[CHAT_VECTOR] Sending request to OpenAI API...\n")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenAITimeout)*time.Second)
		defer cancel()

		resp, err := client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:       openai.GPT4oMini,
				Messages:    messages,
				MaxTokens:   1000,
				Temperature: 0.7,
			},
		)

		if err != nil {
			fmt.Printf("[CHAT_VECTOR] ERROR: OpenAI API error: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("OpenAI API error: %v", err),
			})
		}

		if len(resp.Choices) == 0 {
			fmt.Printf("[CHAT_VECTOR] ERROR: No response from OpenAI\n")
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: "No response from OpenAI",
			})
		}

		fmt.Printf("[CHAT_VECTOR] OpenAI response received successfully\n")

		// Create response with all found products
		response := resp.Choices[0].Message.Content
		if len(inStockProducts) > 0 {
			response += fmt.Sprintf("\n\n**Found %d relevant products** (showing top matches with similarity scores):", len(inStockProducts))
		}

		fmt.Printf("[CHAT_VECTOR] Final response: %d characters, %d products in metadata\n", len(response), len(productMetadata))
		fmt.Printf("[CHAT_VECTOR] ===== CHAT REQUEST COMPLETE =====\n\n")

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response: response,
			Products: productMetadata,
		})
	}
}

// buildVectorOpenAIMessages creates OpenAI messages with vector search results
func buildVectorOpenAIMessages(conversation []models.ConversationMessage, products []embeddings.ProductEmbedding, detectedLang utils.Language) []openai.ChatCompletionMessage {
	fmt.Printf("[BUILD_MESSAGES] Building OpenAI messages with %d products\n", len(products))
	
	// Create the main system prompt
	systemPrompt := `You are a sales rep for Israel Defense Store (israeldefensestore.com) specializing in tactical gear.

ROLE: Help customers find tactical gear products. You have access to a list of relevant products found using advanced vector search.

RULES:
- Only recommend products from the provided list
- Use product tags for compatibility (Glock, Right/Left Hand, Black/Od Green, OWB/IWB, etc.)
- Check stock status before recommending
- Provide pricing and availability details
- Format responses with **bold** for product names
- Use bullet points for lists
- Show the most relevant products first (they are ranked by similarity)
- Include similarity scores when helpful

RESPONSE FORMAT: **[Product Name]** - [Description] - Price: [Price Range] - Similarity: [Score]`

	// Add language instruction
	languageInstruction := utils.GetLanguageInstruction(detectedLang)

	// Build product context from vector search results
	fmt.Printf("[BUILD_MESSAGES] Building product context...\n")
	var productContext strings.Builder
	productContext.WriteString("RELEVANT PRODUCTS (ranked by similarity to your query):\n\n")

	contextProducts := 0
	for i, product := range products {
		if i >= 15 { // Limit to top 15 for context to avoid token limits
			productContext.WriteString(fmt.Sprintf("\n... and %d more products available", len(products)-15))
			break
		}
		contextProducts++

		productContext.WriteString(fmt.Sprintf("**%s**", product.Product.PostTitle))

		// Add price
		if product.Product.MinPrice != nil && product.Product.MaxPrice != nil {
			if *product.Product.MinPrice == *product.Product.MaxPrice {
				productContext.WriteString(fmt.Sprintf(" - $%s", *product.Product.MinPrice))
			} else {
				productContext.WriteString(fmt.Sprintf(" - $%s-$%s", *product.Product.MinPrice, *product.Product.MaxPrice))
			}
		}

		// Add stock status
		if product.Product.StockStatus != nil {
			if *product.Product.StockStatus == "instock" {
				productContext.WriteString(" - In Stock")
			} else {
				productContext.WriteString(" - Out of Stock")
			}
		}

		// Add similarity score
		productContext.WriteString(fmt.Sprintf(" - Similarity: %.2f", product.Similarity))

		// Add tags if available
		if product.Product.Tags != nil && *product.Product.Tags != "" {
			productContext.WriteString(fmt.Sprintf(" - Tags: %s", *product.Product.Tags))
		}

		productContext.WriteString("\n")
	}

	fmt.Printf("[BUILD_MESSAGES] Added %d products to context (total available: %d)\n", contextProducts, len(products))

	// Combine system prompt, product context, and language instruction
	enhancedContext := systemPrompt + "\n\n" + productContext.String() + "\n\n" + languageInstruction
	fmt.Printf("[BUILD_MESSAGES] Enhanced context length: %d characters\n", len(enhancedContext))

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: enhancedContext,
		},
	}

	// Add conversation messages
	fmt.Printf("[BUILD_MESSAGES] Adding %d conversation messages\n", len(conversation))
	for i, msg := range conversation {
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
		fmt.Printf("[BUILD_MESSAGES] Added message %d: %s (%d chars)\n", i+1, role, len(msg.Message))
	}

	fmt.Printf("[BUILD_MESSAGES] Built %d total messages for OpenAI\n", len(messages))
	return messages
}
