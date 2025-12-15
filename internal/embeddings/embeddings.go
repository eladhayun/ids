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
	idsopenai "ids/internal/openai"
	"ids/internal/utils"

	"github.com/jmoiron/sqlx"
)

// EmbeddingService handles vector embeddings for products
type EmbeddingService struct {
	client      *idsopenai.Client     // Unified client with Azure/OpenAI fallback
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
	// Create unified client with Azure OpenAI (primary) and OpenAI (fallback)
	client, err := idsopenai.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %v", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx); err != nil {
		return nil, err
	}

	fmt.Printf("[EMBEDDING_SERVICE] Using %s for embeddings (model: %s)\n",
		client.GetProviderName(), client.GetEmbeddingModel())

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
// processBatchCommon is a shared helper for processing batches of products
func processBatchCommon(
	products []models.Product,
	client *idsopenai.Client,
	buildText func(models.Product) string,
	storeEmbedding func(models.Product, []float64) error,
	logPrefix string,
) error {
	fmt.Printf("[%s] Processing batch of %d products\n", logPrefix, len(products))

	// Prepare texts for embedding
	fmt.Printf("[%s] Building product texts...\n", logPrefix)
	texts := make([]string, len(products))
	for i, product := range products {
		texts[i] = buildText(product)
	}

	// Generate embeddings using unified client (Azure/OpenAI with fallback)
	fmt.Printf("[%s] Sending batch to %s API...\n", logPrefix, client.GetProviderName())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	embeddings, err := client.CreateEmbeddings(ctx, texts)
	if err != nil {
		fmt.Printf("[%s] ERROR: Failed to generate embeddings: %v\n", logPrefix, err)
		return fmt.Errorf("failed to generate embeddings: %v", err)
	}

	fmt.Printf("[%s] Received %d embeddings from %s\n", logPrefix, len(embeddings), client.GetProviderName())

	// Store embeddings in database
	fmt.Printf("[%s] Storing embeddings in database...\n", logPrefix)
	for i, embeddingData := range embeddings {
		product := products[i]
		// Convert []float32 to []float64
		embedding := make([]float64, len(embeddingData))
		for j, v := range embeddingData {
			embedding[j] = float64(v)
		}
		if err := storeEmbedding(product, embedding); err != nil {
			fmt.Printf("[%s] ERROR: Failed to store embedding for product %d: %v\n", logPrefix, product.ID, err)
			return fmt.Errorf("failed to store embedding for product %d: %v", product.ID, err)
		}
	}

	fmt.Printf("[%s] Successfully stored %d embeddings\n", logPrefix, len(embeddings))
	return nil
}

func (es *EmbeddingService) processBatch(products []models.Product) error {
	return processBatchCommon(
		products,
		es.client,
		es.buildProductText,
		es.storeEmbedding,
		"EMBEDDING_GEN",
	)
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
		desc := cleanHTMLDescription(*product.Description)
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

	// Generate embedding for the query using unified client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("[VECTOR_SEARCH] Generating query embedding via %s...\n", es.client.GetProviderName())
	embeddings, err := es.client.CreateEmbeddings(ctx, []string{query})
	if err != nil {
		fmt.Printf("[VECTOR_SEARCH] ERROR: Failed to generate query embedding: %v\n", err)
		return nil, false, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	fmt.Printf("[VECTOR_SEARCH] Query embedding generated successfully (dimensions: %d)\n", len(embeddings[0]))

	// Convert []float32 to []float64
	queryEmbedding := make([]float64, len(embeddings[0]))
	for j, v := range embeddings[0] {
		queryEmbedding[j] = float64(v)
	}

	// Get all product embeddings from PostgreSQL (no MariaDB joins)
	// Product metadata is stored denormalized in the product_embeddings table
	fmt.Printf("[PRODUCT_EMBEDDINGS] Fetching product embeddings from PostgreSQL (no MariaDB access)...\n")
	if es.writeClient == nil {
		return nil, false, fmt.Errorf("PostgreSQL write client not available for product embeddings search")
	}
	rows, err := es.writeClient.GetDB().QueryContext(ctx, queryProductEmbeddings)
	if err != nil {
		fmt.Printf("[PRODUCT_EMBEDDINGS] ❌ ERROR: Failed to fetch product embeddings from PostgreSQL: %v\n", err)
		return nil, false, fmt.Errorf("failed to fetch product embeddings: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			fmt.Printf("Warning: Error closing rows: %v\n", err)
		}
	}()

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

		if err != nil {
			skippedCount++
			continue // Skip invalid rows
		}

		// Convert nullable fields to pointers
		product = convertNullableFieldsToProduct(product, postName, description, shortDescription, sku, minPrice, maxPrice, stockStatus, tags, stockQuantity)

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
	sortBySimilarity(results)

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
	fallbackToSimilarity := applyTokenFiltering(&results, requiredTokens, es.tagTokenSet)

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

// applyTokenFiltering applies token-based filtering to results
func applyTokenFiltering(results *[]ProductEmbedding, requiredTokens []string, tagTokenSet map[string]struct{}) bool {
	if len(requiredTokens) == 0 {
		return false
	}

	fmt.Printf("[VECTOR_SEARCH] Applying exact-match filtering with tokens: %v\n", requiredTokens)

	var filteredResults []ProductEmbedding
	for _, result := range *results {
		productTokenSet := buildProductTokenSet(result.Product)
		if ok, missing := utils.ContainsAllTokens(productTokenSet, requiredTokens); ok {
			filteredResults = append(filteredResults, result)
		} else {
			fmt.Printf("[VECTOR_SEARCH] Filtering out product %d (%s); missing tokens: %v\n",
				result.Product.ID, result.Product.PostTitle, missing)
		}
	}

	if len(filteredResults) > 0 {
		*results = filteredResults
		fmt.Printf("[VECTOR_SEARCH] %d products remain after token filtering\n", len(*results))
		return false
	}

	fmt.Printf("[VECTOR_SEARCH] Token filtering removed all products, keeping similarity results\n")
	return true
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
