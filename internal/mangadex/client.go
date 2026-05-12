package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// --- API envelope types ---

type APIResponse struct {
	Result   string          `json:"result"`   // "ok" or "error"
	Response string          `json:"response"` // "collection" or "entity"
	Data     json.RawMessage `json:"data"`
	Errors   json.RawMessage `json:"errors,omitempty"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
	Total    int             `json:"total"`
}

type APIError struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("mangadex: %s (status %d): %s", e.Title, e.Status, e.Detail)
}

// --- Client ---

type Client struct {
	http              *http.Client
	generalRL         *rate.Limiter // 5 req/s for most endpoints
	atHomeRL          *rate.Limiter // 40 req/min for /at-home/server
	baseURL           string
	preferredLanguage string
}

type Manga struct {
	ID        string
	Title     string
	AltTitles []string
	Status    string
	CoverURL  string
	Tags      []string
}

type Chapter struct {
	ID       string
	Volume   string
	Chapter  string
	Title    string
	Language string
	Group    string
}

func NewClient(baseUrl string, preferredLanguage string) *Client {
	return &Client{
		http:              &http.Client{Timeout: 30 * time.Second},
		generalRL:         rate.NewLimiter(rate.Limit(5), 1),
		atHomeRL:          rate.NewLimiter(rate.Limit(40.0/60.0), 1),
		baseURL:           baseUrl,
		preferredLanguage: preferredLanguage,
	}
}

func (c *Client) headers() http.Header {
	return http.Header{
		"User-Agent": []string{"kindle-cli/1.0"},
		"Accept":     []string{"application/json"},
	}
}

// request performs an HTTP request with the specified rate limiter.
// Pass nil to use the general (5 req/s) limiter.
func (c *Client) request(ctx context.Context, method, path string, rl *rate.Limiter) ([]byte, error) {
	if rl == nil {
		rl = c.generalRL
	}
	if err := rl.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header = c.headers()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited (retry after %s)", resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Status > 0 {
			return nil, apiErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Request performs a request using the general rate limiter.
func (c *Client) Request(ctx context.Context, method, path string) ([]byte, error) {
	return c.request(ctx, method, path, nil)
}

// RequestAtHome performs a request using the at-home rate limiter (40 req/min).
func (c *Client) RequestAtHome(ctx context.Context, method, path string) ([]byte, error) {
	return c.request(ctx, method, path, c.atHomeRL)
}
