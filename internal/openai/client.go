// Package openai provides a unified client for OpenAI API access
// with support for both Azure OpenAI (primary) and OpenAI platform (fallback)
package openai

import (
	"context"
	"fmt"
	"time"

	"ids/internal/config"

	"github.com/sashabaranov/go-openai"
)

// Client wraps OpenAI client with Azure OpenAI support and fallback capability
type Client struct {
	primary      *openai.Client
	fallback     *openai.Client
	cfg          *config.Config
	useAzure     bool
	gptModel     string
	embedModel   openai.EmbeddingModel
	providerName string
}

// NewClient creates a new OpenAI client with Azure as primary and OpenAI as fallback
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		cfg: cfg,
	}

	// Try Azure OpenAI first (primary)
	if cfg.UseAzureOpenAI() {
		azureConfig := openai.DefaultAzureConfig(cfg.AzureOpenAIKey, cfg.AzureOpenAIEndpoint)
		client.primary = openai.NewClientWithConfig(azureConfig)
		client.useAzure = true
		client.gptModel = cfg.AzureOpenAIGPTDeployment
		client.embedModel = openai.EmbeddingModel(cfg.AzureOpenAIEmbeddingDeployment)
		client.providerName = "Azure OpenAI"

		fmt.Printf("[OPENAI_CLIENT] Primary provider: Azure OpenAI (endpoint: %s)\n", cfg.AzureOpenAIEndpoint)
	}

	// Setup OpenAI as fallback (or primary if Azure not configured)
	if cfg.HasOpenAIFallback() {
		client.fallback = openai.NewClient(cfg.OpenAIKey)

		if !client.useAzure {
			// Use OpenAI as primary since Azure is not configured
			client.primary = client.fallback
			client.fallback = nil
			client.gptModel = string(openai.GPT4oMini)
			client.embedModel = openai.SmallEmbedding3
			client.providerName = "OpenAI"

			fmt.Printf("[OPENAI_CLIENT] Primary provider: OpenAI (Azure not configured)\n")
		} else {
			fmt.Printf("[OPENAI_CLIENT] Fallback provider: OpenAI\n")
		}
	}

	if client.primary == nil {
		return nil, fmt.Errorf("no OpenAI provider configured: set AZURE_OPENAI_ENDPOINT + AZURE_OPENAI_KEY or OPENAI_API_KEY")
	}

	return client, nil
}

// TestConnection verifies the API connection works
func (c *Client) TestConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := c.CreateEmbeddings(ctx, []string{"test"})
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", c.providerName, err)
	}

	fmt.Printf("[OPENAI_CLIENT] Connection test successful (%s)\n", c.providerName)
	return nil
}

// CreateEmbeddings generates embeddings for the given texts
func (c *Client) CreateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.primary.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: c.embedModel,
	})

	if err != nil && c.fallback != nil {
		// Try fallback provider
		fmt.Printf("[OPENAI_CLIENT] Primary failed, trying fallback: %v\n", err)
		resp, err = c.fallback.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: texts,
			Model: openai.SmallEmbedding3,
		})
		if err != nil {
			return nil, fmt.Errorf("both providers failed: %v", err)
		}
		fmt.Printf("[OPENAI_CLIENT] Fallback succeeded\n")
	} else if err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// CreateChatCompletion generates a chat completion
func (c *Client) CreateChatCompletion(ctx context.Context, messages []openai.ChatCompletionMessage, maxTokens int, temperature float32) (*openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:       c.gptModel,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	resp, err := c.primary.CreateChatCompletion(ctx, req)
	if err != nil && c.fallback != nil {
		// Try fallback provider with OpenAI model name
		fmt.Printf("[OPENAI_CLIENT] Primary chat failed, trying fallback: %v\n", err)
		req.Model = string(openai.GPT4oMini)
		resp, err = c.fallback.CreateChatCompletion(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("both providers failed: %v", err)
		}
		fmt.Printf("[OPENAI_CLIENT] Fallback chat succeeded\n")
	} else if err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetProviderName returns the current primary provider name
func (c *Client) GetProviderName() string {
	return c.providerName
}

// IsUsingAzure returns true if Azure OpenAI is the primary provider
func (c *Client) IsUsingAzure() bool {
	return c.useAzure
}

// GetGPTModel returns the GPT model/deployment name being used
func (c *Client) GetGPTModel() string {
	return c.gptModel
}

// GetEmbeddingModel returns the embedding model/deployment name being used
func (c *Client) GetEmbeddingModel() string {
	return string(c.embedModel)
}
