package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ids/internal/analytics"
	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/emails"
	"ids/internal/embeddings"
	"ids/internal/models"
	idsopenai "ids/internal/openai"
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
//
//nolint:gocyclo // Handler has necessary complexity for validation, search, and response building
func ChatHandler(db *sqlx.DB, cfg *config.Config, cache *cache.Cache, embeddingService *embeddings.EmbeddingService, writeClient *database.WriteClient, analyticsService *analytics.Service, conversationService *database.ConversationService) echo.HandlerFunc {
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

		// Track query embedding (billable - 1 embedding per product search)
		if analyticsService != nil {
			go func() { _ = analyticsService.TrackQueryEmbedding("product_search", "text-embedding-3-small") }()
		}

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
				// Track query embedding (billable - 1 embedding per email search)
				if analyticsService != nil {
					go func() { _ = analyticsService.TrackQueryEmbedding("email_search", "text-embedding-3-small") }()
				}
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

		// Create unified OpenAI client (Azure primary, OpenAI fallback) and get response
		client, err := idsopenai.NewClient(cfg)
		if err != nil {
			fmt.Printf("[CHAT] ERROR: Failed to create OpenAI client: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("Failed to create OpenAI client: %v", err),
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenAITimeout)*time.Second)
		defer cancel()

		fmt.Printf("[CHAT] Sending chat request to %s...\n", client.GetProviderName())
		resp, err := client.CreateChatCompletion(ctx, messages, 1500, 0.7)

		if err != nil {
			fmt.Printf("[CHAT] ERROR: %s API error: %v\n", client.GetProviderName(), err)
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("%s API error: %v", client.GetProviderName(), err),
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

		// Track analytics
		if analyticsService != nil {
			totalTokens := 0
			if resp.Usage.TotalTokens > 0 {
				totalTokens = resp.Usage.TotalTokens
			}
			go func() {
				if err := analyticsService.TrackConversation(len(inStockProducts), len(similarEmails), totalTokens, string(openai.GPT4oMini)); err != nil {
					fmt.Printf("[CHAT] Warning: Failed to track analytics: %v\n", err)
				}
			}()
		}

		// Detect if customer is dissatisfied and needs support escalation
		requestSupport := detectDissatisfaction(
			req.Conversation,
			userQuery,
			inStockProducts,
			similarEmails,
		)

		if requestSupport {
			response += "\n\nI notice you might need additional assistance. Would you like me to send this conversation to our support team? Please provide your email address so we can help you better."
			fmt.Printf("[CHAT] ⚠️  Dissatisfaction detected - requesting support escalation\n")
		}

		fmt.Printf("[CHAT] 📊 DATASOURCE SUMMARY: Used %d product embeddings, %d email embeddings\n", len(inStockProducts), len(similarEmails))

		// Save conversation to database if session_id is provided and conversation service is available
		if req.SessionID != "" && conversationService != nil {
			go func() {
				// Save all conversation messages (user and assistant)
				for _, msg := range req.Conversation {
					role := "user"
					if strings.Contains(strings.ToLower(msg.Role), "assistant") ||
						strings.Contains(strings.ToLower(msg.Role), "bot") ||
						strings.Contains(strings.ToLower(msg.Role), "ai") {
						role = "assistant"
					}
					if err := conversationService.SaveMessage(req.SessionID, role, msg.Message); err != nil {
						fmt.Printf("[CHAT] Warning: Failed to save message: %v\n", err)
					}
				}
				// Save the AI response
				if err := conversationService.SaveMessage(req.SessionID, "assistant", response); err != nil {
					fmt.Printf("[CHAT] Warning: Failed to save AI response: %v\n", err)
				}
			}()
		} else if req.SessionID == "" {
			fmt.Printf("[CHAT] Warning: No session_id provided, conversation not saved\n")
		}

		fmt.Printf("[CHAT] ===== REQUEST COMPLETE =====\n\n")

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response:       response,
			Products:       productMetadata,
			RequestSupport: requestSupport,
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

// detectDissatisfaction uses heuristics to detect if customer needs support escalation
func detectDissatisfaction(
	conversation []models.ConversationMessage,
	currentQuery string,
	products []embeddings.ProductEmbedding,
	similarEmails []models.EmailSearchResult,
) bool {
	// 1. Check for repeated questions
	if hasRepeatedQuestions(conversation) {
		fmt.Printf("[DETECTION] Repeated questions detected\n")
		return true
	}

	// 2. Check for dissatisfaction keywords
	if hasDissatisfactionKeywords(currentQuery) {
		fmt.Printf("[DETECTION] Dissatisfaction keywords detected\n")
		return true
	}

	// 3. Check for no products found when query seems product-related
	if hasProductRelatedQueryButNoResults(currentQuery, products) {
		fmt.Printf("[DETECTION] Product-related query with no results\n")
		return true
	}

	// 4. Check for low similarity scores
	if hasLowSimilarityScores(products, similarEmails) {
		fmt.Printf("[DETECTION] Low similarity scores detected\n")
		return true
	}

	return false
}

// hasRepeatedQuestions checks if user asks similar questions multiple times
func hasRepeatedQuestions(conversation []models.ConversationMessage) bool {
	// Get last 5 user messages
	var userMessages []string
	count := 0
	for i := len(conversation) - 1; i >= 0 && count < 5; i-- {
		if strings.Contains(strings.ToLower(conversation[i].Role), "user") {
			userMessages = append(userMessages, strings.ToLower(strings.TrimSpace(conversation[i].Message)))
			count++
		}
	}

	// Check for similarity between any two messages
	for i := 0; i < len(userMessages); i++ {
		for j := i + 1; j < len(userMessages); j++ {
			similarity := calculateSimilarity(userMessages[i], userMessages[j])
			if similarity > 0.7 {
				return true
			}
		}
	}

	return false
}

// calculateSimilarity calculates simple word overlap similarity between two strings
func calculateSimilarity(s1, s2 string) float64 {
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Count common words
	common := 0
	wordMap := make(map[string]bool)
	for _, word := range words1 {
		wordMap[word] = true
	}
	for _, word := range words2 {
		if wordMap[word] {
			common++
		}
	}

	// Return similarity as ratio of common words to total unique words
	totalUnique := len(wordMap)
	for _, word := range words2 {
		if !wordMap[word] {
			totalUnique++
		}
	}

	if totalUnique == 0 {
		return 0.0
	}

	return float64(common) / float64(totalUnique)
}

// hasDissatisfactionKeywords checks for keywords indicating dissatisfaction
func hasDissatisfactionKeywords(query string) bool {
	queryLower := strings.ToLower(query)
	dissatisfactionKeywords := []string{
		"not helpful",
		"wrong",
		"doesn't work",
		"still don't",
		"not what i",
		"incorrect",
		"useless",
		"not working",
		"doesn't help",
		"can't find",
		"no results",
		"nothing found",
		"not satisfied",
		"not good",
		"bad",
	}

	for _, keyword := range dissatisfactionKeywords {
		if strings.Contains(queryLower, keyword) {
			return true
		}
	}

	return false
}

// hasProductRelatedQueryButNoResults checks if query seems product-related but no products found
func hasProductRelatedQueryButNoResults(query string, products []embeddings.ProductEmbedding) bool {
	if len(products) > 0 {
		return false
	}

	queryLower := strings.ToLower(query)
	productKeywords := []string{
		"product",
		"item",
		"have",
		"stock",
		"available",
		"show",
		"find",
		"search",
		"looking for",
		"need",
		"want",
		"buy",
		"purchase",
	}

	hasProductKeyword := false
	for _, keyword := range productKeywords {
		if strings.Contains(queryLower, keyword) {
			hasProductKeyword = true
			break
		}
	}

	return hasProductKeyword
}

// hasLowSimilarityScores checks if both product and email search returned low similarity
func hasLowSimilarityScores(products []embeddings.ProductEmbedding, similarEmails []models.EmailSearchResult) bool {
	// Check product similarity
	hasLowProductSimilarity := true
	if len(products) > 0 {
		// Check if any product has similarity >= 0.3
		for _, product := range products {
			if product.Similarity >= 0.3 {
				hasLowProductSimilarity = false
				break
			}
		}
	}

	// Check email similarity
	hasLowEmailSimilarity := true
	if len(similarEmails) > 0 {
		// Check if any email has similarity >= 0.3
		for _, email := range similarEmails {
			if email.Similarity >= 0.3 {
				hasLowEmailSimilarity = false
				break
			}
		}
	}

	// If both are low, it's a sign of dissatisfaction
	return hasLowProductSimilarity && hasLowEmailSimilarity && (len(products) > 0 || len(similarEmails) > 0)
}
