# Embeddings Generation: Run Once vs Scheduled Mode

> **Current production status (2026-09-01):** The production `ids-init-embeddings` Kubernetes CronJob runs `/home/appuser/init-embeddings-write --once` daily at `00:00 UTC`. PostgreSQL/pgvector is the sole vector store, and Qdrant has been removed. The continuous mode documented below remains a binary capability but is not the current production deployment model.

## Summary of Changes

Added a `--once` flag to the embeddings generation tool to support both local development and production use cases.

---

## 🎯 How to Use

### Local Development (Run Once)
Perfect for when you're testing changes on your dev machine:

```bash
make run-embeddings
```

**Behavior:**
- Runs embeddings generation for all products
- **Exits cleanly when done** (no Ctrl+C needed!)
- Takes ~30-60 minutes
- Uses the `--once` flag automatically

---

### Standalone Continuous Mode
For a standalone process that deliberately owns its own schedule:

```bash
./bin/init-embeddings-write
```

**Behavior:**
- Runs initial embeddings generation
- **Stays running** and regenerates on schedule (e.g., weekly)
- Continues even if a generation fails
- Default behavior (no flags needed)

---

## 🔧 Technical Details

### Command-Line Flag Added
```
--once    Run embeddings generation once and exit (default: false)
```

### Code Changes

**File:** `cmd/init-embeddings-write/main.go`
- Added `flag` package import
- Added `--once` boolean flag
- After initial generation, checks flag:
  - If `--once=true`: exits cleanly
  - If `--once=false` (default): enters scheduler loop

**File:** `Makefile`
- Updated `run-embeddings` target to use `--once` flag
- This makes local dev convenient without breaking production

### Backwards Compatibility

The binary default remains scheduled mode, so existing standalone users are compatible. Kubernetes production intentionally overrides that default with `--once` because the CronJob owns the schedule.

---

## 🚀 Production Safety

### Current Kubernetes Production Usage
```yaml
# Kubernetes CronJob: run once, then let Kubernetes schedule the next Job
command: ["/home/appuser/init-embeddings-write", "--once"]
```

The no-flag command is appropriate only for a long-running standalone Deployment/container that is meant to use the binary's internal scheduler.

---

## 📋 Testing Done

- ✅ Code formatted with `go fmt`
- ✅ No linting errors with `go vet`
- ✅ Builds successfully
- ✅ Flag help text works: `./bin/init-embeddings-write --help`
- ✅ Makefile command works with proper flag

---

## 🎯 Benefits

### For Developers:
- No more Ctrl+C after embeddings complete
- Clean exit with proper error codes
- Faster local testing workflow

### For Production:
- One execution per Kubernetes CronJob-created Job
- Daily scheduling and overlap prevention managed by Kubernetes
- PostgreSQL/pgvector reads and writes only
- No Qdrant service, volume, or runtime dependency

---

**Date:** November 19, 2025

**Current production note added:** September 1, 2026

**Impact:** Local development workflow improvement and current Kubernetes operating guidance

**Breaking Changes:** None
