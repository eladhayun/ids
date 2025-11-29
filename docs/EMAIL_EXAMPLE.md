# Email Import Example

This guide shows a complete workflow for importing email conversations and using them to enhance chat responses.

## Quick Start Example

### Step 1: Prepare Sample Email

Create a test email file `sample.eml`:

```
From: customer@example.com
To: support@israeldefensestore.com
Subject: Question about Glock 19 holsters
Date: Mon, 29 Nov 2024 10:00:00 -0500
Message-ID: <abc123@example.com>
Content-Type: text/plain; charset=UTF-8

Hi,

I'm looking for an OWB holster for my Glock 19. I prefer kydex material
and need it for right-hand carry. What do you recommend?

Thanks,
John
```

Create a response email `response.eml`:

```
From: support@israeldefensestore.com
To: customer@example.com
Subject: Re: Question about Glock 19 holsters
Date: Mon, 29 Nov 2024 11:30:00 -0500
Message-ID: <def456@example.com>
In-Reply-To: <abc123@example.com>
References: <abc123@example.com>
Content-Type: text/plain; charset=UTF-8

Hi John,

Great choice! For the Glock 19 OWB setup, I'd recommend our Bravo Concealment
OWB holster. It's made from premium Kydex, features:
- Adjustable retention
- Right-hand configuration
- Available in black or FDE
- Fits duty belt or competition belt

Price: $59.99
Stock: In stock

Would you like me to send you the product link?

Best regards,
Support Team
```

### Step 2: Import Emails

```bash
# Build the import tool
cd /Users/elad/Development/jshipster/ids
make build-import-emails

# Import the emails
./bin/import-emails -eml /path/to/email/directory
```

Output:
```
Parsing EML from: /path/to/email/directory
Scanning directory for EML files...
Successfully parsed 2 emails
Storing emails in database...
Stored 2 emails successfully (0 errors)

Generating embeddings for individual emails...
[EMAIL_EMBEDDINGS] Starting email embedding generation...
[EMAIL_EMBEDDINGS] Found 2 emails to process
[EMAIL_EMBEDDINGS] Processing batch 1-2...
[EMAIL_EMBEDDINGS] Email embedding generation complete

Generating embeddings for email threads...
[THREAD_EMBEDDINGS] Starting thread embedding generation...
[THREAD_EMBEDDINGS] Found 1 threads to process
[THREAD_EMBEDDINGS] Thread embedding generation complete

✓ Email import complete!
  - Parsed: 2 emails
  - Stored: 2 emails
  - Embeddings: Generated
```

### Step 3: Verify Import

Connect to database and check:

```bash
mariadb -h localhost -P 3306 -u isrealde_wp654 -p'isrealde_wp654' -D isrealde_wp654 --ssl=false
```

```sql
-- Check emails
SELECT id, subject, from_addr, is_customer FROM emails;
```

Output:
```
+----+-------------------------------+--------------------------------+-------------+
| id | subject                       | from_addr                      | is_customer |
+----+-------------------------------+--------------------------------+-------------+
|  1 | Question about Glock 19 hol.. | customer@example.com           |           1 |
|  2 | Re: Question about Glock 19.. | support@israeldefensestore.com |           0 |
+----+-------------------------------+--------------------------------+-------------+
```

```sql
-- Check threads
SELECT thread_id, subject, email_count FROM email_threads;
```

Output:
```
+--------------+-------------------------------+-------------+
| thread_id    | subject                       | email_count |
+--------------+-------------------------------+-------------+
| abc123@ex... | Question about Glock 19 hol.. |           2 |
+--------------+-------------------------------+-------------+
```

```sql
-- Check embeddings
SELECT 
    COUNT(CASE WHEN email_id IS NOT NULL THEN 1 END) as email_embeddings,
    COUNT(CASE WHEN thread_id IS NOT NULL THEN 1 END) as thread_embeddings
FROM email_embeddings;
```

Output:
```
+------------------+-------------------+
| email_embeddings | thread_embeddings |
+------------------+-------------------+
|                2 |                 1 |
+------------------+-------------------+
```

### Step 4: Test Enhanced Chat

Now test the chat endpoint with a similar query:

```bash
curl -X POST http://localhost:8080/api/chat/enhanced \
  -H "Content-Type: application/json" \
  -d '{
    "conversation": [
      {
        "role": "user",
        "message": "I need an OWB holster for Glock 19 in kydex, right hand"
      }
    ]
  }'
```

Expected response:
```json
{
  "response": "Based on your requirements for a Glock 19 OWB holster in Kydex for right-hand carry, I recommend:\n\n**Bravo Concealment OWB Holster**\n- Material: Premium Kydex construction\n- Configuration: Right-hand draw\n- Features: Adjustable retention, fits duty and competition belts\n- Colors: Black or FDE available\n- Price: $59.99\n- Stock: In Stock\n- Similarity: 0.87\n- URL: https://israeldefensestore.com/product/bravo-owb-holster\n\nThis holster is a popular choice for Glock 19 carriers. Would you like more information about this product?\n\n**Found 1 relevant products**",
  "products": {
    "Bravo Concealment OWB Holster": "bravo-owb-holster"
  }
}
```

Notice how the AI's response quality improves because it learned from the similar past conversation!

## Comparison: Before vs After Email Import

### Before (Standard Chat)

Request:
```json
{
  "conversation": [
    {"role": "user", "message": "holster for glock"}
  ]
}
```

Response:
```
Here are some holsters compatible with Glock:
- Holster A
- Holster B
- Holster C

Which Glock model do you have?
```

### After (Enhanced Chat with Email Context)

Request:
```json
{
  "conversation": [
    {"role": "user", "message": "holster for glock"}
  ]
}
```

Response:
```
I'd be happy to help you find a Glock holster! To recommend the best option, 
I need a few details:

1. Which Glock model do you have? (e.g., Glock 19, 17, 43X)
2. Do you prefer OWB (outside waistband) or IWB (inside waistband)?
3. Material preference? (Kydex, leather, hybrid)
4. Carry position? (Right-hand or left-hand)

Based on popular choices, many customers opt for Kydex OWB holsters for Glock 19 
in right-hand configuration. I have several highly-rated options in stock if 
that matches your needs!
```

The enhanced version provides better guidance because it learned from past successful interactions!

## Real-World Use Cases

### Use Case 1: Product Compatibility Questions

**Customer asks:** "Will the P365 holster fit the P365XL?"

**Without email context:** Generic answer about checking product specifications

**With email context:** Learned from past conversations:
- These are different models
- P365 holster won't fit P365XL properly
- Recommend specific P365XL holster
- Suggest contacting manufacturer if unsure

### Use Case 2: Sizing Questions for Apparel

**Customer asks:** "What size Dubon parka should I get?"

**Without email context:** "Check the size chart"

**With email context:** Learned from past conversations:
- Dubon parkas run slightly large
- Recommend sizing down if between sizes
- Mention specific fit characteristics (slim/regular/relaxed)
- Link to size chart with helpful notes

### Use Case 3: Shipping Times

**Customer asks:** "When will my order arrive?"

**Without email context:** Generic shipping policy

**With email context:** Learned from past conversations:
- Current shipping times for different regions
- Common delays to mention proactively
- Tracking information guidance
- Expedited options if needed

## Advanced: Bulk Import

For large email archives:

```bash
# Import MBOX file (e.g., exported from Gmail)
./bin/import-emails -mbox /path/to/gmail-export.mbox

# Import with progress tracking
./bin/import-emails -mbox /path/to/large-file.mbox | tee import.log

# Import without embeddings (faster initial import)
./bin/import-emails -mbox /path/to/huge-file.mbox -embeddings=false

# Generate embeddings later (if needed)
# Note: You'll need to extend init-embeddings-write to support email embeddings
```

## Monitoring Results

### Check similarity scores:

```sql
SELECT 
    e.subject,
    ee.similarity_score,
    e.date
FROM emails e
JOIN email_search_logs ee ON ee.email_id = e.id
WHERE ee.query_text LIKE '%holster%'
ORDER BY ee.similarity_score DESC
LIMIT 10;
```

### Most referenced threads:

```sql
SELECT 
    et.subject,
    COUNT(esl.id) as reference_count
FROM email_threads et
JOIN email_search_logs esl ON esl.thread_id = et.thread_id
GROUP BY et.thread_id, et.subject
ORDER BY reference_count DESC
LIMIT 10;
```

## Tips for Best Results

1. **Import diverse conversations**: Include various topics, products, issues
2. **Include resolutions**: Make sure threads include both questions AND answers
3. **Keep recent**: Periodically import new emails to stay current
4. **Clean old data**: Archive very old conversations that are no longer relevant
5. **Monitor quality**: Check which past conversations are being used
6. **Update embeddings**: Regenerate when product catalog changes significantly

## Troubleshooting

### Emails imported but not appearing in chat

Check embeddings:
```sql
SELECT COUNT(*) FROM email_embeddings WHERE thread_id IS NOT NULL;
```

If 0, regenerate embeddings:
```bash
./bin/import-emails -eml /path/to/emails -embeddings=true
```

### Poor similarity scores

- Check that email bodies have substantial content
- Verify embeddings were generated (check `email_embeddings` table)
- Try with more diverse training data
- Adjust similarity threshold in code

### Import errors

- Check file encoding (should be UTF-8)
- Verify email format (valid RFC 5322)
- Check database connection
- Look for specific error messages in output

## Next Steps

1. ✅ Import a small test batch (10-50 emails)
2. ✅ Test enhanced chat endpoint
3. ✅ Compare responses with standard chat
4. ✅ Import larger dataset
5. ✅ Monitor results and iterate
6. Consider implementing:
   - Email summary generation
   - Automatic categorization
   - Response templates based on past conversations
   - Analytics dashboard for most helpful past conversations

