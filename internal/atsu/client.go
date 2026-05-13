package atsu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const BaseURL = "https://atsu.moe"

type Client struct {
	http               *http.Client
	PreferredLanguages []string
}

// Manga represents a search result.
type Manga struct {
	ID           string
	Title        string
	EnglishTitle string
	PosterURL    string
	Type         string
	Status       string
	Year         int
}

// MangaDetail includes metadata and chapter list.
type MangaDetail struct {
	ID           string
	Title        string
	EnglishTitle string
	PosterURL    string
	Type         string
	Status       string
	Year         int
	Synopsis     string
	Chapters     []Chapter
	Scanlators   []Scanlator
}

// Scanlator represents a translation group.
type Scanlator struct {
	ID   string
	Name string
}

// Chapter represents a single chapter.
type Chapter struct {
	ID                string
	ScanlationMangaID string
	Title             string
	Number            float64
	PageCount         int
	Index             int
	// Volume is inferred from chapter grouping (atsu doesn't have explicit volumes)
}

// ChapterPages contains a chapter's image pages.
type ChapterPages struct {
	ID     string
	Title  string
	Pages  []Page
}

// Page represents a single page image.
type Page struct {
	ID          string
	ImageURL    string
	Number      int
	Width       int
	Height      int
}

func NewClient(languages []string) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		PreferredLanguages: languages,
	}
}

func (c *Client) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

func getJSON[T any](ctx context.Context, c *Client, url string) (*T, error) {
	body, err := c.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &result, nil
}

// fullImageURL constructs a full image URL from a relative path.
func fullImageURL(path string) string {
	if path == "" {
		return ""
	}
	return BaseURL + path
}
