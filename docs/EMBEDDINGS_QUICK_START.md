# Embeddings Quick Start Guide

## Current Production Architecture

As of 2026-09-01, IDS uses PostgreSQL with pgvector as its sole production vector store. Qdrant has been removed from the cluster and production configuration keeps `QDRANT_ENABLED=false` with no `QDRANT_URL`.

The data path is:

1. Read published and private WooCommerce products from the remote, read-only MariaDB database.
2. Generate 1536-dimensional vectors with the configured Azure OpenAI embedding deployment.
3. Incrementally upsert product metadata, checksums, and vectors into the `ids_embeddings` PostgreSQL database.
4. Search product, email, and thread vectors in PostgreSQL using pgvector cosine distance and HNSW indexes.

## Local Development: Run Once

`make run-embeddings` builds the tool, runs one embedding refresh, and exits:

```bash
make run-embeddings
```

The equivalent direct command is:

```bash
make build-embeddings
./bin/init-embeddings-write --once
```

Use this after embedding-logic changes, when validating search relevance, or when product vectors need an immediate refresh.

## Production: Kubernetes CronJob

Production does not keep the embedding generator running continuously. The companion GitOps repository defines the `ids-init-embeddings` CronJob, scheduled daily at `00:00 UTC` with `concurrencyPolicy: Forbid`.

Each job invokes the application image once:

```yaml
command:
  - /bin/sh
  - -c
  - /home/appuser/init-embeddings-write --once
```

The one-shot command is the intended Kubernetes behavior: the Job succeeds and exits, and Kubernetes starts the next run from the CronJob schedule.

The binary still supports its built-in continuous scheduler when run without `--once`, but that mode is not used by the production GitOps deployment.

## Required Configuration

Use secret values appropriate to the environment; never commit real credentials.

```bash
# Read-only MariaDB/WooCommerce product source
DATABASE_URL=mysql://username:password@localhost:3306/database_name

# PostgreSQL application and vector store
EMBEDDINGS_DATABASE_URL=postgres://username:password@localhost:5432/ids_embeddings?sslmode=disable

# Azure OpenAI
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com/
AZURE_OPENAI_KEY=your_azure_openai_key_here
AZURE_OPENAI_GPT_DEPLOYMENT=gpt-4o-mini
AZURE_OPENAI_EMBEDDING_DEPLOYMENT=text-embedding-3-small

# Enable when DATABASE_URL is reached through the SSH sidecar/tunnel
WAIT_FOR_TUNNEL=false

# PostgreSQL-only production mode
QDRANT_ENABLED=false
```

Do not set `QDRANT_URL`. Restoring Qdrant requires a separately reviewed application and infrastructure change; it is not a fallback for normal operation.

## What a Refresh Does

- Enables the PostgreSQL `vector` extension when necessary.
- Creates the product embedding/checksum tables and HNSW index when necessary.
- Reads all eligible products from MariaDB.
- Compares product checksums and regenerates only new or changed product vectors.
- Stores vectors as PostgreSQL `vector(1536)` values with denormalized search metadata.
- Leaves email and thread vectors in the same PostgreSQL database; those are populated by the email import workflow.

## Verification

For a local/manual run, confirm the command exits successfully and logs PostgreSQL/pgvector activity without Qdrant initialization or connection messages.

For Kubernetes, use the `jshipster` context and verify the latest CronJob-created Job:

```bash
kubectl --context jshipster -n ids get cronjob ids-init-embeddings
kubectl --context jshipster -n ids get jobs --sort-by=.metadata.creationTimestamp
kubectl --context jshipster -n ids logs job/<job-name> -c init-embeddings
```

A no-change run is successful when it reads the product catalog, reports zero products needing regeneration, and exits with status 0.

## Troubleshooting

### MariaDB connection fails

- Verify `DATABASE_URL` and read-only credentials.
- If the database is reached through SSH, verify the tunnel and set `WAIT_FOR_TUNNEL=true`.

### PostgreSQL or pgvector setup fails

- Verify `EMBEDDINGS_DATABASE_URL` points to the IDS PostgreSQL database.
- Confirm the role can create/use the `vector` extension, tables, and indexes and can upsert rows.
- Check PostgreSQL capacity and PVC health in the cluster.

### Azure OpenAI fails

- Verify `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_KEY`, and the embedding deployment name.
- Check quota and request-timeout errors before retrying a full refresh.

### Qdrant appears in logs

- Production should have `QDRANT_ENABLED=false` and no `QDRANT_URL`.
- Treat an attempted Qdrant connection as configuration drift and correct the GitOps configuration.

---

**Created:** November 19, 2025

**Updated:** September 1, 2026

**Purpose:** Operate IDS embeddings with PostgreSQL/pgvector as the sole production vector store
