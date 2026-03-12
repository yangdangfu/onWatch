package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// KimiResponse is the generic wrapper for all Kimi API responses
type KimiResponse[T any] struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Data    T      `json:"data"`
}

// KimiQuotaResponse is the internal unified response format
type KimiQuotaResponse struct {
	Limits []KimiLimit `json:"limits"`
}

// KimiUsagesResponse is the response from GET /coding/v1/usages (International)
type KimiUsagesResponse struct {
	User struct {
		UserID      string `json:"userId"`
		Region      string `json:"region"`      // "REGION_CN", "REGION_INTL", etc.
		Membership  struct {
			Level string `json:"level"` // "LEVEL_INTERMEDIATE", etc.
		} `json:"membership"`
		BusinessID  string `json:"businessId"`
	} `json:"user"`
	Usage struct {
		Limit     string `json:"limit"`
		Used      string `json:"used"`
		Remaining string `json:"remaining"`
		ResetTime string `json:"resetTime"`
	} `json:"usage"`
	Limits []KimiWindowLimit `json:"limits"`
}

// KimiWindowLimit represents a window-based rate limit
type KimiWindowLimit struct {
	Window struct {
		Duration int    `json:"duration"`
		TimeUnit string `json:"timeUnit"` // "TIME_UNIT_MINUTE"
	} `json:"window"`
	Detail struct {
		Limit     string `json:"limit"`
		Used      string `json:"used"`
		Remaining string `json:"remaining"`
		ResetTime string `json:"resetTime"`
	} `json:"detail"`
}

// MoonshotQuotaResponse is the response format for Moonshot (China) API
type MoonshotQuotaResponse struct {
	Data struct {
		TotalQuota float64 `json:"totalQuota"`
		UsedQuota  float64 `json:"usedQuota"`
		Remaining  float64 `json:"remaining"`
	} `json:"data"`
}

// KimiLimit represents an individual limit (TIME_LIMIT or TOKENS_LIMIT)
type KimiLimit struct {
	Type         string            `json:"type"`
	Unit         int               `json:"unit"`
	Number       int               `json:"number"`
	Usage        float64           `json:"usage"`
	CurrentValue float64           `json:"currentValue"`
	Remaining    float64           `json:"remaining"`
	Percentage   int               `json:"percentage"`
	NextResetMs  *int64            `json:"nextResetTime,omitempty"`
	UsageDetails []KimiUsageDetail `json:"usageDetails,omitempty"`
}

// KimiUsageDetail represents per-model usage breakdown
type KimiUsageDetail struct {
	ModelCode string  `json:"modelCode"`
	Usage     float64 `json:"usage"`
}

// GetResetTime returns the reset time as a time.Time pointer.
func (l *KimiLimit) GetResetTime() *time.Time {
	if l.NextResetMs == nil {
		return nil
	}
	t := time.UnixMilli(*l.NextResetMs)
	return &t
}

// KimiSnapshot is the storage representation (flat, for SQLite)
type KimiSnapshot struct {
	ID         int64
	CapturedAt time.Time
	// TIME_LIMIT fields
	TimeLimit        int
	TimeUnit         int
	TimeNumber       int
	TimeUsage        float64
	TimeCurrentValue float64
	TimeRemaining    float64
	TimePercentage   int
	TimeUsageDetails string
	// TOKENS_LIMIT fields
	TokensLimit         int
	TokensUnit          int
	TokensNumber        int
	TokensUsage         float64
	TokensCurrentValue  float64
	TokensRemaining     float64
	TokensPercentage    int
	TokensNextResetTime *time.Time
}

// ToSnapshot converts KimiQuotaResponse to KimiSnapshot
func (r *KimiQuotaResponse) ToSnapshot(capturedAt time.Time) *KimiSnapshot {
	snapshot := &KimiSnapshot{
		CapturedAt: capturedAt,
	}

	for _, limit := range r.Limits {
		switch limit.Type {
		case "TIME_LIMIT":
			snapshot.TimeLimit = limit.Unit * limit.Number
			snapshot.TimeUnit = limit.Unit
			snapshot.TimeNumber = limit.Number
			snapshot.TimeUsage = limit.Usage
			snapshot.TimeCurrentValue = limit.CurrentValue
			snapshot.TimeRemaining = limit.Remaining
			snapshot.TimePercentage = limit.Percentage
			if len(limit.UsageDetails) > 0 {
				b, _ := json.Marshal(limit.UsageDetails)
				snapshot.TimeUsageDetails = string(b)
			}
		case "TOKENS_LIMIT":
			snapshot.TokensLimit = limit.Unit * limit.Number
			snapshot.TokensUnit = limit.Unit
			snapshot.TokensNumber = limit.Number
			snapshot.TokensUsage = limit.Usage
			snapshot.TokensCurrentValue = limit.CurrentValue
			snapshot.TokensRemaining = limit.Remaining
			snapshot.TokensPercentage = limit.Percentage
			if limit.NextResetMs != nil {
				t := time.UnixMilli(*limit.NextResetMs)
				snapshot.TokensNextResetTime = &t
			}
		}
	}

	return snapshot
}

// ParseKimiUsagesResponse parses the Kimi /v1/usages response format
func ParseKimiUsagesResponse(data []byte) (*KimiQuotaResponse, error) {
	var resp KimiUsagesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	quotaResp := &KimiQuotaResponse{}

	// Parse main usage (treat as tokens limit)
	limit, _ := strconv.ParseFloat(resp.Usage.Limit, 64)
	remaining, _ := strconv.ParseFloat(resp.Usage.Remaining, 64)
	used, _ := strconv.ParseFloat(resp.Usage.Used, 64)
	// If used is not provided, calculate from limit - remaining
	if used == 0 && limit > remaining {
		used = limit - remaining
	}
	percentage := 0
	if limit > 0 {
		percentage = int((used / limit) * 100)
	}

	var nextResetMs *int64
	if resp.Usage.ResetTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, resp.Usage.ResetTime); err == nil {
			ms := t.UnixMilli()
			nextResetMs = &ms
		}
	}

	quotaResp.Limits = append(quotaResp.Limits, KimiLimit{
		Type:         "TOKENS_LIMIT",
		Unit:         1,
		Number:       int(limit),
		Usage:        limit,
		CurrentValue: used,
		Remaining:    remaining,
		Percentage:   percentage,
		NextResetMs:  nextResetMs,
	})

	// Parse window-based limits (rate limits)
	for _, wl := range resp.Limits {
		duration := wl.Window.Duration
		limitVal, _ := strconv.ParseFloat(wl.Detail.Limit, 64)
		remainingVal, _ := strconv.ParseFloat(wl.Detail.Remaining, 64)
		usedVal, _ := strconv.ParseFloat(wl.Detail.Used, 64)
		// If used is not provided, calculate from limit - remaining
		if usedVal == 0 && limitVal > remainingVal {
			usedVal = limitVal - remainingVal
		}

		var nextReset *int64
		if wl.Detail.ResetTime != "" {
			if t, err := time.Parse(time.RFC3339Nano, wl.Detail.ResetTime); err == nil {
				ms := t.UnixMilli()
				nextReset = &ms
			}
		}

		// Treat 5-minute window limits as time limits
		if wl.Window.TimeUnit == "TIME_UNIT_MINUTE" && duration >= 300 {
			quotaResp.Limits = append(quotaResp.Limits, KimiLimit{
				Type:         "TIME_LIMIT",
				Unit:         duration / 60, // minutes
				Number:       1,
				Usage:        limitVal,
				CurrentValue: usedVal,
				Remaining:    remainingVal,
				Percentage:   int((usedVal / limitVal) * 100),
				NextResetMs:  nextReset,
			})
		}
	}

	return quotaResp, nil
}

// ParseKimiResponse parses a Kimi API response from JSON bytes (legacy format)
func ParseKimiResponse(data []byte) (*KimiQuotaResponse, error) {
	var wrapper KimiResponse[KimiQuotaResponse]
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	if !wrapper.Success {
		return nil, fmt.Errorf("API error: code=%d, msg=%s", wrapper.Code, wrapper.Msg)
	}

	return &wrapper.Data, nil
}

// ToKimiQuotaResponse converts MoonshotQuotaResponse to KimiQuotaResponse
func (m *MoonshotQuotaResponse) ToKimiQuotaResponse() *KimiQuotaResponse {
	percentage := 0
	if m.Data.TotalQuota > 0 {
		percentage = int((m.Data.UsedQuota / m.Data.TotalQuota) * 100)
	}

	return &KimiQuotaResponse{
		Limits: []KimiLimit{
			{
				Type:         "TOKENS_LIMIT",
				Usage:        m.Data.TotalQuota,
				CurrentValue: m.Data.UsedQuota,
				Remaining:    m.Data.Remaining,
				Percentage:   percentage,
			},
		},
	}
}

// ParseMoonshotResponse parses a Moonshot (China) API response from JSON bytes
func ParseMoonshotResponse(data []byte) (*KimiQuotaResponse, error) {
	var resp struct {
		Data struct {
			TotalQuota float64 `json:"totalQuota"`
			UsedQuota  float64 `json:"usedQuota"`
			Remaining  float64 `json:"remaining"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	msResp := MoonshotQuotaResponse{Data: resp.Data}
	return msResp.ToKimiQuotaResponse(), nil
}
