package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"ids/internal/config"
	"ids/internal/models"

	"github.com/jmoiron/sqlx"
	"github.com/sashabaranov/go-openai"
)

// EmbeddingService handles vector embeddings for products
type EmbeddingService struct {
	client *openai.Client
	db     *sqlx.DB
}

// ProductEmbedding represents a product with its vector embedding
type ProductEmbedding struct {
	Product    models.Product `json:"product"`
	Embedding  []float64      `json:"embedding"`
	Similarity float64        `json:"similarity,omitempty"`
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(cfg *config.Config, db *sqlx.DB) (*EmbeddingService, error) {
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

	return &EmbeddingService{
		client: client,
		db:     db,
	}, nil
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
func (es *EmbeddingService) SearchSimilarProducts(query string, limit int) ([]ProductEmbedding, error) {
	fmt.Printf("[VECTOR_SEARCH] Starting search for query: '%s' with limit: %d\n", query, limit)

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
		return nil, fmt.Errorf("failed to generate query embedding: %v", err)
	}

	fmt.Printf("[VECTOR_SEARCH] Query embedding generated successfully (dimensions: %d)\n", len(resp.Data[0].Embedding))

	// Convert []float32 to []float64
	queryEmbedding := make([]float64, len(resp.Data[0].Embedding))
	for j, v := range resp.Data[0].Embedding {
		queryEmbedding[j] = float64(v)
	}

	// Get all product embeddings from database
	embeddingsQuery := `
		SELECT pe.product_id, pe.embedding, p.post_title, p.post_name, p.post_content, p.post_excerpt,
			l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
		FROM product_embeddings pe
		JOIN wpjr_posts p ON p.ID = pe.product_id
		JOIN wpjr_wc_product_meta_lookup l ON l.product_id = p.ID
		WHERE p.post_type = 'product'
			AND p.post_status IN ('publish','private')
	`

	fmt.Printf("[VECTOR_SEARCH] Fetching product embeddings from database...\n")
	rows, err := es.db.QueryContext(ctx, embeddingsQuery)
	if err != nil {
		fmt.Printf("[VECTOR_SEARCH] ERROR: Failed to fetch product embeddings: %v\n", err)
		return nil, fmt.Errorf("failed to fetch product embeddings: %v", err)
	}
	defer rows.Close()

	var results []ProductEmbedding
	processedCount := 0
	skippedCount := 0

	for rows.Next() {
		var productID int
		var embeddingJSON string
		var product models.Product

		err := rows.Scan(
			&productID,
			&embeddingJSON,
			&product.PostTitle,
			&product.PostName,
			&product.Description,
			&product.ShortDescription,
			&product.SKU,
			&product.MinPrice,
			&product.MaxPrice,
			&product.StockStatus,
			&product.StockQuantity,
		)
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

	// Return top results
	if limit > 0 && limit < len(results) {
		fmt.Printf("[VECTOR_SEARCH] Limiting results to top %d (from %d total)\n", limit, len(results))
		results = results[:limit]
	}

	fmt.Printf("[VECTOR_SEARCH] Returning %d products\n", len(results))
	return results, nil
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
