// Package api provides clients for interacting with the Kimi API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Custom errors for Kimi API failures.
var (
	ErrKimiUnauthorized    = errors.New("kimi: unauthorized - invalid API key")
	ErrKimiServerError     = errors.New("kimi: server error")
	ErrKimiNetworkError    = errors.New("kimi: network error")
	ErrKimiInvalidResponse = errors.New("kimi: invalid response")
	ErrKimiAPIError        = errors.New("kimi: API returned error")
)

// KimiRegion constants
const (
	KimiRegionInternational = "international"
	KimiRegionChina         = "china"
)

// Default endpoints for each region
const (
	// Kimi International (coding) - usages endpoint (experimental)
	KimiInternationalEndpoint = "https://api.kimi.com/coding/v1/usages"
	// Moonshot China
	KimiChinaEndpoint  = "https://api.moonshot.cn/v1/users/me/balance"
	KimiChinaEndpoint2 = "https://api.moonshot.cn/v1/users/me/billing/quota"
)

// KimiClient is an HTTP client for the Kimi API.
type KimiClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	region     string // "international" or "china"
	logger     *slog.Logger
}

// KimiOption configures a KimiClient.
type KimiOption func(*KimiClient)

// WithKimiBaseURL sets a custom base URL (for testing).
func WithKimiBaseURL(url string) KimiOption {
	return func(c *KimiClient) {
		c.baseURL = url
	}
}

// WithKimiTimeout sets a custom timeout (for testing).
func WithKimiTimeout(timeout time.Duration) KimiOption {
	return func(c *KimiClient) {
		c.httpClient.Timeout = timeout
	}
}

// WithKimiRegion sets the region (international or china).
func WithKimiRegion(region string) KimiOption {
	return func(c *KimiClient) {
		c.region = region
	}
}

// NewKimiClient creates a new Kimi API client.
// If baseURL is empty, it will be set based on the region.
func NewKimiClient(apiKey string, baseURL string, region string, logger *slog.Logger, opts ...KimiOption) *KimiClient {
	// Normalize region
	if region == "" {
		region = KimiRegionInternational
	}
	region = strings.ToLower(region)

	// Normalize region aliases
	if region == "cn" || region == "domestic" {
		region = KimiRegionChina
	}

	// Set default base URL based on region if not provided
	if baseURL == "" {
		if region == KimiRegionChina {
			baseURL = KimiChinaEndpoint
		} else {
			baseURL = KimiInternationalEndpoint
		}
	}

	client := &KimiClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          1,
				MaxIdleConnsPerHost:   1,
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ForceAttemptHTTP2:     true,
			},
		},
		apiKey:  apiKey,
		baseURL: baseURL,
		region:  region,
		logger:  logger,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// FetchQuotas retrieves the current quota information from the Kimi API.
// It tries multiple endpoints if the primary one returns 404.
func (c *KimiClient) FetchQuotas(ctx context.Context) (*KimiQuotaResponse, error) {
	// Get list of endpoints to try based on region
	endpoints := c.getEndpointsToTry()

	var lastErr error
	for i, endpoint := range endpoints {
		quotaResp, err := c.tryEndpoint(ctx, endpoint)
		if err == nil {
			return quotaResp, nil
		}

		// Log the failed attempt
		c.logger.Debug("Kimi endpoint failed",
			"endpoint", endpoint,
			"attempt", i+1,
			"error", err,
		)

		// If unauthorized, don't try other endpoints
		if errors.Is(err, ErrKimiUnauthorized) {
			return nil, err
		}

		lastErr = err

		// Only try next endpoint if we got 404 (endpoint not found)
		if !isEndpointNotFoundError(err) {
			break
		}
	}

	return nil, fmt.Errorf("kimi: all endpoints failed for region %s: %w", c.region, lastErr)
}

// getEndpointsToTry returns the list of endpoints to try based on region
func (c *KimiClient) getEndpointsToTry() []string {
	// If user specified a custom URL, only try that
	if c.baseURL != KimiInternationalEndpoint && c.baseURL != KimiChinaEndpoint &&
		c.baseURL != KimiChinaEndpoint2 {
		return []string{c.baseURL}
	}

	if c.region == KimiRegionChina {
		return []string{
			KimiChinaEndpoint,  // Primary: balance endpoint
			KimiChinaEndpoint2, // Fallback: billing/quota endpoint
		}
	}

	// International endpoint (usages)
	return []string{KimiInternationalEndpoint}
}

// isEndpointNotFoundError checks if the error indicates a 404 response
func isEndpointNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "404") ||
		strings.Contains(err.Error(), "unexpected status code 404")
}

// tryEndpoint attempts to fetch quotas from a specific endpoint
func (c *KimiClient) tryEndpoint(ctx context.Context, endpoint string) (*KimiQuotaResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("kimi: creating request: %w", err)
	}

	// Set headers based on region
	// International Kimi uses Bearer token, China Moonshot uses Bearer token too
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "onwatch/1.0")
	req.Header.Set("Accept", "application/json")

	// Log request (with redacted API key)
	c.logger.Debug("fetching Kimi quotas",
		"url", endpoint,
		"region", c.region,
		"api_key", redactKimiAPIKey(c.apiKey),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", ErrKimiNetworkError, err)
	}
	defer resp.Body.Close()

	// Log response status
	c.logger.Debug("Kimi quota response received",
		"status", resp.StatusCode,
		"url", endpoint,
	)

	// Read response body (bounded to 64KB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("%w: reading body: %v", ErrKimiInvalidResponse, err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("%w: empty response body", ErrKimiInvalidResponse)
	}

	// Handle HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Continue parsing
	case http.StatusUnauthorized:
		return nil, ErrKimiUnauthorized
	case http.StatusForbidden:
		return nil, ErrKimiUnauthorized
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return nil, fmt.Errorf("%w: status %d", ErrKimiServerError, resp.StatusCode)
	default:
		// Log the response body for debugging
		bodySnippet := strings.TrimSpace(string(body))
		if len(bodySnippet) > 200 {
			bodySnippet = bodySnippet[:200] + "..."
		}
		c.logger.Warn("Kimi API returned unexpected status",
			"status", resp.StatusCode,
			"url", endpoint,
			"body", bodySnippet,
		)
		// Try to parse error message from response body
		var errResp KimiResponse[interface{}]
		if json.Unmarshal(body, &errResp) == nil && !errResp.Success && errResp.Msg != "" {
			return nil, fmt.Errorf("%w: %s", ErrKimiAPIError, errResp.Msg)
		}
		return nil, fmt.Errorf("kimi: unexpected status code %d from %s", resp.StatusCode, endpoint)
	}

	// Parse response based on region
	var quotaResp *KimiQuotaResponse

	if c.region == KimiRegionChina {
		// Try Moonshot format first, then fall back to Kimi format
		quotaResp, err = ParseMoonshotResponse(body)
		if err != nil {
			// Fall back to Kimi usages format
			quotaResp, err = ParseKimiUsagesResponse(body)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrKimiInvalidResponse, err)
			}
		}
	} else {
		// International Kimi - use usages format first
		quotaResp, err = ParseKimiUsagesResponse(body)
		if err != nil {
			// Try legacy Kimi format as fallback
			quotaResp, err = ParseKimiResponse(body)
			if err != nil {
				// Try Moonshot format as last resort
				quotaResp, err = ParseMoonshotResponse(body)
				if err != nil {
					return nil, fmt.Errorf("%w: %v", ErrKimiInvalidResponse, err)
				}
			}
		}
	}

	// Log usage info if we have limits
	if len(quotaResp.Limits) > 0 {
		timeUsage := float64(0)
		tokensUsage := float64(0)
		for _, limit := range quotaResp.Limits {
			if limit.Type == "TIME_LIMIT" {
				timeUsage = limit.CurrentValue
			} else if limit.Type == "TOKENS_LIMIT" {
				tokensUsage = limit.CurrentValue
			}
		}
		c.logger.Debug("Kimi quotas fetched successfully",
			"region", c.region,
			"endpoint", endpoint,
			"time_usage", timeUsage,
			"tokens_usage", tokensUsage,
		)
	}

	return quotaResp, nil
}

// redactKimiAPIKey masks the API key for logging.
func redactKimiAPIKey(key string) string {
	if key == "" {
		return "(empty)"
	}

	if len(key) < 12 {
		return "***...***"
	}

	// Show first 7 chars (sk-kimi-) and last 3 chars
	return key[:7] + "***...***" + key[len(key)-3:]
}
