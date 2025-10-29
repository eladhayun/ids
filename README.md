# IDS API Documentation

This document describes the IDS API server with product management and AI chat functionality.

## API Documentation

The API includes comprehensive Swagger/OpenAPI documentation that can be accessed at:

**Swagger UI**: `http://localhost:8080/swagger/`

### Available Endpoints

- **GET** `/` - Root endpoint with service information
- **GET** `/healthz` - Basic health check
- **GET** `/healthz/db` - Database health check
- **GET** `/products` - Get paginated list of products
- **POST** `/chat` - Chat with AI assistant
- **GET** `/swagger/` - Swagger UI documentation

### Generating Documentation

To regenerate the Swagger documentation:

```bash
# Install swag tool (if not already installed)
make install-tools

# Generate documentation
make swagger
```

Or use the script directly:

```bash
./scripts/generate-swagger.sh
```

### Docker Build with Swagger

The Dockerfile automatically generates Swagger documentation during the build process:

```bash
# Build Docker image (Swagger docs generated automatically)
make docker-build

# Or build with pre-generated Swagger docs
make docker-build-with-swagger

# Or use Docker directly
docker build -t ids-api .
```

The Docker build process:
1. Installs the `swag` tool
2. Generates fresh Swagger documentation
3. Builds the application with the generated docs
4. Creates a production-ready image

## Chatbot API Documentation

This section describes the new chatbot functionality added to the IDS API.

## Overview

The chatbot API endpoint allows you to send conversation data to an AI assistant that has access to your product database. The assistant can answer questions about products, help with recommendations, and provide information based on the available product data.

## Endpoint

**POST** `/chat`

## Request Format

The endpoint accepts a JSON payload with the following structure:

```json
{
  "conversation": [
    {
      "role": "user",
      "message": "Hello, I am looking for products in your store."
    },
    {
      "role": "assistant",
      "message": "Hello! I would be happy to help you find products."
    },
    {
      "role": "user", 
      "message": "What electronics do you have available?"
    }
  ]
}
```

### Request Fields

- `conversation` (array, required): Array of conversation messages
  - `role` (string, required): Role of the message sender ("user" or "assistant")
  - `message` (string, required): The actual message content

### Role Detection

The system automatically determines if a message is from a user or assistant based on the `role` field:
- Messages with `role` containing "assistant", "bot", or "ai" are treated as assistant messages
- All other messages are treated as user messages

## Response Format

```json
{
  "response": "Based on our product database, I can see we have several electronics available...",
  "error": "Optional error message if something went wrong"
}
```

### Response Fields

- `response` (string): The AI assistant's response to the conversation
- `error` (string, optional): Error message if the request failed

## Configuration

### Environment Variables

Add the following to your `.env` file:

```env
OPENAI_API_KEY=your_openai_api_key_here
PRODUCT_CACHE_TTL=10
```

### Required Dependencies

The chatbot functionality requires:
- OpenAI API key (get one from https://platform.openai.com/)
- Database connection (for product context)

## How It Works

1. **Context Building**: The system fetches sample product data from your database to provide context to the AI
2. **Caching**: Product data is cached in memory for the configured TTL (default 10 minutes) to improve performance
3. **Message Processing**: Conversation messages are converted to OpenAI's chat format
4. **AI Processing**: The conversation is sent to OpenAI's GPT-4o model with product context
5. **Response**: The AI's response is returned to the client

## Example Usage

### Using curl

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "role": "user",
        "message": "What products do you have under $50?"
      }
    ]
  }'
```

### Using the test script

```bash
./test_chat_endpoint.sh
```

## Error Handling

The API handles various error conditions:

- **503 Service Unavailable**: Database connection not available
- **500 Internal Server Error**: OpenAI API key not configured or API error
- **400 Bad Request**: Invalid request body or empty conversation

## Product Context

The AI assistant has access to product information including:
- Product ID, title, and descriptions
- SKU and pricing information
- Stock status and quantity
- Product tags and categories

This context helps the assistant provide accurate and relevant responses about your products.

## Performance & Caching

- **Product Data Caching**: Product context data is cached in memory for improved performance
- **Configurable TTL**: Cache duration is configurable via `PRODUCT_CACHE_TTL` (default: 10 minutes)
- **Cache Scope**: Only applies to the product context query, not the main products endpoint
- **Automatic Expiration**: Cached data automatically expires and is refreshed

## Rate Limits

- OpenAI API calls are subject to OpenAI's rate limits
- Database queries are limited to 50 products for context
- Request timeout is set to 30 seconds

## Security Notes

- Keep your OpenAI API key secure and never commit it to version control
- The API key is loaded from environment variables
- Consider implementing additional authentication for production use
