# Kimi Setup Guide

This guide explains how to configure onWatch to track your Kimi Code quota usage.

## Regional Support

Kimi has two distinct services with different API endpoints:

| Region | Service | Platform | API Endpoint |
|--------|---------|----------|--------------|
| International | Kimi Code | kimi.com | `https://api.kimi.com/coding/v1/usages` |
| China | Moonshot | platform.moonshot.cn | `https://api.moonshot.cn/v1/users/me/balance` |

By default, onWatch uses the **International** endpoint. Use `KIMI_REGION=china` for the Moonshot/Moonshot AI (月之暗面) China domestic version.

## Prerequisites

- A Kimi Code account (International) or Moonshot account (China) with an active subscription
- Your API key from the respective platform

## Configuration

### 1. Get Your API Key

**International (kimi.com):**
1. Visit https://www.kimi.com/code
2. Navigate to the membership/API section
3. Generate or copy your API key (format: `sk-kimi-xxx...`)

**China (platform.moonshot.cn):**
1. Visit https://platform.moonshot.cn/
2. Create an API key from the console
3. Copy your API key

### 2. Add to onWatch Configuration

Add the following to your `~/.onwatch/.env` file:

**International (default):**
```bash
KIMI_API_KEY=sk-kimi-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**China (Moonshot/月之暗面):**
```bash
KIMI_API_KEY=your-moonshot-api-key-here
KIMI_REGION=china
```

### 3. Restart onWatch

```bash
onwatch stop
onwatch
```

## Verification

After configuration, onWatch will:

1. Display "Kimi" in the providers list at startup (with region info)
2. Poll Kimi's API every 2 minutes (by default)
3. Show quota cards for:
   - **Tokens**: Token consumption budget with reset time
   - **Time**: Rate limit window (5-minute rolling window)
4. Display usage history, cycles, and session tracking in the dashboard

## API Response Example

**International (api.kimi.com/coding/v1/usages):**
```json
{
  "user": {
    "userId": "...",
    "region": "REGION_CN",
    "membership": {"level": "LEVEL_INTERMEDIATE"}
  },
  "usage": {
    "limit": "100",
    "used": "23",
    "remaining": "77",
    "resetTime": "2026-03-12T06:11:36.813745Z"
  },
  "limits": [
    {
      "window": {"duration": 300, "timeUnit": "TIME_UNIT_MINUTE"},
      "detail": {
        "limit": "100",
        "used": "2",
        "remaining": "98",
        "resetTime": "2026-03-12T08:11:36.813745Z"
      }
    }
  ]
}
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `KIMI_API_KEY` | Yes | Your Kimi Code or Moonshot API key |
| `KIMI_REGION` | No | Region: `international` (default), `china`, `cn`, or `domestic` |
| `KIMI_BASE_URL` | No | Override API endpoint (usually not needed) |

## Troubleshooting

### 404 Error / "Unable to load insights"

1. **Check your region setting** - Ensure `KIMI_REGION` matches your account type
2. **Verify API key format**:
   - International: starts with `sk-kimi-`
   - China: Moonshot API key format
3. **Try the other region** - Your API key might be for the other service

### "kimi: unauthorized" Error

- Verify your `KIMI_API_KEY` is correct
- Check that the API key hasn't expired
- Ensure the API key has the necessary permissions

### "kimi: network error" Error

- Check your internet connection
- For International, ensure you can access `api.kimi.com`
- For China, ensure you can access `api.moonshot.cn`
- Check if there are firewall rules blocking the request

### No Data Appearing

- Ensure onWatch is running (`onwatch status`)
- Check the logs for errors (`~/.onwatch/.onwatch.log`)
- Wait at least one poll interval (default 2 minutes) for first data

## Related Documentation

- [Kimi Code Third-Party Agents](https://www.kimi.com/code/docs/more/third-party-agents.html)
- [Moonshot AI Platform](https://platform.moonshot.cn/)
- [Claude Code Configuration](https://docs.anthropic.com/en/docs/claude-code)
