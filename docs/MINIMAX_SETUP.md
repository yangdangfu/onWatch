# MiniMax Setup Guide

This guide configures MiniMax Coding Plan usage tracking in onWatch.

## Regional Support

MiniMax has two regional endpoints:

| Region | Platform | API Endpoint |
|--------|----------|--------------|
| International | platform.minimax.io | `https://api.minimax.io/v1/api/openplatform/coding_plan/remains` |
| China (海螺AI) | platform.minimax.chat | `https://api.minimax.chat/v1/api/openplatform/coding_plan/remains` |

By default, onWatch uses the **International** endpoint. Use `MINIMAX_REGION=china` for the China domestic version.

## Prerequisites

- Active MiniMax Coding Plan subscription
- MiniMax API key

## Configuration

### 1. Get Your API Key

**International (platform.minimax.io):**
1. Open https://platform.minimax.io
2. Go to **API Keys**
3. Create or copy an API key

**China (海螺AI - platform.minimax.chat):**
1. Open https://platform.minimax.chat
2. Go to **API Keys**
3. Create or copy an API key

### 2. Add to Configuration

Add to your `~/.onwatch/.env` file:

**International (default):**
```bash
MINIMAX_API_KEY=your-api-key-here
```

**China (海螺AI):**
```bash
MINIMAX_API_KEY=your-api-key-here
MINIMAX_REGION=china
```

### 3. Restart onWatch

```bash
onwatch stop
onwatch
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `MINIMAX_API_KEY` | Yes | Your MiniMax API key |
| `MINIMAX_REGION` | No | `international` (default) or `china` |
| `MINIMAX_BASE_URL` | No | Override API endpoint (auto-set based on region) |

## Verification

After configuration, onWatch will:
1. Display "MiniMax" in the providers list at startup
2. Poll MiniMax's API every 5 minutes (by default)
3. Show quota cards for M2, M2.1, and M2.5 models

## Troubleshooting

### "minimax: unauthorized" Error
- Verify your `MINIMAX_API_KEY` is correct
- Check that the API key hasn't expired
- Ensure you're using the correct region setting

### "minimax: network error" Error
- Check your internet connection
- For China region, ensure you can access `api.minimax.chat`
- Check if there are firewall rules blocking the request

### No Data Appearing
- Ensure onWatch is running (`onwatch status`)
- Check the logs for errors (`~/.onwatch/.onwatch.log`)
- Verify you have an active MiniMax Coding Plan subscription

## Notes

- Auth is sent as a `Bearer` token
- onWatch stores usage snapshots locally in SQLite
- MiniMax uses a shared quota pool across M2, M2.1, and M2.5 models
