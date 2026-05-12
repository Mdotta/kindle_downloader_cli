package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// --- endpoint-specific types ---

type mangaData struct {
	ID         string          `json:"id"`
	Attributes mangaAttributes `json:"attributes"`
	Relations  []relation      `json:"relationships"`
}

type mangaAttributes struct {
	Title     localeMap   `json:"title"`
	AltTitles []localeMap `json:"altTitles"`
	Status    string      `json:"status"`
	Tags      []struct {
		Attributes struct {
			Name localeMap `json:"name"`
		} `json:"attributes"`
	} `json:"tags"`
}

type localeMap map[string]string

type relation struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type chapterData struct {
	ID         string            `json:"id"`
	Attributes chapterAttributes `json:"attributes"`
	Relations  []relation        `json:"relationships"`
}

type chapterAttributes struct {
	Volume             string `json:"volume"`
	Chapter            string `json:"chapter"`
	Title              string `json:"title"`
	TranslatedLanguage string `json:"translatedLanguage"`
}

type errorResponse struct {
	Errors []APIError `json:"errors"`
}

// --- search ---

func (c *Client) SearchManga(ctx context.Context, query string) ([]Manga, error) {
	params := url.Values{}
	params.Set("title", query)
	params.Set("limit", "20")
	params.Add("includes[]", "cover_art")
	params.Add("contentRating[]", "safe")
	params.Add("contentRating[]", "suggestive")

	path := "/manga?" + params.Encode()
	body, err := c.Request(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	fmt.Println("\npath:", path)

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if resp.Result == "error" {
		return nil, parseAPIErrors(body)
	}

	var data []mangaData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing manga data: %w", err)
	}

	mangas := make([]Manga, 0, len(data))
	for _, d := range data {
		mangas = append(mangas, toManga(d))
	}
	return mangas, nil
}

// --- chapter feed ---

func (c *Client) GetChapters(ctx context.Context, mangaID string) (map[string][]Chapter, error) {
	params := url.Values{}
	params.Add("translatedLanguage[]", c.preferredLanguage)
	params.Add("order[volume]", "desc")
	params.Add("order[chapter]", "desc")
	params.Set("limit", "100")

	path := "/chapter?" + params.Encode() + "&manga=" + mangaID
	body, err := c.Request(ctx, "GET", path)
	if err != nil {
		return nil, err
	}
	fmt.Println("\npath:", path)

	var resp APIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if resp.Result == "error" {
		return nil, parseAPIErrors(body)
	}

	var data []chapterData
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing chapter data: %w", err)
	}

	volumes := make(map[string][]Chapter)
	for _, d := range data {
		vol := d.Attributes.Volume
		if vol == "" {
			vol = "No Volume"
		}
		ch := Chapter{
			ID:       d.ID,
			Volume:   vol,
			Chapter:  d.Attributes.Chapter,
			Title:    d.Attributes.Title,
			Language: d.Attributes.TranslatedLanguage,
			Group:    extractGroup(d.Relations),
		}
		volumes[vol] = append(volumes[vol], ch)
	}
	return volumes, nil
}

// --- helpers ---

func parseAPIErrors(body []byte) error {
	var errResp errorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("api error (unparsable): %s", string(body))
	}
	msgs := make([]string, len(errResp.Errors))
	for i, e := range errResp.Errors {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("api errors: %s", strings.Join(msgs, "; "))
}

func toManga(d mangaData) Manga {
	return Manga{
		ID:        d.ID,
		Title:     pickTitle(d.Attributes.Title, "en"),
		AltTitles: flattenLocales(d.Attributes.AltTitles),
		Status:    d.Attributes.Status,
		CoverURL:  extractCoverURL(d.ID, d.Relations),
		Tags:      extractTags(d.Attributes.Tags),
	}
}

func pickTitle(loc localeMap, preferred string) string {
	if s, ok := loc[preferred]; ok {
		return s
	}
	for _, v := range loc {
		return v
	}
	return ""
}

func flattenLocales(locales []localeMap) []string {
	out := make([]string, 0, len(locales))
	for _, loc := range locales {
		for _, v := range loc {
			out = append(out, v)
		}
	}
	return out
}

func extractCoverURL(mangaID string, rels []relation) string {
	for _, rel := range rels {
		if rel.Type == "cover_art" {
			var attrs struct {
				FileName string `json:"fileName"`
			}
			if err := json.Unmarshal(rel.Attributes, &attrs); err == nil && attrs.FileName != "" {
				return fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s", mangaID, attrs.FileName)
			}
		}
	}
	return ""
}

func extractTags(tags []struct {
	Attributes struct {
		Name localeMap `json:"name"`
	} `json:"attributes"`
}) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, pickTitle(t.Attributes.Name, "en"))
	}
	return out
}

func extractGroup(rels []relation) string {
	for _, rel := range rels {
		if rel.Type == "scanlation_group" {
			var attrs struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(rel.Attributes, &attrs); err == nil && attrs.Name != "" {
				return attrs.Name
			}
		}
	}
	return ""
}
