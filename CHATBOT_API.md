# Chatbot API Documentation

This document describes the new chatbot functionality added to the IDS API.

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
      "row_key": "user_1",
      "message": "Hello, I am looking for products in your store."
    },
    {
      "row_key": "assistant_1",
      "message": "Hello! I would be happy to help you find products."
    },
    {
      "row_key": "user_2", 
      "message": "What electronics do you have available?"
    }
  ]
}
```

### Request Fields

- `conversation` (array, required): Array of conversation messages
  - `row_key` (string, required): Identifier for the message (used to determine if it's from user or assistant)
  - `message` (string, required): The actual message content

### Role Detection

The system automatically determines if a message is from a user or assistant based on the `row_key`:
- Messages with `row_key` containing "assistant", "bot", or "ai" are treated as assistant messages
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
```

### Required Dependencies

The chatbot functionality requires:
- OpenAI API key (get one from https://platform.openai.com/)
- Database connection (for product context)

## How It Works

1. **Context Building**: The system fetches sample product data from your database to provide context to the AI
2. **Message Processing**: Conversation messages are converted to OpenAI's chat format
3. **AI Processing**: The conversation is sent to OpenAI's GPT-4o model with product context
4. **Response**: The AI's response is returned to the client

## Example Usage

### Using curl

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "row_key": "user_1",
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

## Rate Limits

- OpenAI API calls are subject to OpenAI's rate limits
- Database queries are limited to 50 products for context
- Request timeout is set to 30 seconds

## Security Notes

- Keep your OpenAI API key secure and never commit it to version control
- The API key is loaded from environment variables
- Consider implementing additional authentication for production use
