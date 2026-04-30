// Package openai provides a unified client for Azure OpenAI access
package openai

import (
	"context"
	"fmt"
	"time"

	"ids/internal/config"

	"github.com/sashabaranov/go-openai"
)

// Client wraps the Azure OpenAI client
type Client struct {
	client       *openai.Client
	cfg          *config.Config
	gptModel     string
	embedModel   openai.EmbeddingModel
	providerName string
}

// NewClient creates a new Azure OpenAI client
func NewClient(cfg *config.Config) (*Client, error) {
	if !cfg.UseAzureOpenAI() {
		return nil, fmt.Errorf("azure OpenAI not configured: set AZURE_OPENAI_ENDPOINT + AZURE_OPENAI_KEY")
	}

	azureConfig := openai.DefaultAzureConfig(cfg.AzureOpenAIKey, cfg.AzureOpenAIEndpoint)
	client := &Client{
		client:       openai.NewClientWithConfig(azureConfig),
		cfg:          cfg,
		gptModel:     cfg.AzureOpenAIGPTDeployment,
		embedModel:   openai.EmbeddingModel(cfg.AzureOpenAIEmbeddingDeployment),
		providerName: "Azure OpenAI",
	}

	fmt.Printf("[OPENAI_CLIENT] Provider: Azure OpenAI (endpoint: %s)\n", cfg.AzureOpenAIEndpoint)
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
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: c.embedModel,
	})
	if err != nil {
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
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.gptModel,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetProviderName returns the provider name
func (c *Client) GetProviderName() string {
	return c.providerName
}

// GetGPTModel returns the GPT deployment name
func (c *Client) GetGPTModel() string {
	return c.gptModel
}

// GetEmbeddingModel returns the embedding deployment name
func (c *Client) GetEmbeddingModel() string {
	return string(c.embedModel)
}
