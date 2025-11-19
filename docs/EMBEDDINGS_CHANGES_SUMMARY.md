# Embeddings Generation: Run Once vs Scheduled Mode

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

### Production (Scheduled Mode)
For Kubernetes/Docker deployments that should run continuously:

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
✅ **No breaking changes!**
- Default behavior is unchanged (scheduled mode)
- Existing production deployments continue working
- Production command stays the same: `./bin/init-embeddings-write`

---

## 🚀 Production Safety

### ✅ Correct Production Usage
```yaml
# Kubernetes deployment - NO FLAGS
command: ["/app/bin/init-embeddings-write"]
```

### ❌ Wrong for Production
```yaml
# Would exit after first run!
command: ["/app/bin/init-embeddings-write", "--once"]
```

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
- Unchanged behavior (backwards compatible)
- Automatic continuous regeneration
- Handles failures gracefully

---

**Date:** November 19, 2025  
**Impact:** Local development workflow improvement  
**Breaking Changes:** None

