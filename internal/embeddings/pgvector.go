package embeddings

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"ids/internal/models"
)

// FormatVectorForPgvector converts a float64 slice to pgvector string format
// Example output: "[0.1,0.2,0.3]"
func FormatVectorForPgvector(embedding []float64) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// FormatFloat32VectorForPgvector converts a float32 slice to pgvector string format
func FormatFloat32VectorForPgvector(embedding []float32) string {
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ScanProductEmbeddingRow scans a row from pgvector query results into a ProductEmbedding
// Returns the product embedding and any error encountered
func ScanProductEmbeddingRow(rows *sql.Rows, logPrefix string) (*ProductEmbedding, error) {
	var productID int
	var embeddingStr string
	var product models.Product
	var similarity float64

	// Use sql.NullString for nullable fields
	var postName, description, shortDescription, sku, minPrice, maxPrice, stockStatus, tags sql.NullString
	var stockQuantity sql.NullFloat64

	err := rows.Scan(
		&productID,
		&embeddingStr,
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
		&similarity,
	)

	if err != nil {
		fmt.Printf("[%s] Warning: Failed to scan row: %v\n", logPrefix, err)
		return nil, err
	}

	// Convert nullable fields to pointers
	product = convertNullableFieldsToProduct(product, postName, description, shortDescription, sku, minPrice, maxPrice, stockStatus, tags, stockQuantity)

	product.ID = productID
	return &ProductEmbedding{
		Product:    product,
		Embedding:  nil, // Don't need to store embedding in results
		Similarity: similarity,
	}, nil
}

// ScanProductEmbeddingRows scans all rows from pgvector query results
func ScanProductEmbeddingRows(rows *sql.Rows, logPrefix string) []ProductEmbedding {
	var results []ProductEmbedding
	for rows.Next() {
		result, err := ScanProductEmbeddingRow(rows, logPrefix)
		if err != nil {
			continue // Skip invalid rows
		}
		results = append(results, *result)
	}
	return results
}
