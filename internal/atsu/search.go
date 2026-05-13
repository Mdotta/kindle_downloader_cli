package atsu

import (
	"context"
	"fmt"
	"net/url"
)

type searchResponse struct {
	Found int          `json:"found"`
	Hits  []searchHit  `json:"hits"`
}

type searchHit struct {
	Document struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		EnglishTitle string `json:"englishTitle"`
		Poster       string `json:"poster"`
		PosterMedium string `json:"posterMedium"`
		PosterSmall  string `json:"posterSmall"`
		Type         string `json:"type"`
		IsAdult      bool   `json:"isAdult"`
		Status       string `json:"status"`
		Year         int    `json:"year"`
	} `json:"document"`
}

func (c *Client) Search(ctx context.Context, query string, page int, perPage int) ([]Manga, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 12
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("query_by", "title,englishTitle,otherNames,authors")
	params.Set("query_by_weights", "4,3,2,1")
	params.Set("num_typos", "4,3,2,1")
	params.Set("include_fields", "id,title,englishTitle,poster,posterSmall,posterMedium,type,isAdult,status,year")
	params.Set("filter_by", "views:>0")
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("per_page", fmt.Sprintf("%d", perPage))

	u := fmt.Sprintf("%s/collections/manga/documents/search?%s", BaseURL, params.Encode())

	resp, err := getJSON[searchResponse](ctx, c, u)
	if err != nil {
		return nil, 0, err
	}

	mangas := make([]Manga, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		d := hit.Document
		mangas = append(mangas, Manga{
			ID:           d.ID,
			Title:        d.Title,
			EnglishTitle: d.EnglishTitle,
			PosterURL:    fullImageURL(d.Poster),
			Type:         d.Type,
			Status:       d.Status,
			Year:         d.Year,
		})
	}
	return mangas, resp.Found, nil
}
