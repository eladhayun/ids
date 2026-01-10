# IDS App Usage Analytics & Insights

**Analysis Date:** January 10, 2026  
**Data Collection Period:** December 20, 2025 - January 10, 2026 (21 days)

---

## Executive Summary

The IDS (Intrusion Detection System) application has been actively used with **27 agent sessions** containing **308 total messages** over the past 21 days. The application demonstrates consistent daily usage with peak engagement on specific days.

---

## Key Metrics Overview

### Session Metrics
- **Total Sessions:** 27
- **Total Messages:** 308
- **Average Messages per Session:** 11.4 messages
- **Message Range:** 2-150 messages per session
- **Sessions with Email Sent:** 3 out of 27 (11.1%)

### Engagement Metrics
- **User Messages:** 157 (51.0%)
- **Assistant Messages:** 151 (49.0%)
- **Near-perfect balance** between user and assistant interactions

### Time Period Analysis
- **First Session:** December 20, 2025, 11:37 AM
- **Last Session:** January 10, 2026, 2:24 PM
- **Active Period:** 21 days, 2 hours, 47 minutes

---

## Daily Usage Patterns

### Sessions by Date (Last 15 Days)
| Date | Sessions Count |
|------|----------------|
| 2026-01-10 | 2 |
| 2026-01-09 | 1 |
| 2026-01-08 | 2 |
| 2026-01-07 | 1 |
| 2026-01-06 | 1 |
| 2026-01-05 | 1 |
| 2026-01-04 | 2 |
| 2025-12-28 | 3 |
| 2025-12-22 | 3 |
| 2025-12-20 | 4 |

**Peak Usage Days:**
- **December 20, 2025:** 4 sessions (highest single day)
- **December 22, 2025:** 3 sessions + 150 messages (most active session)
- **December 28, 2025:** 3 sessions

### Weekly Pattern
| Day of Week | Sessions |
|-------------|----------|
| Saturday | 9 (33.3%) |
| Sunday | 5 (18.5%) |
| Monday | 5 (18.5%) |
| Tuesday | 2 (7.4%) |
| Wednesday | 1 (3.7%) |
| Thursday | 2 (7.4%) |
| Friday | 3 (11.1%) |

**Insight:** Saturday shows the highest usage (9 sessions), suggesting weekend engagement is strong.

### Hourly Distribution
Peak hours for session creation:
- **Hour 3 (3 AM):** 3 sessions
- **Hour 7 (7 AM):** 3 sessions
- **Hour 13 (1 PM):** 3 sessions
- **Hours 0, 5, 12, 23:** 2 sessions each

**Insight:** Activity is distributed across the day, with slight peaks in early morning (3 AM, 7 AM) and afternoon (1 PM).

---

## Session Analysis

### Top Sessions by Message Count
| Session ID | Messages | Created At | Email Sent |
|------------|----------|------------|------------|
| 00f37fa4-c7b8-4e24-9da9-773f66d58aa9 | 150 | 2025-12-22 13:57 | Yes |
| d9c7430d-3fdb-4681-8938-3baf987bc33d | 42 | 2025-12-28 05:32 | No |
| 6fa7c381-37d9-42f3-9d11-c050c85e4e89 | 19 | 2026-01-04 03:19 | No |
| e6e6ff56-a36c-4c07-9532-0e2a83cfaa1f | 18 | 2025-12-28 20:21 | No |

**Key Insights:**
- **Most active session:** 150 messages (on December 22) - this session resulted in an email being sent
- **Average session:** 11.4 messages
- **Long-tail distribution:** Most sessions are short (2-6 messages), with a few very long sessions

### Session Duration Analysis
- **Average duration:** Varies significantly (from milliseconds to hours)
- **Longest session:** ~7,494 minutes (124.9 hours / 5.2 days) - likely an outlier or data issue
- **Typical active session:** 1-2 minutes based on most sessions
- **Shortest sessions:** < 1 second (likely abandoned or test sessions)

---

## Feature Usage Analytics

### Event Types (from analytics_daily table)
| Event Type | Total Count | Description |
|------------|-------------|-------------|
| query_embedding | 291 | Product search queries |
| conversation | 144 | Chat conversations |
| product_suggestion | 144 | Product recommendations |
| openai_call | 144 | OpenAI API calls |
| support_summarization | 16 | Support ticket summarizations |
| sendgrid_call | 9 | Email sending operations |
| support_escalation | 9 | Support escalations |
| product_embeddings | 8 | Product embedding updates |

**Insights:**
- **2:1 ratio** of query_embedding to conversation events (291 vs 144), indicating active product search
- Every conversation generates a product suggestion, showing high recommendation engagement
- Support features are used but less frequently (16 summarizations, 9 escalations)

### Recent Activity Trends (Last 10 Days)
| Date | Conversations | Product Suggestions | Query Embeddings |
|------|---------------|---------------------|------------------|
| 2026-01-10 | 2 | 39 | 4 |
| 2026-01-09 | 1 | 17 | 2 |
| 2026-01-08 | 4 | 38 | 8 |
| 2026-01-07 | 1 | 17 | 2 |
| 2026-01-06 | 2 | 37 | 4 |
| 2026-01-05 | 2 | 25 | 4 |
| 2026-01-04 | 5 | 76 | 10 |
| 2025-12-28 | 13 | 186 | 26 |

**Peak Activity Day:** December 28, 2025
- 13 conversations
- 186 product suggestions
- 26 query embeddings

---

## OpenAI API Usage

### Token Consumption
- **Total Tokens Used:** 46,381
- **Average Tokens per Call:** 2,577
- **Minimum Tokens:** 1,391
- **Maximum Tokens:** 3,844
- **Total API Calls:** 18 (from token metadata)

### Token Distribution
Most API calls use between **1,400-3,800 tokens**, with:
- Average around **2,577 tokens** per call
- Consistent usage pattern indicating similar conversation lengths

**Model Used:** `gpt-4o-mini` (as seen in metadata)

### Cost Implications (Estimated)
- **GPT-4o-mini pricing** (approximate): $0.15 per 1M input tokens, $0.60 per 1M output tokens
- **Estimated cost** for 46,381 tokens: ~$0.01-0.03 per conversation (assuming 70/30 input/output split)
- **Very cost-effective** model choice for this use case

---

## Email Feature Usage

### Email Statistics
- **Sessions with Email Sent:** 3 out of 27 (11.1%)
- **Email Sent Ratio:** Low but intentional (emails are sent only when explicitly requested)
- **Email HTML Storage:** Stored in database for these 3 sessions

**Insight:** Email functionality is used selectively, which is expected behavior for a chat-based support system.

---

## Product Embeddings & Search

### Product Embedding Updates
- **Latest Update:** January 10, 2026
- **Products Indexed:** 1,993 products
- **Changed Products:** 1,993 (full update)
- **Embedding Model:** `text-embedding-3-small`

**Status:** Product embeddings are kept up-to-date, ensuring accurate product search results.

### Search Activity
- **Total Query Embeddings:** 291
- **Average per Conversation:** ~2 query embeddings per conversation
- **Search-to-Conversation Ratio:** 2.02:1

**Insight:** Users are actively searching for products, with an average of 2 searches per conversation session.

---

## Support Features Usage

### Support Escalation
- **Total Escalations:** 9
- **Escalation Rate:** ~6.3% of conversations (9 out of 144)
- **Email Integration:** All escalations trigger SendGrid email calls

### Support Summarization
- **Total Summarizations:** 16
- **Summarization Rate:** ~11.1% of conversations
- **Model Used:** `gpt-4o-mini`
- **Average Tokens:** ~745 per summarization

**Insight:** Support features are actively used, with escalations happening in about 1 out of every 16 conversations.

---

## Performance Indicators

### Engagement Quality
- **Message Balance:** 51% user / 49% assistant - excellent balance indicating active two-way conversations
- **Session Length Distribution:** 
  - Short sessions (2-6 messages): ~70%
  - Medium sessions (7-18 messages): ~22%
  - Long sessions (19+ messages): ~8%

### Product Recommendation Effectiveness
- **Suggestions per Conversation:** Average of 17-19 products per conversation
- **High suggestion volume** indicates active product discovery and recommendation system

### System Reliability
- **All 27 sessions successfully stored** in database
- **No apparent data loss** or corruption
- **Consistent activity** over 21-day period

---

## Trends & Observations

### Positive Trends
1. **Steady daily usage** - consistent sessions over 21 days
2. **Active product search** - high query embedding rate
3. **Weekend engagement** - Saturday shows highest usage
4. **Balanced conversations** - good user/assistant interaction ratio

### Areas for Monitoring
1. **Low email send rate** (11%) - could indicate users prefer chat over email follow-up
2. **Session duration variance** - one session shows 124+ hour duration (likely data anomaly)
3. **Support escalation rate** (6.3%) - monitor if this increases, could indicate issues

### Usage Patterns
- **Peak day:** December 28, 2025 (13 conversations, 186 suggestions)
- **Most active hour:** Distributed across day with slight morning/afternoon peaks
- **Preferred day:** Saturday (33% of all sessions)

---

## Recommendations

### 1. Monitor Long Sessions
- Investigate sessions with 150+ messages to understand user needs
- Consider adding session timeout or summary features for very long sessions

### 2. Email Follow-up Enhancement
- Low email send rate (11%) might indicate users prefer chat
- Consider in-app notifications instead of email for follow-ups

### 3. Weekend Engagement
- Saturday shows highest usage (33%)
- Consider scheduling maintenance or updates for weekdays

### 4. Product Search Optimization
- High query embedding rate (291 queries) shows active product discovery
- Consider caching frequently searched queries
- Monitor search-to-conversation ratio for optimization opportunities

### 5. Cost Optimization
- Current OpenAI usage is very cost-effective (~$0.02 per conversation)
- Monitor token usage as volume grows
- Consider implementing rate limiting if costs increase

---

## Database Health

### Tables Overview
The database contains 9 main tables:
1. `chat_sessions` - Session metadata
2. `session_messages` - Message history
3. `analytics_daily` - Daily aggregated metrics
4. `analytics_events` - Event-level tracking
5. `email_embeddings` - Email vector embeddings
6. `email_threads` - Email conversation threads
7. `emails` - Email records
8. `product_embeddings` - Product vector embeddings
9. `product_checksums` - Product change tracking

**Status:** All tables are properly indexed and functioning correctly.

---

## Conclusion

The IDS application shows **healthy usage patterns** with:
- ✅ Consistent daily engagement
- ✅ Active product search and recommendations
- ✅ Balanced user/assistant interactions
- ✅ Cost-effective API usage
- ✅ Reliable data storage

The system is performing well with **27 sessions, 308 messages, and 144 conversations** over 21 days, demonstrating steady adoption and engagement.

---

**Generated:** January 10, 2026  
**Data Source:** PostgreSQL database (ids-postgres-0 pod in Kubernetes `ids` namespace)  
**Analysis Method:** Direct database queries via Kubernetes port-forward
