package fly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	defaultBaseURL    = "https://api.machines.dev/v1"
	defaultTimeout    = 30 * time.Second
	maxRetries        = 3
	initialBackoff    = 1 * time.Second
	maxBackoff        = 30 * time.Second
	backoffMultiplier = 2.0
)

// Client is an HTTP client for the Fly.io Machines API
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	org        string
	appName    string
}

// New creates a new Fly.io API client
func New(token, org, appName string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
		token:   token,
		org:     org,
		appName: appName,
	}
}

// WithHTTPClient sets a custom HTTP client
func (c *Client) WithHTTPClient(client *http.Client) *Client {
	c.httpClient = client
	return c
}

// WithBaseURL sets a custom base URL (useful for testing)
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL
	return c
}

// request executes an HTTP request with retry logic for rate limiting
func (c *Client) request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	url := c.baseURL + path
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Reset reader for retries
		if body != nil {
			jsonData, _ := json.Marshal(body)
			reqBody = bytes.NewReader(jsonData)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		// Handle rate limiting with exponential backoff
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()

			if attempt == maxRetries {
				return nil, &FlyError{
					StatusCode: http.StatusTooManyRequests,
					Message:    "rate limit exceeded after retries",
				}
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff = time.Duration(math.Min(float64(backoff)*backoffMultiplier, float64(maxBackoff)))
				continue
			}
		}

		// Handle other error status codes
		if resp.StatusCode >= 400 {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)

			var errMsg string
			var errResp struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error != "" {
				errMsg = errResp.Error
			} else {
				errMsg = string(bodyBytes)
			}

			return nil, &FlyError{
				StatusCode: resp.StatusCode,
				Message:    errMsg,
			}
		}

		return resp, nil
	}

	return nil, fmt.Errorf("unexpected retry loop exit")
}

// decodeResponse decodes a JSON response into the target struct
func decodeResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()

	if target == nil {
		// Just drain the body
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
