# Database Metadata for AI Agents - Israel Defense Store

## Overview
This document provides comprehensive metadata about the `isrealde_wp654` MariaDB database to enable AI agents to provide accurate product recommendations and understand the tactical gear e-commerce system.

## Database Connection
- **Database**: `isrealde_wp654`
- **Type**: MariaDB (WordPress + WooCommerce)
- **Connection String**: `root:my-secret-pw@tcp(localhost:3306)/isrealde_wp654`
- **Total Products**: 2,004 products
- **Published Products**: 1,826 (91.1%)
- **Private Products**: 1 (0.05%)

## Core Product Tables

### 1. wpjr_posts (Main Product Content)
**Purpose**: Stores all product content including titles, descriptions, and metadata.

| Field | Type | Description | Key Info |
|-------|------|-------------|----------|
| ID | bigint(20) unsigned | Primary key, Product ID | Auto-increment |
| post_title | text | Product name/title | Main product identifier |
| post_content | longtext | Full product description | Detailed specifications |
| post_excerpt | text | Short description | Used for summaries |
| post_status | varchar(20) | Product status | 'publish' (1,826), 'private' (1), 'draft' (109) |
| post_type | varchar(20) | Content type | 'product' for all products |
| post_date | datetime | Creation date | When product was added |
| post_modified | datetime | Last modified | Last update timestamp |

**AI Usage**: Use `post_title` for product names, `post_content` for detailed descriptions, `post_excerpt` for short summaries.

### 2. wpjr_wc_product_meta_lookup (Product Commerce Data)
**Purpose**: WooCommerce-specific product metadata including pricing, inventory, and commerce attributes.

| Field | Type | Description | Key Info |
|-------|------|-------------|----------|
| product_id | bigint(20) | Links to wpjr_posts.ID | Primary key |
| sku | varchar(100) | Product SKU | Unique identifier |
| min_price | decimal(19,4) | Minimum price | Range: $1.25 - $2,999.99 |
| max_price | decimal(19,4) | Maximum price | Average: $197.15 |
| stock_quantity | double | Available quantity | Inventory count |
| stock_status | varchar(100) | Stock availability | 'instock' (7,437), 'outofstock' (794) |
| virtual | tinyint(1) | Virtual product flag | 0 (8,221), 1 (11) |
| downloadable | tinyint(1) | Downloadable product | 0 (8,221), 1 (1) |
| onsale | tinyint(1) | Sale status | Sale indicator |
| rating_count | bigint(20) | Number of reviews | Review count |
| average_rating | decimal(3,2) | Average rating | 0.00 - 5.00 |
| total_sales | bigint(20) | Total units sold | Sales performance |

**AI Usage**: Use `min_price`/`max_price` for pricing recommendations, `stock_status` for availability, `sku` for product identification.

## Taxonomy System (Categories & Tags)

### 3. wpjr_terms (Taxonomy Terms)
**Purpose**: Stores all taxonomy terms (categories, tags, brands, etc.).

| Field | Type | Description | Key Info |
|-------|------|-------------|----------|
| term_id | bigint(20) unsigned | Primary key | Auto-increment |
| name | varchar(200) | Term name | Category/tag name |
| slug | varchar(200) | URL-friendly name | SEO-friendly identifier |
| term_group | bigint(10) | Grouping | Organization field |

### 4. wpjr_term_taxonomy (Taxonomy Definitions)
**Purpose**: Defines taxonomy types and their properties.

| Field | Type | Description | Key Info |
|-------|------|-------------|----------|
| term_taxonomy_id | bigint(20) unsigned | Primary key | Auto-increment |
| term_id | bigint(20) | Links to wpjr_terms | Foreign key |
| taxonomy | varchar(32) | Taxonomy type | See taxonomy types below |
| description | longtext | Taxonomy description | Detailed info |
| parent | bigint(20) unsigned | Parent taxonomy | Hierarchy support |
| count | bigint(20) | Usage count | How many products use this |

### 5. wpjr_term_relationships (Product-Taxonomy Links)
**Purpose**: Links products to their categories, tags, and other taxonomies.

| Field | Type | Description | Key Info |
|-------|------|-------------|----------|
| object_id | bigint(20) unsigned | Product ID | Links to wpjr_posts.ID |
| term_taxonomy_id | bigint(20) unsigned | Taxonomy ID | Links to wpjr_term_taxonomy |
| term_order | int(11) | Display order | Sorting within taxonomies |

## Taxonomy Types Available

### Product Categories (155 total)
**Taxonomy**: `product_cat`
**Top Categories by Product Count**:
- Gun Holsters (380 products)
- Fobus (309 products) 
- SALE (251 products)
- Recoil Systems (230 products)
- DPM (224 products)
- FAB Defense (217 products)
- Conversion Kits (161 products)
- ORPAZ Defense (110 products)

### Product Tags (1,471 total)
**Taxonomy**: `product_tag`
**Top Tags by Product Count**:
- Black (1,311 products)
- Right Hand (525 products)
- For Pistols (455 products)
- Od Green (397 products)
- OWB (380 products)
- Paddle (364 products)
- Duty Holster (357 products)
- Glock (332 products)
- Polymer Holster (310 products)
- Belt Loop (293 products)
- Molle (293 products)
- Left Hand (287 products)
- For Rifles (281 products)
- Thigh Rig / Drop Leg (275 products)
- Modular (274 products)

### Other Taxonomies
- **product_brand**: Product manufacturers/brands
- **product_shipping_class**: Shipping categories
- **product_type**: Product types (simple, variable, etc.)
- **product_visibility**: Visibility settings (catalog, search, etc.)

## Key Relationships for AI Queries

### Main Product Query Pattern
```sql
SELECT 
  p.ID, p.post_title, p.post_content, p.post_excerpt,
  l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity,
  GROUP_CONCAT(DISTINCT t.name ORDER BY t.name SEPARATOR ', ') AS tags
FROM wpjr_wc_product_meta_lookup l
JOIN wpjr_posts p ON p.ID = l.product_id
LEFT JOIN wpjr_term_relationships tr ON tr.object_id = p.ID
LEFT JOIN wpjr_term_taxonomy tt ON tt.term_taxonomy_id = tr.term_taxonomy_id
LEFT JOIN wpjr_terms t ON t.term_id = tt.term_id
WHERE p.post_type = 'product' 
  AND p.post_status IN ('publish','private')
GROUP BY p.ID, p.post_title, p.post_content, p.post_excerpt,
         l.sku, l.min_price, l.max_price, l.stock_status, l.stock_quantity
```

## Business Context for AI Recommendations

### Product Categories
- **Primary Focus**: Tactical gear, military equipment, gun accessories
- **Key Brands**: Fobus, DPM, FAB Defense, ORPAZ Defense
- **Product Types**: Holsters, conversion kits, recoil systems, accessories

### Pricing Ranges
- **Minimum Price**: $1.25
- **Maximum Price**: $2,999.99
- **Average Price**: $197.15
- **Price Distribution**: Wide range from budget to premium tactical gear

### Inventory Status
- **In Stock**: 7,437 products (90.4%)
- **Out of Stock**: 794 products (9.6%)
- **Stock Management**: Real-time inventory tracking available

### Product Attributes
- **Physical Products**: 8,221 (99.9%)
- **Virtual Products**: 11 (0.1%)
- **Downloadable Products**: 1 (0.01%)

## AI Agent Usage Guidelines

### For Product Recommendations
1. **Use Categories**: Filter by `product_cat` for broad product types
2. **Use Tags**: Filter by `product_tag` for specific attributes (color, hand orientation, compatibility)
3. **Check Availability**: Always verify `stock_status = 'instock'`
4. **Consider Price Range**: Use `min_price` and `max_price` for budget filtering
5. **Match Compatibility**: Use tags like "Glock", "For Pistols", "Right Hand" for compatibility

### For Search and Discovery
1. **Search Fields**: `post_title`, `post_content`, `post_excerpt`
2. **Filter by Status**: Only show `post_status IN ('publish', 'private')`
3. **Use SKU**: For exact product identification
4. **Leverage Tags**: For attribute-based filtering

### For Inventory Management
1. **Stock Status**: Check `stock_status` before recommending
2. **Quantity Available**: Use `stock_quantity` for availability details
3. **Sales Performance**: Consider `total_sales` for popular items

### For Pricing Information
1. **Price Range**: Use both `min_price` and `max_price` for variable pricing
2. **Sale Status**: Check `onsale` flag for discounted items
3. **Currency**: All prices in USD

## Sample Queries for AI Agents

### Find Holsters for Glock Pistols
```sql
SELECT p.post_title, l.sku, l.min_price, l.stock_status
FROM wpjr_posts p
JOIN wpjr_wc_product_meta_lookup l ON p.ID = l.product_id
JOIN wpjr_term_relationships tr ON p.ID = tr.object_id
JOIN wpjr_term_taxonomy tt ON tr.term_taxonomy_id = tt.term_taxonomy_id
JOIN wpjr_terms t ON tt.term_id = t.term_id
WHERE p.post_type = 'product' 
  AND p.post_status = 'publish'
  AND tt.taxonomy = 'product_tag'
  AND t.name = 'Glock'
  AND p.post_title LIKE '%holster%'
  AND l.stock_status = 'instock'
```

### Find Products Under $100
```sql
SELECT p.post_title, l.sku, l.min_price, l.max_price
FROM wpjr_posts p
JOIN wpjr_wc_product_meta_lookup l ON p.ID = l.product_id
WHERE p.post_type = 'product'
  AND p.post_status = 'publish'
  AND l.min_price <= 100
  AND l.stock_status = 'instock'
ORDER BY l.min_price
```

This metadata enables AI agents to provide accurate, context-aware recommendations for tactical gear customers based on their specific needs, budget, and compatibility requirements.
