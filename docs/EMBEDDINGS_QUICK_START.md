# Embeddings Quick Start Guide

## 🎯 Running Embeddings Generation

The embeddings system regenerates vector embeddings for all products to improve search relevance.

---

## 📝 Commands

### **Local Development: Run Once** (Recommended for Dev)
Regenerates embeddings for ALL products, then exits cleanly.

```bash
make run-embeddings
```

**What it does:**
- Loads environment variables from `.env`
- Builds the embeddings tool if needed
- Runs full embeddings generation for all products
- **Exits cleanly after completion** (no Ctrl+C needed)
- Takes ~30-60 minutes depending on product count

**When to run:**
- After code changes to embedding logic
- After deploying search improvements
- Testing embedding improvements locally
- When search relevance issues are reported

---

### **Production: Scheduled Mode** (For Kubernetes/Docker)
Runs embeddings generation continuously on a schedule.

```bash
./bin/init-embeddings-write
# OR with explicit flag:
./bin/init-embeddings-write --once=false
```

**What it does:**
- Runs initial embeddings generation
- **Stays running** and regenerates on schedule (e.g., weekly)
- Continues even if a generation fails
- Requires Ctrl+C or SIGTERM to stop

**When to use:**
- In production Kubernetes deployments
- In Docker containers with restart policies
- When you want automatic periodic regeneration

---

## 🔧 How It Works

### Command-Line Flags

The embeddings tool supports the following flag:

```bash
--once    Run embeddings generation once and exit (default: false, runs continuously)
```

**Examples:**
```bash
# Run once and exit (for local dev)
./bin/init-embeddings-write --once

# Run continuously with scheduler (for production)
./bin/init-embeddings-write

# View help
./bin/init-embeddings-write --help
```

### Execution Flow

1. **Parses command-line flags** - Checks if `--once` is set
2. **Checks for `.env` file** - Fails gracefully if missing (when using Makefile)
3. **Loads environment variables** - Exports all variables from `.env`
4. **Builds the embeddings tool** - If not already built (when using Makefile)
5. **Runs full generation** - Processes all published products
6. **Exits or schedules next run** - Depending on `--once` flag
   - If `--once=true`: Exits cleanly after completion
   - If `--once=false` (default): Enters scheduler loop and waits for next run

---

## 📋 Environment Variables Required

Your `.env` file should contain:

```bash
# Database Connection
DB_HOST=localhost
DB_PORT=3306
DB_USER=isrealde_wp654
DB_PASSWORD=isrealde_wp654
DB_NAME=isrealde_wp654

# OpenAI API
OPENAI_API_KEY=your_api_key_here

# Embedding Schedule (hours between regenerations)
EMBEDDING_SCHEDULE_HOURS=168
```

---

## ❗ Troubleshooting

### Error: ".env file not found"
**Solution:** Make sure you're running from the project root where `.env` exists:
```bash
cd /Users/elad/Development/jshipster/ids
make run-embeddings
```

### Error: "Failed to connect to database"
**Solution:** Verify your database credentials in `.env` and that the database is running.

### Error: "OpenAI API error"
**Solution:** Check that `OPENAI_API_KEY` in `.env` is valid and has sufficient credits.

---

## 📊 What Gets Improved

After regenerating with the latest code, products benefit from:

1. **Better Brand Recognition**
   - "Recover Tactical" automatically detected and boosted
   - Brand names repeated for higher weight

2. **Synonym Expansion**
   - dubon → doobon, parka, coat
   - pix → p-ix, p-ix+
   - coat → jacket, parka

3. **Variation Keywords**
   - Product variations analyzed
   - Unique keywords extracted and included

4. **Boosting System**
   - Exact title matches: +0.2 similarity boost
   - Tag matches: +0.25 boost per token
   - Title token matches: +0.05 boost

5. **Special Product Handling**
   - P-IX+ products get extra keyword repetition
   - Recover Tactical brand explicitly added when missing
   - Product variation keywords extracted

---

## 🎯 Summary

### For Local Development:
```bash
make run-embeddings
```
Runs once and exits cleanly - no Ctrl+C needed!

### For Production (Kubernetes):
```bash
./bin/init-embeddings-write
```
Runs continuously with automatic scheduling.

---

## 🚀 Production Deployment Notes

### Kubernetes Deployment
Your production deployment should run **without** the `--once` flag to enable continuous scheduling:

```yaml
# Correct for production
command: ["/app/bin/init-embeddings-write"]

# NOT for production (would exit after first run)
command: ["/app/bin/init-embeddings-write", "--once"]
```

The default behavior (no flags) is **production-ready** and will:
1. Run initial embeddings generation on startup
2. Continue running and regenerate on schedule (e.g., weekly)
3. Handle signals gracefully (SIGTERM for pod shutdown)
4. Restart automatically if the pod crashes (via Kubernetes restart policy)

### Backwards Compatibility
✅ **No breaking changes!** Existing production deployments will continue working exactly as before. The default behavior is unchanged.

---

**Created:** November 19, 2025  
**Updated:** November 19, 2025  
**For:** IDS API Project  
**Purpose:** Improve product search relevance
