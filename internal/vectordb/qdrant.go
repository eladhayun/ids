package vectordb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

const (
	// Collection names
	ProductsCollection     = "products"
	EmailThreadsCollection = "email_threads"

	// Vector dimensions for text-embedding-3-small
	VectorDimensions = 1536
)

// QdrantClient wraps the Qdrant client with IDS-specific functionality
type QdrantClient struct {
	client *qdrant.Client
	url    string
}

// ProductPayload contains product metadata stored in Qdrant
type ProductPayload struct {
	ProductID        int    `json:"product_id"`
	PostTitle        string `json:"post_title"`
	PostName         string `json:"post_name"`
	SKU              string `json:"sku"`
	StockStatus      string `json:"stock_status"`
	MinPrice         string `json:"min_price"`
	MaxPrice         string `json:"max_price"`
	Tags             string `json:"tags"`
	Description      string `json:"description"`
	ShortDescription string `json:"short_description"`
}

// EmailPayload contains email thread metadata stored in Qdrant
type EmailPayload struct {
	ThreadID   string `json:"thread_id"`
	Subject    string `json:"subject"`
	EmailCount int    `json:"email_count"`
	FirstDate  string `json:"first_date"`
	LastDate   string `json:"last_date"`
}

// ProductSearchResult represents a product search result from Qdrant
type ProductSearchResult struct {
	ProductID  int
	Payload    ProductPayload
	Similarity float32
}

// EmailSearchResult represents an email search result from Qdrant
type EmailSearchResult struct {
	ThreadID   string
	Payload    EmailPayload
	Similarity float32
}

// NewQdrantClient creates a new Qdrant client
// url can be in format "hostname" or "hostname:port" (port is extracted if present, otherwise uses 6334)
func NewQdrantClient(url string) (*QdrantClient, error) {
	// Extract hostname and port from URL
	hostname := url
	port := 6334 // Default gRPC port

	if idx := strings.Index(url, ":"); idx > 0 {
		hostname = url[:idx]
		// Extract port if present (though we always use 6334 for gRPC)
		portStr := url[idx+1:]
		if parsedPort, err := strconv.Atoi(portStr); err == nil {
			port = parsedPort
		}
	}

	fmt.Printf("[QDRANT] Creating client with hostname: %s, port: %d\n", hostname, port)

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: hostname,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	return &QdrantClient{
		client: client,
		url:    url,
	}, nil
}

// Close closes the Qdrant client connection
func (q *QdrantClient) Close() error {
	return q.client.Close()
}

// HealthCheck checks if Qdrant is healthy
func (q *QdrantClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := q.client.HealthCheck(ctx)
	return err
}

// EnsureCollections creates the required collections if they don't exist
func (q *QdrantClient) EnsureCollections(ctx context.Context) error {
	// Create products collection
	if err := q.ensureCollection(ctx, ProductsCollection); err != nil {
		return fmt.Errorf("failed to create products collection: %w", err)
	}

	// Create email threads collection
	if err := q.ensureCollection(ctx, EmailThreadsCollection); err != nil {
		return fmt.Errorf("failed to create email_threads collection: %w", err)
	}

	return nil
}

func (q *QdrantClient) ensureCollection(ctx context.Context, name string) error {
	// Check if collection exists
	exists, err := q.client.CollectionExists(ctx, name)
	if err != nil {
		return err
	}

	if exists {
		fmt.Printf("[QDRANT] Collection '%s' already exists\n", name)
		return nil
	}

	// Create collection with cosine distance
	err = q.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     VectorDimensions,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return err
	}

	fmt.Printf("[QDRANT] Created collection '%s'\n", name)
	return nil
}

// UpsertProduct inserts or updates a product embedding in Qdrant
func (q *QdrantClient) UpsertProduct(ctx context.Context, productID int, embedding []float32, payload ProductPayload) error {
	points := []*qdrant.PointStruct{
		{
			Id:      qdrant.NewIDNum(uint64(productID)),
			Vectors: qdrant.NewVectors(embedding...),
			Payload: qdrant.NewValueMap(map[string]any{
				"product_id":        payload.ProductID,
				"post_title":        payload.PostTitle,
				"post_name":         payload.PostName,
				"sku":               payload.SKU,
				"stock_status":      payload.StockStatus,
				"min_price":         payload.MinPrice,
				"max_price":         payload.MaxPrice,
				"tags":              payload.Tags,
				"description":       payload.Description,
				"short_description": payload.ShortDescription,
			}),
		},
	}

	_, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: ProductsCollection,
		Points:         points,
	})
	return err
}

// UpsertEmailThread inserts or updates an email thread embedding in Qdrant
func (q *QdrantClient) UpsertEmailThread(ctx context.Context, threadID string, embedding []float32, payload EmailPayload) error {
	// Use hash of thread ID as numeric ID
	numericID := hashString(threadID)

	points := []*qdrant.PointStruct{
		{
			Id:      qdrant.NewIDNum(numericID),
			Vectors: qdrant.NewVectors(embedding...),
			Payload: qdrant.NewValueMap(map[string]any{
				"thread_id":   payload.ThreadID,
				"subject":     payload.Subject,
				"email_count": payload.EmailCount,
				"first_date":  payload.FirstDate,
				"last_date":   payload.LastDate,
			}),
		},
	}

	_, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: EmailThreadsCollection,
		Points:         points,
	})
	return err
}

// SearchProducts searches for similar products in Qdrant
//
//nolint:dupl // SearchProducts and SearchEmailThreads have similar structure but different types
func (q *QdrantClient) SearchProducts(ctx context.Context, embedding []float32, limit int) ([]ProductSearchResult, error) {
	results, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: ProductsCollection,
		Query:          qdrant.NewQuery(embedding...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}

	var searchResults []ProductSearchResult
	for _, point := range results {
		payload := extractProductPayload(point.Payload)
		searchResults = append(searchResults, ProductSearchResult{
			ProductID:  payload.ProductID,
			Payload:    payload,
			Similarity: point.Score,
		})
	}

	return searchResults, nil
}

// SearchEmailThreads searches for similar email threads in Qdrant
//
//nolint:dupl // SearchProducts and SearchEmailThreads have similar structure but different types
func (q *QdrantClient) SearchEmailThreads(ctx context.Context, embedding []float32, limit int) ([]EmailSearchResult, error) {
	results, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: EmailThreadsCollection,
		Query:          qdrant.NewQuery(embedding...),
		Limit:          qdrant.PtrOf(uint64(limit)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search email threads: %w", err)
	}

	var searchResults []EmailSearchResult
	for _, point := range results {
		payload := extractEmailPayload(point.Payload)
		searchResults = append(searchResults, EmailSearchResult{
			ThreadID:   payload.ThreadID,
			Payload:    payload,
			Similarity: point.Score,
		})
	}

	return searchResults, nil
}

// extractProductPayload extracts product payload from Qdrant response
func extractProductPayload(payload map[string]*qdrant.Value) ProductPayload {
	return ProductPayload{
		ProductID:        int(getIntValue(payload, "product_id")),
		PostTitle:        getStringValue(payload, "post_title"),
		PostName:         getStringValue(payload, "post_name"),
		SKU:              getStringValue(payload, "sku"),
		StockStatus:      getStringValue(payload, "stock_status"),
		MinPrice:         getStringValue(payload, "min_price"),
		MaxPrice:         getStringValue(payload, "max_price"),
		Tags:             getStringValue(payload, "tags"),
		Description:      getStringValue(payload, "description"),
		ShortDescription: getStringValue(payload, "short_description"),
	}
}

// extractEmailPayload extracts email payload from Qdrant response
func extractEmailPayload(payload map[string]*qdrant.Value) EmailPayload {
	return EmailPayload{
		ThreadID:   getStringValue(payload, "thread_id"),
		Subject:    getStringValue(payload, "subject"),
		EmailCount: int(getIntValue(payload, "email_count")),
		FirstDate:  getStringValue(payload, "first_date"),
		LastDate:   getStringValue(payload, "last_date"),
	}
}

// getStringValue safely extracts a string value from payload
func getStringValue(payload map[string]*qdrant.Value, key string) string {
	if v, ok := payload[key]; ok {
		if str := v.GetStringValue(); str != "" {
			return str
		}
	}
	return ""
}

// getIntValue safely extracts an integer value from payload
func getIntValue(payload map[string]*qdrant.Value, key string) int64 {
	if v, ok := payload[key]; ok {
		return v.GetIntegerValue()
	}
	return 0
}

// hashString creates a numeric hash from a string for Qdrant point IDs
func hashString(s string) uint64 {
	var hash uint64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint64(c)
	}
	return hash
}
