# IDS API Documentation

This document describes the IDS API server with product management and AI-powered chat functionality using vector embeddings.

## Overview

IDS (Israel Defense Store API) is a tactical gear e-commerce API server that provides:
- Vector-based semantic product search using OpenAI embeddings
- AI-powered chat assistant for product recommendations
- Health monitoring endpoints
- Swagger/OpenAPI documentation
- Static web interface for customer support

## API Documentation

The API includes comprehensive Swagger/OpenAPI documentation that can be accessed at:

**Swagger UI**: `http://localhost:8080/swagger/index.html`

### Available Endpoints

- **GET** `/` - Static UI (Tactical Support Assistant chatbot interface)
- **GET** `/swagger/` - Swagger UI documentation

#### API Endpoints (under `/api` prefix)

- **GET** `/api/` - Root API endpoint with service information
- **GET** `/api/healthz` - Basic health check
- **GET** `/api/healthz/db` - Database health check
- **POST** `/api/chat` - Chat with AI assistant (uses vector search when embeddings are available)

## Quick Start

### Prerequisites

- Go 1.25.0 or higher
- MariaDB/MySQL database
- OpenAI API key (for chat functionality)

### Setup

1. Clone the repository
2. Copy `.env.example` to `.env` and configure:
   ```bash
   cp .env.example .env
   ```

3. Update the `.env` file with your configuration:
   ```env
   DATABASE_URL=mysql://username:password@localhost:3306/database_name
   OPENAI_API_KEY=your_openai_api_key_here
   PORT=8080
   ```

4. Build and run:
   ```bash
   # Build the server
   make build
   
   # Run the server
   make run
   
   # Or run in development mode
   make dev
   ```

5. Initialize embeddings (required for vector search):
   ```bash
   # Build the embeddings initialization tool
   make build-embeddings
   
   # Run it to generate embeddings for all products
   ./bin/init-embeddings-write
   ```

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

## Vector Search & Embeddings

The IDS API uses OpenAI's text embeddings to provide intelligent, semantic product search. This enables the AI assistant to understand user queries and recommend the most relevant products based on meaning rather than just keywords.

### How It Works

1. **Embedding Generation**: Product information (title, description, tags, prices) is converted into vector embeddings using OpenAI's `text-embedding-3-small` model
2. **Storage**: Embeddings are stored in the `product_embeddings` table in MariaDB
3. **Semantic Search**: User queries are converted to embeddings and compared using cosine similarity
4. **Smart Ranking**: Products are ranked by similarity score, with in-stock items prioritized
5. **Context-Aware Responses**: The top 15 most relevant products are provided to the AI for generating responses

### Embeddings Command

The `init-embeddings-write` command generates and stores embeddings for all products in the database:

```bash
# Build the embeddings tool
make build-embeddings

# Run embedding generation
./bin/init-embeddings-write
```

This command:
- Creates the `product_embeddings` table if it doesn't exist
- Fetches all published and private products from the database
- Generates embeddings in batches of 100 products
- Stores embeddings in JSON format
- Can be run on-demand or scheduled (includes built-in daily execution mode)

**Note**: The command requires write access to the database and will create the necessary tables automatically.

## Chatbot API Documentation

### Overview

The chatbot API endpoint provides an AI-powered assistant that uses vector embeddings to find and recommend tactical gear products. The assistant understands natural language queries and provides contextual product recommendations based on semantic similarity.

### Endpoint

**POST** `/api/chat`

### Chat Modes

The server automatically selects the appropriate chat handler:
- **Vector Search Mode** (default): Uses embeddings for semantic product search when the `product_embeddings` table exists and contains data
- **Basic Mode** (fallback): Provides general tactical gear information without specific product recommendations

### Request Format

The endpoint accepts a JSON payload with the following structure:

```json
{
  "conversation": [
    {
      "role": "user",
      "message": "Hello, I am looking for holsters for Glock 19."
    },
    {
      "role": "assistant",
      "message": "Hello! I would be happy to help you find holsters."
    },
    {
      "role": "user", 
      "message": "What do you have for right-handed use?"
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

### Response Format

```json
{
  "response": "Based on our product database, I can see we have several holsters available...\n\n**Found 5 relevant products** (showing top matches with similarity scores):",
  "products": {
    "Fobus Standard Holster": "fobus-standard-holster",
    "ORPAZ Defense Glock Holster": "orpaz-defense-glock-19-holster"
  },
  "error": "Optional error message if something went wrong"
}
```

### Response Fields

- `response` (string): The AI assistant's response with product recommendations
- `products` (object, optional): Map of product names to their slugs/SKUs for creating product links
- `error` (string, optional): Error message if the request failed

## Configuration

### Environment Variables

The application supports the following environment variables in your `.env` file:

```env
# Database Configuration
DATABASE_URL=mysql://username:password@localhost:3306/database_name

# Server Configuration
PORT=8080
VERSION=1.0.0
LOG_LEVEL=info

# OpenAI Configuration
OPENAI_API_KEY=your_openai_api_key_here
OPENAI_TIMEOUT=60  # API timeout in seconds (default: 60)

# Tunnel Configuration (for SSH tunnel scenarios)
WAIT_FOR_TUNNEL=false  # Wait for SSH tunnel before connecting to database
```

### Required Dependencies

The chatbot functionality requires:
- OpenAI API key (get one from https://platform.openai.com/)
- Database connection (for product data and embeddings)
- Product embeddings table (created by `init-embeddings-write` command)

## How It Works

### Vector Search Mode (When Embeddings Available)

1. **Query Extraction**: The last user message is extracted from the conversation
2. **Vector Search**: The query is converted to an embedding and compared against all product embeddings using cosine similarity
3. **Filtering**: Products are filtered to prioritize in-stock items
4. **Top Products**: The top 20 most similar products are retrieved
5. **Context Building**: The top 15 products are formatted with names, prices, stock status, and similarity scores
6. **AI Processing**: The conversation is sent to OpenAI's `gpt-4o-mini` model with product context
7. **Response**: The AI generates recommendations based on the most relevant products
8. **Product Metadata**: Product slugs/SKUs are returned for frontend linking

### Basic Mode (Fallback)

1. **General Context**: Basic store information is provided to the AI
2. **Message Processing**: Conversation messages are converted to OpenAI's chat format
3. **AI Processing**: The conversation is sent to OpenAI's `gpt-4o-mini` model
4. **Response**: The AI provides general tactical gear guidance without specific product recommendations

## Example Usage

### Using the Web Interface

1. Start the server: `make run`
2. Open your browser to `http://localhost:8080`
3. Use the Tactical Support Assistant chatbot interface to interact with the AI

### Using curl

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "role": "user",
        "message": "I need a holster for my Glock 19, right-handed, under $50"
      }
    ]
  }'
```

Example response with vector search:
```json
{
  "response": "I found several holsters that match your requirements:\n\n**Fobus Standard Holster** - $34.99 - In Stock - Similarity: 0.87\n**ORPAZ Defense Glock 19** - $42.50 - In Stock - Similarity: 0.85\n\n**Found 5 relevant products** (showing top matches with similarity scores):",
  "products": {
    "Fobus Standard Holster": "fobus-standard-holster",
    "ORPAZ Defense Glock 19": "orpaz-defense-glock-19-holster"
  }
}
```

## Error Handling

The API handles various error conditions:

- **503 Service Unavailable**: Database connection not available
- **500 Internal Server Error**: OpenAI API key not configured or API error
- **400 Bad Request**: Invalid request body, empty conversation, or no user message found

## Product Context

When using vector search mode, the AI assistant has access to:
- Product titles and descriptions
- SKU and pricing information
- Stock status and quantity
- Product tags and categories
- Semantic similarity scores

The assistant uses the top 15 most relevant products (out of 20 retrieved) to provide accurate recommendations.

## Performance Considerations

### Vector Search Mode
- **Embedding Generation**: Initial setup requires generating embeddings for all products
- **Search Performance**: Vector similarity calculation is performed in-memory for all products
- **Result Ranking**: Products are sorted by cosine similarity and filtered by stock status
- **Context Limits**: Top 15 products are included in AI context to avoid token limits

### API Performance
- **OpenAI Timeout**: Configurable via `OPENAI_TIMEOUT` (default: 60 seconds)
- **Retry Logic**: Basic chat mode includes exponential backoff retry (3 attempts)
- **Token Limits**: Responses limited to 1000-2000 tokens depending on mode
- **Temperature**: Set to 0.7 for balanced creativity and accuracy

## Rate Limits

- OpenAI API calls are subject to OpenAI's rate limits
- Vector search queries all products but limits results to top 20
- Request timeout configurable via `OPENAI_TIMEOUT` environment variable
- Batch embedding generation uses 100 products per batch

## Database Schema

### product_embeddings Table

The embeddings table is automatically created by the `init-embeddings-write` command:

```sql
CREATE TABLE product_embeddings (
    product_id INT PRIMARY KEY,
    embedding JSON NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_product_id (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
```

- **product_id**: Foreign key to wpjr_posts.ID
- **embedding**: JSON array of 1536 float values (OpenAI text-embedding-3-small dimensions)
- **created_at**: Timestamp of when the embedding was first created
- **updated_at**: Timestamp of the last update

## Development Commands

```bash
# Build the server
make build

# Run in development mode (auto-reload)
make dev

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Lint code
make lint

# Generate Swagger docs
make swagger

# Build embeddings tool
make build-embeddings

# Format, lint, and build embeddings tool
make embeddings

# Database management (Docker)
make db-start    # Start MariaDB container
make db-stop     # Stop MariaDB container
make db-restart  # Restart MariaDB container
make db-status   # Check container status
make db-logs     # View container logs
```

## Multi-Language Support

The application includes language detection capabilities for the following languages:
- English (en)
- Hebrew (עברית / he)
- Arabic (العربية / ar)
- Russian (Русский / ru)
- Chinese (中文 / zh)
- Japanese (日本語 / ja)
- Korean (한국어 / ko)

**Note**: Currently, the chat handlers are configured to always respond in English. Language detection is implemented in the `internal/utils/language.go` module but not yet integrated into the active chat flow. Future updates can enable automatic language detection from user queries.

## Limitations and Future Enhancements

### Current Limitations
1. **Language Detection**: Language detection is implemented but not currently active in chat handlers
2. **In-Memory Vector Search**: All embeddings are loaded into memory for similarity calculation
3. **No Pagination**: Vector search returns top N results without pagination support
4. **Basic Retry Logic**: Only basic chat mode has retry logic with exponential backoff

### Potential Enhancements
1. **Vector Database**: Use a dedicated vector database (e.g., Pinecone, Weaviate) for scalable similarity search
2. **Incremental Updates**: Support updating individual product embeddings without regenerating all
3. **Multi-Language Chat**: Enable automatic language detection and response in the user's language
4. **Advanced Filtering**: Add filters for price range, categories, and other product attributes
5. **Hybrid Search**: Combine vector similarity with traditional keyword search
6. **Caching**: Add caching for frequent queries to reduce OpenAI API calls

## Troubleshooting

### Chat Endpoint Returns General Information Only

**Problem**: The chat endpoint provides general tactical gear information without specific product recommendations.

**Solution**: 
1. Ensure the `product_embeddings` table exists in your database
2. Run the embeddings initialization command: `./bin/init-embeddings-write`
3. Verify embeddings were generated successfully in the database
4. Restart the server to initialize the embedding service

### Database Connection Failed

**Problem**: Server starts but shows "Database connection not available" warnings.

**Solution**:
1. Verify your `DATABASE_URL` in the `.env` file is correct
2. Ensure the database server is running
3. Check database credentials and network connectivity
4. If using SSH tunnel, ensure `WAIT_FOR_TUNNEL=true` is set

### OpenAI API Errors

**Problem**: Chat requests fail with OpenAI API errors.

**Solution**:
1. Verify your `OPENAI_API_KEY` is valid and active
2. Check your OpenAI account has sufficient credits/quota
3. Increase `OPENAI_TIMEOUT` if requests are timing out
4. Review OpenAI API status page for service disruptions

### Embeddings Generation Fails

**Problem**: `init-embeddings-write` command fails or times out.

**Solution**:
1. Ensure you have write access to the database
2. Verify the database connection string includes write permissions
3. Check that you have sufficient OpenAI API quota for batch processing
4. Review the console output for specific error messages
5. The command processes products in batches of 100 - partial failures may require manual cleanup

### Server Won't Start

**Problem**: Server fails to start or exits immediately.

**Solution**:
1. Check the `.env` file exists and has proper configuration
2. Ensure port 8080 (or configured port) is not already in use
3. Review server logs for specific error messages
4. Verify Go version is 1.25.0 or higher

## Security Notes

- Keep your OpenAI API key secure and never commit it to version control
- The API key is loaded from environment variables
- Database embeddings are stored in JSON format and can be large
- Consider implementing authentication for production use
- The API includes CORS configuration that allows all origins for development
- The `init-embeddings-write` command requires write access to the database
- Use environment variables for all sensitive configuration
- Review and restrict CORS settings before deploying to production

## License and Contact

For issues, questions, or contributions, please refer to the repository's issue tracker.
