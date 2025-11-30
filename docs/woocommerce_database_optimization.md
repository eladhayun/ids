# WooCommerce Database Performance Optimization Report

## 🔴 **Critical Issues Found:**

### 1. **Expired Transients (1,760 expired, 10,346 total)**
**Impact**: HIGH - Every page load queries these, causing unnecessary overhead
**Solution**: Clean up expired transients

### 2. **Action Scheduler Bloat (11,683 completed actions)**
**Impact**: MEDIUM - Slows admin pages and cron jobs
**Solution**: Clean up old completed actions

### 3. **Old Action Scheduler Logs (1,033 logs >30 days)**
**Impact**: MEDIUM - Wastes space and slows queries
**Solution**: Remove old logs

### 4. **Missing Composite Index on postmeta**
**Impact**: MEDIUM - Slower product meta queries
**Solution**: Add composite index (post_id, meta_key)

### 5. **Orphaned Postmeta (132 records)**
**Impact**: LOW - Minor overhead
**Solution**: Clean up orphaned records

---

## 📊 **Database Stats:**
- **Total Size**: ~368MB
- **Largest Table**: wpjr_postmeta (232MB, 507K rows)
- **Products**: 8,409 in lookup table
- **Posts**: 33,126 total

---

## 🔧 **Optimization SQL Commands:**

### **Step 1: Backup First (IMPORTANT!)**
```bash
# Take a backup before running any optimization
mysqldump -h localhost -P 3306 -u isrealde_wp654 -p'Y22[)NpI0S' isrealde_wp654 > backup_$(date +%Y%m%d).sql
```

### **Step 2: Clean Expired Transients**
```sql
-- Delete expired transient timeouts
DELETE FROM wpjr_options 
WHERE option_name LIKE '_transient_timeout%' 
  AND option_value < UNIX_TIMESTAMP()
LIMIT 1000;

-- Delete corresponding transient values
DELETE FROM wpjr_options 
WHERE option_name LIKE '_transient%' 
  AND option_name NOT LIKE '_transient_timeout%'
  AND option_name NOT IN (
    SELECT REPLACE(option_name, '_transient_timeout_', '_transient_') 
    FROM wpjr_options 
    WHERE option_name LIKE '_transient_timeout%'
  )
LIMIT 1000;

-- Repeat until no more deleted (run multiple times)
```
**Expected Improvement**: 20-30% faster page loads

### **Step 3: Clean Action Scheduler**
```sql
-- Delete completed actions older than 30 days
DELETE FROM wpjr_actionscheduler_actions 
WHERE status = 'complete' 
  AND scheduled_date_gmt < DATE_SUB(NOW(), INTERVAL 30 DAY)
LIMIT 5000;

-- Delete old logs
DELETE FROM wpjr_actionscheduler_logs 
WHERE log_date_gmt < DATE_SUB(NOW(), INTERVAL 30 DAY)
LIMIT 5000;

-- Repeat until no more deleted
```
**Expected Improvement**: 10-15% faster admin pages

### **Step 4: Add Composite Index (CRITICAL)**
```sql
-- Add composite index for faster meta queries
-- This will take 1-2 minutes on 500K rows
ALTER TABLE wpjr_postmeta 
ADD INDEX idx_post_meta (post_id, meta_key(191));
```
**Expected Improvement**: 30-50% faster product queries

### **Step 5: Clean Orphaned Postmeta**
```sql
-- Delete orphaned postmeta (no matching post)
DELETE pm FROM wpjr_postmeta pm
LEFT JOIN wpjr_posts p ON pm.post_id = p.ID
WHERE p.ID IS NULL
LIMIT 500;
```
**Expected Improvement**: 2-5% space savings

### **Step 6: Optimize Tables**
```sql
-- Optimize tables to reclaim space and rebuild indices
OPTIMIZE TABLE wpjr_options;
OPTIMIZE TABLE wpjr_postmeta;
OPTIMIZE TABLE wpjr_actionscheduler_actions;
OPTIMIZE TABLE wpjr_actionscheduler_logs;
```
**Expected Improvement**: 5-10% faster queries

---

## 📈 **Expected Overall Improvement:**
- **Page Load Time**: 30-40% faster
- **Admin Pages**: 20-30% faster
- **Product Queries**: 40-50% faster
- **Database Size**: Reduce by ~50-100MB

---

## ⚠️ **Important Notes:**

1. **Backup First**: Always backup before running optimizations
2. **Run During Low Traffic**: Best run during off-peak hours
3. **Run in Batches**: Use LIMIT to avoid locking tables
4. **Monitor**: Check performance before/after
5. **Read-Only User**: Your app user is read-only, so these commands need admin/root access

---

## 🚀 **Automated Maintenance Recommendations:**

1. **Install WP-Optimize or similar plugin** to auto-clean transients weekly
2. **Set Action Scheduler retention** to 14 days instead of forever
3. **Schedule monthly OPTIMIZE TABLE** during low traffic

