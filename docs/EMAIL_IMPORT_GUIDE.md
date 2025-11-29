# Email Import and Context Enhancement Guide

## Overview

This system allows you to import email conversations (from EML and MBOX files) into the database and use them to enhance chat responses. The AI can learn from past customer interactions to provide better, more contextual answers.

## Features

- **Parse EML files**: Individual email files
- **Parse MBOX files**: Large mailbox archives
- **Thread detection**: Automatically groups related emails into conversations
- **Vector embeddings**: Generate semantic embeddings for smart search
- **Context enhancement**: Chat responses enhanced with similar past conversations
- **Dual search**: Searches both products AND relevant email history

## Architecture

```
Email Files (EML/MBOX)
        ↓
   Email Parser
        ↓
    Database (emails, email_threads, email_embeddings)
        ↓
  Vector Embeddings
        ↓
Chat Handler → Searches similar products + similar conversations
```

## Database Schema

### emails table
- Stores individual email messages
- Fields: message_id, subject, from_addr, to_addr, date, body, thread_id, is_customer
- Indexed by: message_id, thread_id, date

### email_threads table
- Groups related emails into conversations
- Fields: thread_id, subject, email_count, first_date, last_date, summary
- Tracks conversation metadata

### email_embeddings table
- Stores vector embeddings for emails and threads
- Linked to either individual emails or complete threads
- Used for similarity search

## Installation

### 1. Build the Import Tool

```bash
make build-import-emails
```

This creates the `bin/import-emails` binary.

### 2. Prepare Your Email Files

Supported formats:
- **EML**: Individual email files (`.eml` extension)
- **MBOX**: Mailbox archives (common email export format)

## Usage

### Import Single EML File

```bash
./bin/import-emails -eml /path/to/email.eml
```

### Import Directory of EML Files

```bash
./bin/import-emails -eml /path/to/emails/directory
```

The tool will recursively scan the directory for all `.eml` files.

### Import MBOX File

```bash
./bin/import-emails -mbox /path/to/mailbox.mbox
```

### Import Without Generating Embeddings

If you want to import first and generate embeddings later:

```bash
./bin/import-emails -eml /path/to/emails -embeddings=false
```

Then generate embeddings separately using the init-embeddings-write tool (to be extended).

## What Happens During Import

1. **Parse Emails**: Extracts metadata (subject, from, to, date) and body text
2. **Thread Detection**: Groups related emails using Message-ID, In-Reply-To, and References headers
3. **Store in Database**: Saves emails and creates/updates thread records
4. **Generate Embeddings**: Creates vector embeddings for semantic search
   - Individual email embeddings
   - Thread-level embeddings (for complete conversations)

## Using Enhanced Chat

### API Endpoint

The system provides two chat endpoints:

1. **Standard Chat** (existing): `/api/chat`
   - Searches only products

2. **Enhanced Chat** (new): `/api/chat/enhanced`
   - Searches products AND similar email conversations
   - Uses past conversations to provide better context

### Example Request

```bash
curl -X POST http://localhost:8080/api/chat/enhanced \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "role": "user",
        "message": "I need a holster for my Glock 19"
      }
    ]
  }'
```

### How It Works

1. **User Query**: Customer sends a question
2. **Product Search**: Finds relevant products using vector similarity
3. **Email Search**: Finds similar past conversations (threads)
4. **Context Building**: Combines product data with insights from past conversations
5. **AI Response**: GPT generates response with full context
6. **Result**: More accurate, contextual answer that learns from past interactions

## Benefits

### For Customers
- **Better Answers**: AI learns from past successful interactions
- **Consistency**: Similar questions get consistent, proven answers
- **Faster Resolution**: Common issues resolved using known solutions

### For Business
- **Knowledge Capture**: Institutional knowledge from email support preserved
- **Training Data**: Past conversations help train better responses
- **Pattern Recognition**: Identify common issues and improve products/services

## Email Format Tips

### EML Files
- Exported from most email clients (Outlook, Thunderbird, Apple Mail)
- One file per email
- Preserves all metadata and formatting

### MBOX Files
- Common export format for entire mailboxes
- All emails in a single file
- Format: Each email starts with "From " line
- Exported from Gmail, Thunderbird, etc.

### Getting Your Emails

**Gmail Export:**
1. Go to Google Takeout: https://takeout.google.com
2. Select "Mail"
3. Choose MBOX format
4. Download and extract

**Outlook:**
1. File → Open & Export → Import/Export
2. Export to a file → Personal Storage Table (.pst)
3. Use a converter tool to convert PST to MBOX or EML

**Thunderbird:**
1. Install ImportExportTools NG addon
2. Right-click folder → ImportExportTools NG → Export
3. Choose MBOX or EML format

## Advanced Usage

### Filtering Emails

You can modify the parser to filter emails:

```go
// Only import customer emails (not internal)
if !email.IsCustomer {
    continue
}

// Only import recent emails
if email.Date.Before(time.Now().AddDate(0, -6, 0)) {
    continue // Skip emails older than 6 months
}
```

### Custom Thread Detection

The system uses standard email threading (Message-ID, References, In-Reply-To). To customize:

```go
// Modify GenerateThreadID in parser.go
func GenerateThreadID(email *models.Email) string {
    // Your custom logic here
}
```

### Similarity Thresholds

Adjust similarity thresholds in `email_embeddings.go`:

```go
// Only use emails with high similarity
if similarity < 0.75 {
    continue
}
```

## Monitoring

### Check Import Status

```sql
-- Count imported emails
SELECT COUNT(*) FROM emails;

-- Count threads
SELECT COUNT(*) FROM email_threads;

-- Check embedding status
SELECT 
    COUNT(DISTINCT email_id) as individual_embeddings,
    COUNT(DISTINCT thread_id) as thread_embeddings
FROM email_embeddings;

-- View recent threads
SELECT thread_id, subject, email_count, first_date, last_date
FROM email_threads
ORDER BY last_date DESC
LIMIT 10;
```

### Performance

- **Small dataset** (<1000 emails): Import in seconds
- **Medium dataset** (1000-10000 emails): 5-15 minutes
- **Large dataset** (>10000 emails): Process in batches

OpenAI API rate limits apply:
- Standard: 3000 requests/minute
- Batch size: 50 emails per request (adjustable)

## Troubleshooting

### "Failed to parse email"
- Check file encoding (should be UTF-8 or ASCII)
- Try with `-embeddings=false` to separate import from embedding generation
- Some corrupted emails may fail - tool continues with others

### "OpenAI API error"
- Check API key in `.env` file: `OPENAI_API_KEY=sk-...`
- Verify API quota and rate limits
- Reduce batch size if hitting rate limits

### "Database connection failed"
- Check `DATABASE_URL` in `.env`
- Ensure MariaDB is running
- Verify credentials

### Empty email body
- Some emails may have only HTML content
- Parser attempts to extract text from HTML
- Check body field in database to verify content

## Privacy & Security

**Important Considerations:**

- **Customer Privacy**: Email import stores customer messages. Ensure compliance with:
  - GDPR (if serving EU customers)
  - CCPA (if serving California customers)
  - Your own privacy policy

- **Data Retention**: Consider implementing:
  ```sql
  -- Delete old emails
  DELETE FROM emails WHERE date < DATE_SUB(NOW(), INTERVAL 1 YEAR);
  ```

- **Sensitive Information**: Review imported emails for:
  - Credit card numbers
  - Social security numbers
  - Passwords or credentials
  - Personal health information

- **Access Control**: Limit database access to authorized personnel only

## Next Steps

1. **Import your first batch**: Start with a small set (100-500 emails)
2. **Test enhanced chat**: Try the `/api/chat/enhanced` endpoint
3. **Evaluate results**: Check if responses improve with email context
4. **Scale up**: Import more historical emails
5. **Monitor usage**: Track which past conversations are most helpful

## Support

For issues or questions:
- Check logs: Look for `[EMAIL_EMBEDDINGS]` and `[CHAT_ENHANCED]` tags
- Review database: Query the tables to verify data
- Test individually: Use `-embeddings=false` to isolate issues

