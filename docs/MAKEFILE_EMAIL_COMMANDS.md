# Makefile Email Import Commands - Quick Reference

## Overview

The Makefile now includes convenient targets to import email conversations with a single command. These targets automatically build the import tool if needed and run the complete import process.

## Commands

### 1. `make import-emails` - Auto-Detect Format (Recommended)

**Best for:** Quick imports when you don't want to specify the format manually.

```bash
# Import from a directory of EML files
make import-emails EMAIL_PATH=/Users/elad/Downloads

# Import from a single EML file
make import-emails EMAIL_PATH=/path/to/email.eml

# Import from an MBOX file
make import-emails EMAIL_PATH=/path/to/mailbox.mbox
```

**What it does:**
- Automatically detects if the path is:
  - An MBOX file (`.mbox` extension)
  - An EML file (`.eml` extension)
  - A directory (scans for EML files)
- Builds the import tool if needed
- Runs the complete import process
- Creates database tables if they don't exist
- Generates vector embeddings

### 2. `make import-emails-eml` - Import EML Files

**Best for:** When you specifically want to import EML files.

```bash
# Import all EML files from a directory
make import-emails-eml PATH_TO_EMAILS=/Users/elad/Downloads

# Import a single EML file
make import-emails-eml PATH_TO_EMAILS=/path/to/email.eml
```

### 3. `make import-emails-mbox` - Import MBOX File

**Best for:** When you specifically want to import an MBOX mailbox file.

```bash
# Import an MBOX file (e.g., Gmail export)
make import-emails-mbox PATH_TO_MBOX=/path/to/gmail-export.mbox
```

## Complete Workflow

### Typical Usage Pattern

```bash
# 1. Initial import from Downloads folder
make import-emails EMAIL_PATH=/Users/elad/Downloads

# 2. Import Gmail export (MBOX format)
make import-emails EMAIL_PATH=/Users/elad/Downloads/gmail-takeout.mbox

# 3. Import Outlook exports (EML files)
make import-emails EMAIL_PATH=/Users/elad/Documents/outlook-emails

# 4. Import a specific customer conversation
make import-emails EMAIL_PATH=/Users/elad/Documents/important-customer.eml
```

## What Happens During Import

Each command performs these steps:

1. **Build Tool** (if needed)
   ```
   Building import-emails...
   Build complete: bin/import-emails
   ```

2. **Detect Format**
   ```
   Detected directory: /Users/elad/Downloads
   ```

3. **Parse Emails**
   ```
   Parsing EML from: /Users/elad/Downloads
   Successfully parsed 26 emails
   ```

4. **Store in Database**
   ```
   Storing emails in database...
   Stored 26 emails successfully (0 errors)
   ```

5. **Generate Embeddings**
   ```
   Generating embeddings for individual emails...
   [EMAIL_EMBEDDINGS] Found 26 emails to process
   [EMAIL_EMBEDDINGS] Processing batch 1-26...
   
   Generating embeddings for email threads...
   [THREAD_EMBEDDINGS] Found 26 threads to process
   ```

6. **Summary**
   ```
   ✓ Email import complete!
     - Parsed: 26 emails
     - Stored: 26 emails
     - Embeddings: Generated
   ```

## Examples

### Example 1: Weekly Email Import

```bash
# Create a weekly import script
cat > ~/weekly-email-import.sh << 'EOF'
#!/bin/bash
cd /Users/elad/Development/jshipster/ids
make import-emails EMAIL_PATH=/Users/elad/Downloads
EOF

chmod +x ~/weekly-email-import.sh

# Run weekly
~/weekly-email-import.sh
```

### Example 2: Import Multiple Sources

```bash
# Import from multiple directories in sequence
make import-emails EMAIL_PATH=/Users/elad/Downloads
make import-emails EMAIL_PATH=/Users/elad/Documents/customer-emails
make import-emails EMAIL_PATH=/Users/elad/Documents/support-archives
```

### Example 3: Import with Logging

```bash
# Save import log for review
make import-emails EMAIL_PATH=/Users/elad/Downloads 2>&1 | tee import-$(date +%Y%m%d).log
```

### Example 4: Bulk Import Script

```bash
# Import all MBOX files from a directory
for mbox in /Users/elad/email-archives/*.mbox; do
    echo "Importing: $mbox"
    make import-emails EMAIL_PATH="$mbox"
done
```

## Troubleshooting

### No PATH specified
```bash
make import-emails
# Error: EMAIL_PATH not specified
# Solution: Add EMAIL_PATH=/path/to/emails
```

### Path not found
```bash
make import-emails EMAIL_PATH=/nonexistent/path
# Error: /nonexistent/path is not a valid file or directory
# Solution: Check the path exists
```

### Build failures
```bash
# If build fails, try cleaning and rebuilding
make clean
make build-import-emails
```

### Database connection issues
```bash
# Check your .env file has correct DATABASE_URL
cat .env | grep DATABASE_URL
```

## Verification

After importing, verify the data:

```bash
# Check email count
mysql -h localhost -P 3306 -u isrealde_wp654 -p'isrealde_wp654' -D isrealde_wp654 \
  -e "SELECT COUNT(*) as total_emails FROM emails;"

# Check recent imports
mysql -h localhost -P 3306 -u isrealde_wp654 -p'isrealde_wp654' -D isrealde_wp654 \
  -e "SELECT id, subject, from_addr, date FROM emails ORDER BY created_at DESC LIMIT 10;"

# Check embeddings status
mysql -h localhost -P 3306 -u isrealde_wp654 -p'isrealde_wp654' -D isrealde_wp654 \
  -e "SELECT 
    COUNT(DISTINCT email_id) as email_embeddings,
    COUNT(DISTINCT thread_id) as thread_embeddings 
  FROM email_embeddings;"
```

## Performance Tips

### For Large Imports

1. **Import without embeddings first** (faster):
   ```bash
   ./bin/import-emails -eml /path/to/large-directory -embeddings=false
   ```

2. **Generate embeddings later**:
   ```bash
   # TODO: Need to create this target
   # make generate-email-embeddings
   ```

### For Incremental Updates

The import process uses `ON DUPLICATE KEY UPDATE`, so you can safely re-run imports:

```bash
# Re-import same directory - only new emails will be added
make import-emails EMAIL_PATH=/Users/elad/Downloads
```

Duplicate emails (same Message-ID) will be updated, not duplicated.

## Integration with CI/CD

### Automated Daily Import

```yaml
# .github/workflows/email-import.yml
name: Daily Email Import

on:
  schedule:
    - cron: '0 2 * * *'  # 2 AM daily

jobs:
  import:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Import emails
        run: make import-emails EMAIL_PATH=/mounted/email/path
```

## See Also

- [EMAIL_IMPORT_GUIDE.md](EMAIL_IMPORT_GUIDE.md) - Complete import guide
- [EMAIL_EXAMPLE.md](EMAIL_EXAMPLE.md) - Step-by-step examples
- [IMPLEMENTATION_COMPLETE.md](IMPLEMENTATION_COMPLETE.md) - System overview

## Quick Reference Card

```bash
# Most common usage
make import-emails EMAIL_PATH=/path/to/emails

# Force rebuild
make clean build-import-emails

# Import with logging
make import-emails EMAIL_PATH=/path 2>&1 | tee import.log

# Check help
make help | grep -A 10 "Email commands"
```

