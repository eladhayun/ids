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
	"ids/internal/database"
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

		// Extract relevant tags from conversation to filter products
		relevantTags := extractRelevantTags(req.Conversation)

		// Get product data from database to provide context (with caching and tag filtering)
		productData, err := getProductDataForContext(db, cache, cfg.ProductCacheTTL, relevantTags)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, models.ChatResponse{
				Error: fmt.Sprintf("Failed to fetch product data: %v", err),
			})
		}

		// Build conversation messages for OpenAI
		messages := buildOpenAIMessages(req.Conversation, productData.Context, utils.Language{Code: utils.LangEnglish, Name: "English", Confidence: 1.0})

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

		// Create a comprehensive response that includes all products
		response := resp.Choices[0].Message.Content
		
		// If we have many products, add a note about the total count
		if len(productData.Products) > 3 {
			response += fmt.Sprintf("\n\n**Note: We have %d total products matching your search. All products are listed above with clickable links.**", len(productData.Products))
		}

		return c.JSON(http.StatusOK, models.ChatResponse{
			Response: response,
			Products: productData.Products,
		})
	}
}

// extractRelevantTags analyzes conversation to find relevant product tags
func extractRelevantTags(conversation []models.ConversationMessage) []string {
	var allMessages strings.Builder
	for _, msg := range conversation {
		if strings.Contains(strings.ToLower(msg.Role), "user") {
			allMessages.WriteString(msg.Message + " ")
		}
	}

	conversationText := strings.ToLower(allMessages.String())
	if conversationText == "" {
		return nil
	}

	// Common tag keywords and their variations (mapped to database tag names)
	tagKeywords := map[string][]string{
		"Glock":                {"glock", "glock 17", "glock 19", "glock 21", "glock 22", "glock 26", "glock 30", "glock 36", "glock 37", "glock 45"},
		"Right Hand":           {"right hand", "right-handed", "righty", "right"},
		"Left Hand":            {"left hand", "left-handed", "lefty", "left"},
		"Black":                {"black", "black color"},
		"Od Green":             {"od green", "olive drab", "green"},
		"For Pistols":          {"pistol", "pistols", "handgun", "handguns", "for pistol"},
		"For Rifles":           {"rifle", "rifles", "long gun", "for rifle"},
		"OWB":                  {"owb", "outside waistband", "outside the waistband"},
		"IWB":                  {"iwb", "inside waistband", "inside the waistband"},
		"Paddle":               {"paddle", "paddle holster"},
		"Belt Loop":            {"belt loop", "belt loops"},
		"Molle":                {"molle", "molle compatible"},
		"Duty Holster":         {"duty holster", "duty", "professional"},
		"Polymer Holster":      {"polymer", "plastic", "polymer holster"},
		"Leather":              {"leather"},
		"Modular":              {"modular", "modular system"},
		"Thigh Rig / Drop Leg": {"drop leg", "thigh rig", "drop leg holster"},
		"Fobus":                {"fobus"},
		"DPM":                  {"dpm"},
		"FAB Defense":          {"fab defense", "fab"},
		"ORPAZ Defense":        {"orpaz", "orpaz defense"},
		".40 Cal":              {".40", ".40 cal", "40 cal", "40 caliber"},
		".45 Cal":              {".45", ".45 cal", "45 cal", "45 caliber", ".45 acp"},
		"9mm":                  {"9mm", "9 mm"},
		".357":                 {".357", "357"},
		".223":                 {".223", "223"},
		".22 Cal":              {".22", ".22 cal", "22 cal"},
	}

	var detectedTags []string
	for tagName, keywords := range tagKeywords {
		for _, keyword := range keywords {
			if strings.Contains(conversationText, keyword) {
				// Check if tag already added
				found := false
				for _, existingTag := range detectedTags {
					if existingTag == tagName {
						found = true
						break
					}
				}
				if !found {
					detectedTags = append(detectedTags, tagName)
				}
				break
			}
		}
	}

	return detectedTags
}

// productContextData holds both the context string and product metadata
type productContextData struct {
	Context  string
	Products map[string]string // Product name -> SKU mapping
}

// getProductDataForContext fetches product data to provide context to the LLM (with caching and optional tag filtering)
func getProductDataForContext(db *sqlx.DB, cache *cache.Cache, cacheTTLMinutes int, filterTags []string) (*productContextData, error) {
	// Create cache key based on filters
	var cacheKey string
	if len(filterTags) > 0 {
		cacheKey = fmt.Sprintf("product_context_data_filtered_%s", strings.Join(filterTags, "_"))
	} else {
		cacheKey = "product_context_data"
	}

	// Try to get from cache first
	if cachedData, found := cache.Get(cacheKey); found {
		if productData, ok := cachedData.(*productContextData); ok {
			return productData, nil
		}
	}

	// Cache miss or invalid data, fetch from database
	// Build query with optional tag filtering
	var query string
	var args []interface{}

	if len(filterTags) > 0 {
		// Query with tag filtering - products must have at least one matching tag
		query = `
			SELECT DISTINCT
			  p.ID,
			  p.post_title,
			  p.post_name,
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
			JOIN wpjr_term_relationships tr ON tr.object_id = p.ID
			JOIN wpjr_term_taxonomy tt ON tt.term_taxonomy_id = tr.term_taxonomy_id
			  AND tt.taxonomy = 'product_tag'
			JOIN wpjr_terms t ON t.term_id = tt.term_id
			WHERE p.post_type = 'product'
			  AND p.post_status IN ('publish','private')
			  AND t.name IN (` + strings.Repeat("?,", len(filterTags)-1) + `?)
			GROUP BY
			  p.ID, p.post_title, p.post_name, p.post_content, p.post_excerpt,
			  l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
			ORDER BY p.ID
		`
		// Add filter tags as query arguments
		for _, tag := range filterTags {
			args = append(args, tag)
		}
	} else {
		// Query without tag filtering - get all products
		query = `
			SELECT
			  p.ID,
			  p.post_title,
			  p.post_name,
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
			  p.ID, p.post_title, p.post_name, p.post_content, p.post_excerpt,
			  l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
			ORDER BY p.ID
		`
		args = nil
	}

	var products []models.Product
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if len(filterTags) > 0 {
		err = database.ExecuteReadOnlyQuery(ctx, db, &products, query, args...)
	} else {
		err = database.ExecuteReadOnlyQuery(ctx, db, &products, query)
	}
	if err != nil {
		return nil, err
	}

	// Format product data as context string and build product metadata map
	var contextBuilder strings.Builder
	productMetadata := make(map[string]string)

	if len(filterTags) > 0 {
		contextBuilder.WriteString(fmt.Sprintf("PRODUCTS (filtered by: %s) - TOTAL COUNT: %d:\n", strings.Join(filterTags, ", "), len(products)))
	} else {
		contextBuilder.WriteString(fmt.Sprintf("AVAILABLE PRODUCTS - TOTAL COUNT: %d:\n", len(products)))
	}

	for _, product := range products {
		// Very simplified product format to reduce tokens
		contextBuilder.WriteString(fmt.Sprintf("**%s**", product.PostTitle))

		if product.MinPrice != nil && product.MaxPrice != nil {
			if *product.MinPrice == *product.MaxPrice {
				contextBuilder.WriteString(fmt.Sprintf(" - $%s", *product.MinPrice))
			} else {
				contextBuilder.WriteString(fmt.Sprintf(" - $%s-$%s", *product.MinPrice, *product.MaxPrice))
			}
		}

		if product.StockStatus != nil && *product.StockStatus != "instock" {
			contextBuilder.WriteString(" (Out of Stock)")
		}

		// Store product name -> URL slug mapping for frontend
		if product.PostName != nil && *product.PostName != "" {
			productMetadata[product.PostTitle] = *product.PostName
		} else if product.SKU != nil && *product.SKU != "" {
			// Fallback to SKU if no post_name available
			productMetadata[product.PostTitle] = *product.SKU
		}

		contextBuilder.WriteString("\n")
	}

	contextBuilder.WriteString(fmt.Sprintf("Show a good selection of products from the %d available above. Focus on variety and quality.\n", len(products)))

	productData := &productContextData{
		Context:  contextBuilder.String(),
		Products: productMetadata,
	}

	// Cache the result for the specified TTL
	cache.Set(cacheKey, productData, time.Duration(cacheTTLMinutes)*time.Minute)

	return productData, nil
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
func buildOpenAIMessages(conversation []models.ConversationMessage, productContext string, detectedLang utils.Language) []openai.ChatCompletionMessage {
	// Create the main system prompt for tactical gear sales
	systemPrompt := `You are a sales rep for Israel Defense Store (israeldefensestore.com) specializing in tactical gear.

ROLE: Help customers find tactical gear products. Only recommend products from our database. Be professional and knowledgeable.

RULES:
- Only recommend in-stock products from provided data
- Use product tags for compatibility (Glock, Right/Left Hand, Black/Od Green, OWB/IWB, etc.)
- Check stock status before recommending
- Provide pricing and availability details
- Format responses with **bold** for product names
- Use bullet points for lists
- Show a good selection of products (5-10) that represent the variety available
- Mention that there are more products available if the count is high

RESPONSE FORMAT: **[Product Name]** - [Description] - Price: [Price Range]`

	// Add language instruction to the system prompt
	languageInstruction := utils.GetLanguageInstruction(detectedLang)

	// Get database metadata for AI context
	databaseMetadata := getDatabaseMetadata()

	// Combine system prompt, database metadata, product context, and language instruction
	enhancedContext := systemPrompt + "\n\n" + databaseMetadata + "\n\n" + productContext + "\n\n" + languageInstruction

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
