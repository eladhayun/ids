# Azure Blob Storage Email Import System

Complete guide for the Azure Blob Storage + Kubernetes Job email import workflow.

## Overview

This system enables automated email import from Azure Blob Storage by:
1. Storing email files (`.eml` and `.mbox`) in Azure Blob Storage
2. Triggering Kubernetes Jobs via API to process the emails
3. Downloading, parsing, and importing emails into the database with vector embeddings

## Architecture

```
┌─────────────────┐     ┌────────────────┐     ┌───────────────┐
│  Upload Emails  │────▶│ Azure Blob     │     │ Kubernetes    │
│  (.eml/.mbox)   │     │ Storage        │────▶│ Job (Import)  │
└─────────────────┘     └────────────────┘     └───────────────┘
                              │                        │
                              │                        ▼
                         ┌────▼──────────┐     ┌──────────────┐
                         │ Backend API   │     │  MariaDB     │
                         │ /api/admin/   │     │  Database    │
                         │ trigger-      │     │  + Vectors   │
                         │ email-import  │     └──────────────┘
                         └───────────────┘
```

## Setup Instructions

### 1. Deploy Azure Blob Storage with Terraform

```bash
cd /Users/elad/Development/jshipster/terraform

# Review the configuration
terraform plan

# Apply the Terraform configuration
terraform apply

# Get the storage credentials
terraform output storage_account_name
terraform output -raw storage_primary_access_key
```

**Important Terraform Resources Created:**
- **Storage Account**: `prodstorage1234` (generic for all workloads)
- **Blob Containers**:
  - `email-imports` - for email files
  - `application-data` - for general app data
  - `backups` - for database backups
  - `exports` - for data exports
- **IAM Roles**: AKS has `Storage Blob Data Contributor` access

### 2. Upload Storage Credentials to Kubernetes

```bash
# Navigate to gitops repo
cd /Users/elad/Development/jshipster/gitops/email-import-job

# Edit secrets.yaml with actual values from Terraform
vim secrets.yaml

# Update with:
# storage-account-name: "prodstorage1234"
# storage-account-key: "<from terraform output>"

# Encrypt the secret with SOPS (if using)
sops -e -i secrets.yaml

# Commit to Git
cd /Users/elad/Development/jshipster/gitops
git add email-import-job/
git commit -m "Add email import job infrastructure"
git push

# ArgoCD will sync automatically
# Or manually sync:
argocd app sync apps/email-import-job
```

### 3. Upload Email Files to Azure Blob Storage

#### Option A: Using Azure CLI

```bash
# Login to Azure
az login

# Upload a single file
az storage blob upload \
  --account-name prodstorage1234 \
  --container-name email-imports \
  --name "my-emails.mbox" \
  --file /path/to/local/emails.mbox

# Upload a directory of EML files
az storage blob upload-batch \
  --account-name prodstorage1234 \
  --source /path/to/eml/directory \
  --destination email-imports \
  --pattern "*.eml"
```

#### Option B: Using AzCopy (for large files)

```bash
# Download azcopy
brew install azcopy

# Get SAS token from Azure Portal or CLI
az storage container generate-sas \
  --account-name prodstorage1234 \
  --name email-imports \
  --permissions racwdl \
  --expiry 2025-12-31T23:59:59Z \
  --auth-mode key

# Upload with azcopy
azcopy copy "/path/to/emails/*" \
  "https://prodstorage1234.blob.core.windows.net/email-imports?<SAS_TOKEN>" \
  --recursive
```

#### Option C: Using Azure Portal

1. Navigate to: https://portal.azure.com
2. Go to Storage Accounts → `prodstorage1234`
3. Click "Containers" → `email-imports`
4. Click "Upload" and select your files

### 4. Trigger Email Import via API

#### Using curl:

```bash
# Trigger the import job
curl -X POST http://your-backend-url/api/admin/trigger-email-import \
  -H "Content-Type: application/json" \
  -d '{}'

# Response:
# {
#   "success": true,
#   "message": "Email import job triggered successfully",
#   "job_name": "email-import-1701267890"
# }
```

#### Using Postman:

1. **POST** `http://your-backend-url/api/admin/trigger-email-import`
2. **Headers**: `Content-Type: application/json`
3. **Body** (optional): `{ "source": "subfolder/" }`

### 5. Monitor Job Status

```bash
# Get job status
curl http://your-backend-url/api/admin/email-import-status/email-import-1701267890

# Response:
# {
#   "job_name": "email-import-1701267890",
#   "status": "running",
#   "active": 1,
#   "succeeded": 0,
#   "failed": 0,
#   "start_time": "2025-11-29T10:00:00Z"
# }
```

#### Using kubectl:

```bash
# List all email import jobs
kubectl get jobs -l app=email-import --context=jshipster

# Get job details
kubectl describe job email-import-1701267890 --context=jshipster

# Get pod logs
kubectl logs -l job-name=email-import-1701267890 --context=jshipster

# Watch job progress
kubectl get pods -l job-name=email-import-1701267890 -w --context=jshipster
```

## How It Works

### Job Execution Flow

1. **API Trigger**: Backend receives POST request to `/api/admin/trigger-email-import`
2. **Job Creation**: Kubernetes client creates a Job with unique name
3. **Init Container**: Downloads all `.eml` and `.mbox` files from Azure Blob Storage
4. **Main Container**: Runs `import-emails` binary to:
   - Parse email files
   - Extract metadata and content
   - Detect conversation threads
   - Generate OpenAI embeddings
   - Upsert to database (no duplicates)
5. **Cleanup**: Job auto-deletes after 24 hours

### Job Components

**Init Container (`download-emails`)**:
- Image: `mcr.microsoft.com/azure-cli:latest`
- Downloads files from Azure Blob Storage using `az storage blob download-batch`
- Stores files in shared volume `/emails`

**Main Container (`import-emails`)**:
- Image: `prodacr1234.azurecr.io/ids-backend:latest`
- Executes `/app/bin/import-emails` binary
- Processes both EML and MBOX files
- Generates embeddings with OpenAI
- Upserts to database (uses `ON DUPLICATE KEY UPDATE`)

## Database Operations

The import process performs **UPSERT** operations:
- Inserts new emails
- Updates existing emails if `message_id` already exists
- Updates thread metadata
- Regenerates embeddings if content changes

**Tables Updated:**
- `emails` - Individual email messages
- `email_threads` - Conversation threads
- `email_embeddings` - Vector embeddings for emails
- `thread_embeddings` - Vector embeddings for threads

## Troubleshooting

### Check Job Status

```bash
# Get all jobs
kubectl get jobs --context=jshipster

# Get job description
kubectl describe job email-import-TIMESTAMP --context=jshipster

# Get pod logs
kubectl logs -l job-name=email-import-TIMESTAMP --context=jshipster --all-containers

# Check pod events
kubectl get events --context=jshipster --sort-by='.lastTimestamp' | grep email-import
```

### Common Issues

#### 1. Job Fails with "Cannot download blobs"

**Cause**: Invalid Azure storage credentials

**Fix**:
```bash
# Verify secret exists
kubectl get secret azure-storage-secret --context=jshipster -o yaml

# Update secret
kubectl delete secret azure-storage-secret --context=jshipster
kubectl create secret generic azure-storage-secret \
  --from-literal=storage-account-name=prodstorage1234 \
  --from-literal=storage-account-key=YOUR_KEY \
  --context=jshipster
```

#### 2. Job Fails with "No such file or directory"

**Cause**: No files in Azure Blob Storage

**Fix**: Upload files to the `email-imports` container

#### 3. Job Fails with "Database connection error"

**Cause**: Invalid database credentials or database not accessible

**Fix**:
```bash
# Check database secret
kubectl get secret ids-secrets --context=jshipster -o yaml

# Test database connection from a pod
kubectl run -it --rm debug --image=mariadb:latest --context=jshipster -- \
  mariadb -h YOUR_DB_HOST -u YOUR_USER -p
```

#### 4. Job Never Completes

**Cause**: Large MBOX file or many EML files

**Solution**: Monitor progress with logs:
```bash
kubectl logs -f -l job-name=email-import-TIMESTAMP --context=jshipster
```

### Delete Failed Jobs

```bash
# Delete a specific job
kubectl delete job email-import-TIMESTAMP --context=jshipster

# Delete all email import jobs
kubectl delete jobs -l app=email-import --context=jshipster
```

## Resource Limits

**Job Resources:**
- **Memory**: 512Mi (request) → 2Gi (limit)
- **CPU**: 250m (request) → 1000m (limit)
- **Storage**: 5Gi (emptyDir volume)
- **TTL**: 86400 seconds (24 hours after completion)

**Adjust for large imports:**

Edit `gitops/email-import-job/job.yaml`:

```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "4Gi"
    cpu: "2000m"
```

## Security Considerations

1. **Secrets Management**: Use SOPS or Azure Key Vault for secret encryption
2. **Network Policies**: Restrict job access to only database and Azure endpoints
3. **RBAC**: Job uses dedicated ServiceAccount `email-import-sa` with minimal permissions
4. **Blob Access**: Use Managed Identity instead of storage keys (future enhancement)

## Cost Optimization

1. **Storage Lifecycle**: Azure automatically archives old blobs after 90 days
2. **Job Cleanup**: Jobs auto-delete after 24 hours
3. **Spot Nodes**: Run jobs on spot/preemptible nodes for cost savings

## API Reference

### POST /api/admin/trigger-email-import

**Request:**
```json
{
  "source": "optional/subfolder/"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Email import job triggered successfully",
  "job_name": "email-import-1701267890"
}
```

### GET /api/admin/email-import-status/:jobName

**Response:**
```json
{
  "job_name": "email-import-1701267890",
  "status": "completed",
  "active": 0,
  "succeeded": 1,
  "failed": 0,
  "start_time": "2025-11-29T10:00:00Z",
  "completion_time": "2025-11-29T10:15:00Z"
}
```

**Status Values:**
- `pending` - Job created but not started
- `running` - Job is currently executing
- `completed` - Job finished successfully
- `failed` - Job encountered an error

## Next Steps

1. **Set up authentication** for admin endpoints
2. **Add webhook notifications** for job completion
3. **Implement resume capability** for interrupted imports
4. **Add progress tracking** with status updates
5. **Support incremental imports** (only new files)

## Related Documentation

- [Email Import Guide](./EMAIL_IMPORT_GUIDE.md) - Local email import
- [Embeddings Quick Start](./EMBEDDINGS_QUICK_START.md) - Vector embeddings system
- [Kubernetes Operations](../gitops/README.md) - GitOps deployment
- [Terraform Infrastructure](../terraform/README.md) - Infrastructure as Code

