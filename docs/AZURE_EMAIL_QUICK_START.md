# Azure Blob Email Import - Quick Start

Get up and running with Azure Blob Storage email imports in 5 steps.

## Prerequisites

- Azure CLI installed (`brew install azure-cli`)
- kubectl configured with jshipster context
- Access to the project's Azure subscription

## Quick Start

### 1. Deploy Infrastructure (One-time)

```bash
cd /Users/elad/Development/jshipster/terraform

# Deploy Azure Blob Storage
terraform init
terraform apply

# Get credentials
STORAGE_NAME=$(terraform output -raw storage_account_name)
STORAGE_KEY=$(terraform output -raw storage_primary_access_key)

echo "Storage Account: $STORAGE_NAME"
echo "Storage Key: $STORAGE_KEY"
```

### 2. Configure Kubernetes Secrets (One-time)

```bash
cd /Users/elad/Development/jshipster/gitops/email-import-job

# Edit secrets.yaml
vim secrets.yaml

# Update with your values:
# storage-account-name: "YOUR_STORAGE_NAME"
# storage-account-key: "YOUR_STORAGE_KEY"

# Commit and push
git add secrets.yaml
git commit -m "Configure Azure storage credentials"
git push

# ArgoCD will sync automatically
# Or manually: argocd app sync apps
```

### 3. Upload Email Files

```bash
# Login to Azure
az login

# Upload your email files
az storage blob upload-batch \
  --account-name $STORAGE_NAME \
  --source /path/to/your/emails \
  --destination email-imports \
  --pattern "*.eml"

# For MBOX files
az storage blob upload \
  --account-name $STORAGE_NAME \
  --container-name email-imports \
  --name "emails.mbox" \
  --file /path/to/emails.mbox
```

### 4. Trigger Import Job

```bash
# Using curl
curl -X POST http://your-backend-url/api/admin/trigger-email-import \
  -H "Content-Type: application/json"

# Or using httpie
http POST http://your-backend-url/api/admin/trigger-email-import
```

**Response:**
```json
{
  "success": true,
  "message": "Email import job triggered successfully",
  "job_name": "email-import-1701267890"
}
```

### 5. Monitor Progress

```bash
# Watch job status
kubectl get jobs -l app=email-import -w --context=jshipster

# Get pod logs
JOB_NAME="email-import-1701267890"
kubectl logs -l job-name=$JOB_NAME --context=jshipster -f

# Check API status
curl http://your-backend-url/api/admin/email-import-status/$JOB_NAME
```

## Common Use Cases

### Scenario 1: Import Gmail Export

```bash
# 1. Export Gmail as MBOX (via Google Takeout)
# 2. Upload to Azure
az storage blob upload \
  --account-name $STORAGE_NAME \
  --container-name email-imports \
  --name "gmail-export.mbox" \
  --file ~/Downloads/gmail-export.mbox

# 3. Trigger import
curl -X POST http://your-backend-url/api/admin/trigger-email-import
```

### Scenario 2: Import Outlook PST (converted to EML)

```bash
# 1. Convert PST to EML using a tool like readpst
readpst -e -o /tmp/outlook-export outlook.pst

# 2. Upload all EML files
az storage blob upload-batch \
  --account-name $STORAGE_NAME \
  --source /tmp/outlook-export \
  --destination email-imports \
  --pattern "*.eml"

# 3. Trigger import
curl -X POST http://your-backend-url/api/admin/trigger-email-import
```

### Scenario 3: Import Large Archive (70GB MBOX)

For very large files, use AzCopy for better performance:

```bash
# Install azcopy
brew install azcopy

# Generate SAS token (valid for 1 hour)
az storage container generate-sas \
  --account-name $STORAGE_NAME \
  --name email-imports \
  --permissions racw \
  --expiry $(date -u -d "1 hour" '+%Y-%m-%dT%H:%MZ') \
  --auth-mode key \
  --account-key $STORAGE_KEY

# Upload with azcopy (faster for large files)
azcopy copy \
  "/path/to/large-archive.mbox" \
  "https://$STORAGE_NAME.blob.core.windows.net/email-imports?<SAS_TOKEN>"

# Trigger import (may take hours for 70GB)
curl -X POST http://your-backend-url/api/admin/trigger-email-import
```

## Verification

### Check Database

```bash
# Connect to database
kubectl port-forward svc/mariadb 3306:3306 --context=jshipster

# In another terminal
mariadb -h localhost -P 3306 -u root -p'my-secret-pw' -D isrealde_wp654
```

```sql
-- Check email count
SELECT COUNT(*) FROM emails;

-- Check thread count
SELECT COUNT(*) FROM email_threads;

-- Check latest imports
SELECT message_id, subject, from_addr, date 
FROM emails 
ORDER BY created_at DESC 
LIMIT 10;

-- Check embeddings
SELECT COUNT(*) FROM email_embeddings;
SELECT COUNT(*) FROM thread_embeddings;
```

### Test Chat with Email Context

```bash
# Ask a question that should reference emails
curl -X POST http://your-backend-url/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What product issues did customers report?",
    "conversation_history": []
  }'
```

The response should include context from imported emails if relevant.

## Troubleshooting

### Job Stuck in Pending

```bash
# Check pod events
kubectl describe job $JOB_NAME --context=jshipster

# Common causes:
# - Insufficient cluster resources
# - Image pull errors
# - Secret not found
```

### Job Fails Immediately

```bash
# Get pod logs
kubectl logs -l job-name=$JOB_NAME --context=jshipster --all-containers

# Common causes:
# - Invalid Azure credentials
# - No files in blob storage
# - Database connection error
```

### Import Takes Too Long

For large imports:
1. Monitor progress: `kubectl logs -f -l job-name=$JOB_NAME --context=jshipster`
2. Expected time: ~1000 emails/minute (varies by size)
3. 70GB MBOX: Could take 6-12 hours

## Cleanup

### Delete Imported Files

```bash
# List files
az storage blob list \
  --account-name $STORAGE_NAME \
  --container-name email-imports \
  --output table

# Delete all files
az storage blob delete-batch \
  --account-name $STORAGE_NAME \
  --source email-imports
```

### Delete Failed Jobs

```bash
# Delete specific job
kubectl delete job $JOB_NAME --context=jshipster

# Delete all email import jobs
kubectl delete jobs -l app=email-import --context=jshipster
```

## Next Steps

- [Full Documentation](./AZURE_BLOB_EMAIL_IMPORT.md) - Complete guide with advanced features
- [Terraform Setup](../../terraform/STORAGE_SETUP.md) - Infrastructure details
- [Email Import Guide](./EMAIL_IMPORT_GUIDE.md) - Local import methods
- [Embeddings System](./EMBEDDINGS_QUICK_START.md) - Understanding vector search

## Support

- Check logs: `kubectl logs -l app=email-import --context=jshipster`
- View Swagger: http://your-backend-url/swagger/
- Database: Connect via `kubectl port-forward`

