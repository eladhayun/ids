package embeddings

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"ids/internal/config"
	"ids/internal/database"
	"ids/internal/models"
	"ids/internal/utils"

	"github.com/jmoiron/sqlx"
	"github.com/sashabaranov/go-openai"
)

// EmbeddingService handles vector embeddings for products
type EmbeddingService struct {
	client      *openai.Client
	db          *sqlx.DB              // MariaDB - only for reading product data when generating embeddings
	writeClient *database.WriteClient // PostgreSQL - for searching embeddings
	tagTokenSet map[string]struct{}
}

// ProductEmbedding represents a product with its vector embedding
type ProductEmbedding struct {
	Product    models.Product `json:"product"`
	Embedding  []float64      `json:"embedding"`
	Similarity float64        `json:"similarity,omitempty"`
}

// NewEmbeddingService creates a new embedding service
// db: MariaDB connection (only for reading product data when generating embeddings)
// writeClient: PostgreSQL connection (for searching embeddings)
func NewEmbeddingService(cfg *config.Config, db *sqlx.DB, writeClient *database.WriteClient) (*EmbeddingService, error) {
	client := openai.NewClient(cfg.OpenAIKey)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{"test"},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OpenAI API: %v", err)
	}

	service := &EmbeddingService{
		client:      client,
		db:          db,
		writeClient: writeClient,
	}

	// Load tag tokens from MariaDB (only needed when generating embeddings)
	if db != nil {
		if err := service.loadTagTokens(); err != nil {
			fmt.Printf("[EMBEDDING_SERVICE] WARNING: Failed to load tag tokens for filtering: %v\n", err)
		}
	}

	return service, nil
}

func (es *EmbeddingService) loadTagTokens() error {
	fmt.Printf("[EMBEDDING_SERVICE] Loading product tag tokens for query filtering...\n")

	query := `
		SELECT DISTINCT t.name
		FROM wpjr_terms t
		JOIN wpjr_term_taxonomy tt ON tt.term_id = t.term_id
		WHERE tt.taxonomy = 'product_tag'
	`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tagNames []string
	if err := es.db.SelectContext(ctx, &tagNames, query); err != nil {
		return fmt.Errorf("failed to load product tags: %w", err)
	}

	tokenSet := make(map[string]struct{})
	for _, name := range tagNames {
		tokens := utils.ExtractMeaningfulTokens(name)
		for _, token := range tokens {
			tokenSet[token] = struct{}{}
		}
	}

	es.tagTokenSet = tokenSet
	fmt.Printf("[EMBEDDING_SERVICE] Loaded %d unique tag tokens\n", len(tokenSet))
	return nil
}

// GenerateProductEmbeddings generates embeddings for all products
func (es *EmbeddingService) GenerateProductEmbeddings() error {
	fmt.Printf("[EMBEDDING_GEN] ===== STARTING EMBEDDING GENERATION =====\n")

	// Get all products from database
	query := `
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
		LEFT JOIN wpjr_term_relationships tr ON tr.object_id = p.ID
		LEFT JOIN wpjr_term_taxonomy tt ON tt.term_taxonomy_id = tr.term_taxonomy_id
			AND tt.taxonomy = 'product_tag'
		LEFT JOIN wpjr_terms t ON t.term_id = tt.term_id
		WHERE p.post_type = 'product'
			AND p.post_status IN ('publish','private')
		GROUP BY
			p.ID, p.post_title, p.post_name, p.post_content, p.post_excerpt,
			l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
		ORDER BY p.ID
	`

	fmt.Printf("[EMBEDDING_GEN] Fetching products from database...\n")
	var products []models.Product
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := es.db.SelectContext(ctx, &products, query)
	if err != nil {
		fmt.Printf("[EMBEDDING_GEN] ERROR: Failed to fetch products: %v\n", err)
		return fmt.Errorf("failed to fetch products: %v", err)
	}

	fmt.Printf("[EMBEDDING_GEN] Found %d products to process\n", len(products))

	// Process products in batches to avoid API limits
	batchSize := 100
	totalBatches := (len(products) + batchSize - 1) / batchSize
	fmt.Printf("[EMBEDDING_GEN] Processing %d products in %d batches of %d\n", len(products), totalBatches, batchSize)

	for i := 0; i < len(products); i += batchSize {
		end := i + batchSize
		if end > len(products) {
			end = len(products)
		}

		batchNum := (i / batchSize) + 1
		fmt.Printf("[EMBEDDING_GEN] Processing batch %d/%d (products %d-%d)...\n", batchNum, totalBatches, i+1, end)

		batch := products[i:end]
		if err := es.processBatch(batch); err != nil {
			fmt.Printf("[EMBEDDING_GEN] ERROR: Failed to process batch %d-%d: %v\n", i, end, err)
			return fmt.Errorf("failed to process batch %d-%d: %v", i, end, err)
		}

		fmt.Printf("[EMBEDDING_GEN] Completed batch %d/%d\n", batchNum, totalBatches)
	}

	fmt.Printf("[EMBEDDING_GEN] ===== EMBEDDING GENERATION COMPLETE =====\n")
	return nil
}

// processBatch processes a batch of products and generates embeddings
func (es *EmbeddingService) processBatch(products []models.Product) error {
	fmt.Printf("[EMBEDDING_GEN] Processing batch of %d products\n", len(products))

	// Prepare texts for embedding
	fmt.Printf("[EMBEDDING_GEN] Building product texts...\n")
	texts := make([]string, len(products))
	for i, product := range products {
		texts[i] = es.buildProductText(product)
	}

	// Generate embeddings
	fmt.Printf("[EMBEDDING_GEN] Sending batch to OpenAI API...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := es.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		fmt.Printf("[EMBEDDING_GEN] ERROR: Failed to generate embeddings: %v\n", err)
		return fmt.Errorf("failed to generate embeddings: %v", err)
	}

	fmt.Printf("[EMBEDDING_GEN] Received %d embeddings from OpenAI\n", len(resp.Data))

	// Store embeddings in database
	fmt.Printf("[EMBEDDING_GEN] Storing embeddings in database...\n")
	for i, embeddingData := range resp.Data {
		product := products[i]
		// Convert []float32 to []float64
		embedding := make([]float64, len(embeddingData.Embedding))
		for j, v := range embeddingData.Embedding {
			embedding[j] = float64(v)
		}
		if err := es.storeEmbedding(product, embedding); err != nil {
			fmt.Printf("[EMBEDDING_GEN] ERROR: Failed to store embedding for product %d: %v\n", product.ID, err)
			return fmt.Errorf("failed to store embedding for product %d: %v", product.ID, err)
		}
	}

	fmt.Printf("[EMBEDDING_GEN] Successfully stored %d embeddings\n", len(resp.Data))
	return nil
}

// buildProductText creates a comprehensive text representation of a product
func (es *EmbeddingService) buildProductText(product models.Product) string {
	var parts []string

	// Add title
	if product.PostTitle != "" {
		parts = append(parts, product.PostTitle)
	}

	// Add description
	if product.Description != nil && *product.Description != "" {
		// Clean HTML tags and limit length
		desc := strings.ReplaceAll(*product.Description, "<br>", " ")
		desc = strings.ReplaceAll(desc, "<p>", " ")
		desc = strings.ReplaceAll(desc, "</p>", " ")
		desc = strings.ReplaceAll(desc, "<div>", " ")
		desc = strings.ReplaceAll(desc, "</div>", " ")
		desc = strings.ReplaceAll(desc, "<span>", " ")
		desc = strings.ReplaceAll(desc, "</span>", " ")
		desc = strings.TrimSpace(desc)
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}
		parts = append(parts, desc)
	}

	// Add short description
	if product.ShortDescription != nil && *product.ShortDescription != "" {
		parts = append(parts, *product.ShortDescription)
	}

	// Add tags
	if product.Tags != nil && *product.Tags != "" {
		parts = append(parts, "Tags: "+*product.Tags)
	}

	// Add SKU
	if product.SKU != nil && *product.SKU != "" {
		parts = append(parts, "SKU: "+*product.SKU)
	}

	// Add price range
	if product.MinPrice != nil && product.MaxPrice != nil {
		if *product.MinPrice == *product.MaxPrice {
			parts = append(parts, fmt.Sprintf("Price: $%s", *product.MinPrice))
		} else {
			parts = append(parts, fmt.Sprintf("Price: $%s - $%s", *product.MinPrice, *product.MaxPrice))
		}
	}

	// Add stock status
	if product.StockStatus != nil {
		parts = append(parts, "Stock: "+*product.StockStatus)
	}

	return strings.Join(parts, " | ")
}

// storeEmbedding stores a product embedding in the database
func (es *EmbeddingService) storeEmbedding(product models.Product, embedding []float64) error {
	// Convert embedding to JSON
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %v", err)
	}

	// Store in database (we'll create a table for this)
	query := `
		INSERT INTO product_embeddings (product_id, embedding, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			embedding = VALUES(embedding),
			updated_at = NOW()
	`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = es.db.ExecContext(ctx, query, product.ID, string(embeddingJSON))
	if err != nil {
		return fmt.Errorf("failed to store embedding: %v", err)
	}

	return nil
}

// SearchSimilarProducts finds products similar to the query using vector similarity
func (es *EmbeddingService) SearchSimilarProducts(query string, limit int) ([]ProductEmbedding, bool, error) {
	fmt.Printf("[PRODUCT_EMBEDDINGS] 🔍 Querying PRODUCT EMBEDDINGS datasource - Query: '%s', Limit: %d\n", query, limit)

	// Generate embedding for the query
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("[VECTOR_SEARCH] Generating query embedding...\n")
	resp, err := es.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{query},
		Model: openai.SmallEmbedding3,
	})
	if err != nil {
		fmt.Printf("[VECTOR_SEARCH] ERROR: Failed to generate query embedding: %v\n", err)
		return nil, false, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	fmt.Printf("[VECTOR_SEARCH] Query embedding generated successfully (dimensions: %d)\n", len(resp.Data[0].Embedding))

	// Convert []float32 to []float64
	queryEmbedding := make([]float64, len(resp.Data[0].Embedding))
	for j, v := range resp.Data[0].Embedding {
		queryEmbedding[j] = float64(v)
	}

	// Get all product embeddings from PostgreSQL (no MariaDB joins)
	// Product metadata is stored denormalized in the product_embeddings table
	embeddingsQuery := `
		SELECT
			product_id,
			embedding,
			COALESCE(post_title, '') as post_title,
			post_name,
			description,
			short_description,
			sku,
			min_price,
			max_price,
			stock_status,
			stock_quantity,
			tags
		FROM product_embeddings
		WHERE post_title IS NOT NULL AND post_title != ''
	`

	fmt.Printf("[PRODUCT_EMBEDDINGS] Fetching product embeddings from PostgreSQL (no MariaDB access)...\n")
	if es.writeClient == nil {
		return nil, false, fmt.Errorf("PostgreSQL write client not available for product embeddings search")
	}
	rows, err := es.writeClient.GetDB().QueryContext(ctx, embeddingsQuery)
	if err != nil {
		fmt.Printf("[PRODUCT_EMBEDDINGS] ❌ ERROR: Failed to fetch product embeddings from PostgreSQL: %v\n", err)
		return nil, false, fmt.Errorf("failed to fetch product embeddings: %v", err)
	}
	defer rows.Close()

	var results []ProductEmbedding
	processedCount := 0
	skippedCount := 0

	for rows.Next() {
		var productID int
		var embeddingJSON string
		var product models.Product

		// Use sql.NullString for nullable fields
		var postName, description, shortDescription, sku, minPrice, maxPrice, stockStatus, tags sql.NullString
		var stockQuantity sql.NullFloat64

		err := rows.Scan(
			&productID,
			&embeddingJSON,
			&product.PostTitle,
			&postName,
			&description,
			&shortDescription,
			&sku,
			&minPrice,
			&maxPrice,
			&stockStatus,
			&stockQuantity,
			&tags,
		)

		// Convert nullable fields to pointers
		if postName.Valid {
			product.PostName = &postName.String
		}
		if description.Valid {
			product.Description = &description.String
		}
		if shortDescription.Valid {
			product.ShortDescription = &shortDescription.String
		}
		if sku.Valid {
			product.SKU = &sku.String
		}
		if minPrice.Valid {
			product.MinPrice = &minPrice.String
		}
		if maxPrice.Valid {
			product.MaxPrice = &maxPrice.String
		}
		if stockStatus.Valid {
			product.StockStatus = &stockStatus.String
		}
		if stockQuantity.Valid {
			product.StockQuantity = &stockQuantity.Float64
		}
		if tags.Valid {
			product.Tags = &tags.String
		}

		if err != nil {
			skippedCount++
			continue // Skip invalid rows
		}

		// Parse embedding
		var embedding []float64
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			skippedCount++
			continue // Skip invalid embeddings
		}

		// Calculate cosine similarity
		similarity := es.cosineSimilarity(queryEmbedding, embedding)

		product.ID = productID
		results = append(results, ProductEmbedding{
			Product:    product,
			Embedding:  embedding,
			Similarity: similarity,
		})
		processedCount++
	}

	fmt.Printf("[VECTOR_SEARCH] Processed %d products, skipped %d invalid entries\n", processedCount, skippedCount)

	// Sort by similarity (highest first)
	fmt.Printf("[VECTOR_SEARCH] Sorting %d results by similarity...\n", len(results))
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Similarity < results[j].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Log top 5 results for debugging
	if len(results) > 0 {
		fmt.Printf("[VECTOR_SEARCH] Top 5 most similar products:\n")
		for i := 0; i < 5 && i < len(results); i++ {
			stockStatus := "unknown"
			if results[i].Product.StockStatus != nil {
				stockStatus = *results[i].Product.StockStatus
			}
			fmt.Printf("  %d. %s (similarity: %.3f, stock: %s)\n",
				i+1, results[i].Product.PostTitle, results[i].Similarity, stockStatus)
		}
	}

	requiredTokens := es.requiredTokensFromQuery(query)
	fallbackToSimilarity := false
	if len(requiredTokens) > 0 {
		fmt.Printf("[VECTOR_SEARCH] Applying exact-match filtering with tokens: %v\n", requiredTokens)

		var filteredResults []ProductEmbedding
		for _, result := range results {
			productTokenSet := buildProductTokenSet(result.Product)
			if ok, missing := utils.ContainsAllTokens(productTokenSet, requiredTokens); ok {
				filteredResults = append(filteredResults, result)
			} else {
				fmt.Printf("[VECTOR_SEARCH] Filtering out product %d (%s); missing tokens: %v\n",
					result.Product.ID, result.Product.PostTitle, missing)
			}
		}

		if len(filteredResults) > 0 {
			results = filteredResults
			fmt.Printf("[VECTOR_SEARCH] %d products remain after token filtering\n", len(results))
		} else {
			fmt.Printf("[VECTOR_SEARCH] Token filtering removed all products, keeping similarity results\n")
			fallbackToSimilarity = true
		}
	}

	// Return top results
	if limit > 0 && limit < len(results) {
		fmt.Printf("[VECTOR_SEARCH] Limiting results to top %d (from %d total)\n", limit, len(results))
		results = results[:limit]
	}

	fmt.Printf("[PRODUCT_EMBEDDINGS] ✅ PRODUCT EMBEDDINGS query complete - Returning %d products (fallback=%t)\n", len(results), fallbackToSimilarity)
	return results, fallbackToSimilarity, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func (es *EmbeddingService) cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (es *EmbeddingService) requiredTokensFromQuery(query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}

	tokens := utils.ExtractMeaningfulTokens(query)
	if len(tokens) == 0 {
		return nil
	}

	required := make([]string, 0, len(tokens))
	seen := make(map[string]struct{})

	for _, token := range tokens {
		_, isKnownTagToken := es.tagTokenSet[token]
		if !isKnownTagToken && !utils.TokenHasDigit(token) {
			continue
		}

		if _, alreadyAdded := seen[token]; alreadyAdded {
			continue
		}

		required = append(required, token)
		seen[token] = struct{}{}
	}

	return required
}

func buildProductTokenSet(product models.Product) map[string]struct{} {
	values := []string{product.PostTitle}

	if product.PostName != nil {
		values = append(values, *product.PostName)
	}
	if product.SKU != nil {
		values = append(values, *product.SKU)
	}
	if product.Tags != nil {
		values = append(values, *product.Tags)
	}
	if product.ShortDescription != nil {
		values = append(values, *product.ShortDescription)
	}
	if product.Description != nil {
		values = append(values, *product.Description)
	}

	return utils.BuildTokenSet(values...)
}

// CreateEmbeddingsTable creates the table for storing product embeddings
func (es *EmbeddingService) CreateEmbeddingsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS product_embeddings (
			product_id INT PRIMARY KEY,
			embedding JSON NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_product_id (product_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := es.db.ExecContext(ctx, query)
	return err
}
