# Account Monitoring & Usage Tracking

This guide covers the account-level monitoring features in execbox-cloud, including usage tracking, cost estimation, and account limits.

## Overview

The monitoring system provides:
- **Real-time usage tracking** - Sessions, executions, and resource consumption
- **Cost estimation** - Automatic cost calculation based on CPU, memory, and duration
- **Account limits** - Configurable daily and concurrent request limits
- **Historical data** - Hourly and daily usage aggregations
- **Export capabilities** - Download usage data as CSV

## API Endpoints

### GET /v1/account/usage

Returns basic usage statistics for the current day.

```bash
curl -H "Authorization: Bearer $API_KEY" \
  https://api.execbox.dev/v1/account/usage
```

**Response:**
```json
{
  "sessions_today": 42,
  "active_sessions": 2,
  "quota_used": 42,
  "quota_remaining": 58,
  "tier": "free",
  "concurrent_limit": 5,
  "daily_limit": 100,
  "max_duration_seconds": 300,
  "max_memory_mb": 512
}
```

### GET /v1/account/usage/enhanced

Returns detailed usage with historical data and cost estimates.

**Query Parameters:**
- `days` (optional, default: 7) - Number of days of history to return

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "https://api.execbox.dev/v1/account/usage/enhanced?days=30"
```

**Response:**
```json
{
  "sessions_today": 42,
  "active_sessions": 2,
  "quota_used": 42,
  "quota_remaining": 58,
  "tier": "free",
  "account_id": "550e8400-e29b-41d4-a716-446655440000",
  "hourly_usage": [
    {
      "hour": "2024-01-22T14:00:00Z",
      "executions": 5,
      "cost_cents": 12,
      "errors": 0
    }
  ],
  "daily_history": [
    {
      "date": "2024-01-22",
      "executions": 42,
      "duration_ms": 125000,
      "cost_cents": 150,
      "errors": 2
    }
  ],
  "cost_estimate_cents": 450,
  "alert_threshold": 80
}
```

### GET /v1/account/limits

Returns the account's configured limits.

```bash
curl -H "Authorization: Bearer $API_KEY" \
  https://api.execbox.dev/v1/account/limits
```

**Response:**
```json
{
  "daily_requests_limit": 100,
  "concurrent_requests_limit": 5,
  "monthly_cost_limit_cents": null,
  "alert_threshold": 80,
  "billing_email": null,
  "timezone": "UTC"
}
```

### PUT /v1/account/limits

Update account limits (within tier allowances).

```bash
curl -X PUT -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"alert_threshold": 90, "timezone": "America/New_York"}' \
  https://api.execbox.dev/v1/account/limits
```

**Request Body (all fields optional):**
```json
{
  "daily_requests_limit": 100,
  "concurrent_requests_limit": 5,
  "monthly_cost_limit_cents": 10000,
  "alert_threshold": 90,
  "billing_email": "billing@example.com",
  "timezone": "America/New_York"
}
```

### GET /v1/account/usage/export

Export usage data as JSON (CSV available via dashboard).

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "https://api.execbox.dev/v1/account/usage/export?days=30"
```

**Response:** Array of daily usage records.

## Cost Calculation

Costs are estimated based on:

| Resource | Rate |
|----------|------|
| Base cost per request | $0.001 |
| CPU time | $0.00005 per CPU-second |
| Memory | $0.00001 per GB-second |

**Example:** A 10-second session using 1 CPU core and 512MB memory:
- Base: $0.001
- CPU: 10 × $0.00005 = $0.0005
- Memory: 10 × 0.5 × $0.00001 = $0.00005
- **Total: ~$0.0016 (0.16 cents)**

## Database Triggers

Usage data is automatically aggregated via PostgreSQL triggers:

1. **Session completion** → Updates `hourly_account_usage`
2. **Daily rollup** → Updates `account_cost_tracking`

Triggers fire only when sessions transition to terminal states (`stopped`, `failed`, `killed`), preventing double-counting.

## Tier Limits

| Tier | Daily Sessions | Concurrent | Max Duration | Max Memory |
|------|---------------|------------|--------------|------------|
| Free | 100 | 5 | 5 min | 512 MB |
| Starter | 1,000 | 10 | 15 min | 1 GB |
| Pro | 10,000 | 50 | 60 min | 4 GB |
| Enterprise | Unlimited | Unlimited | Unlimited | 16 GB |

## Dashboard

The web dashboard at `/dashboard` provides:

- Real-time usage statistics
- Interactive charts (7/30/90 day views)
- Hourly activity breakdown
- CSV export functionality
- Account settings management

## Best Practices

1. **Monitor usage regularly** - Use the enhanced usage endpoint or dashboard
2. **Set alert thresholds** - Get notified before hitting limits
3. **Export data** - Keep records for billing reconciliation
4. **Use appropriate tier** - Upgrade before hitting limits to avoid disruption
