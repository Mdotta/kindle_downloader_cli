package atsu

import (
	"context"
	"fmt"
)

type mangaInfoResponse struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	EnglishTitle string `json:"englishTitle"`
	Poster       struct {
		Image       string `json:"image"`
		SmallImage  string `json:"smallImage"`
		MediumImage string `json:"mediumImage"`
		LargeImage  string `json:"largeImage"`
	} `json:"poster"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Synopsis   string `json:"synopsis"`
	Year       int    `json:"year"`
	Scanlators []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"scanlators"`
	Chapters []struct {
		ID            string  `json:"id"`
		ScanID        string  `json:"scanId"`
		Title         string  `json:"title"`
		Number        float64 `json:"number"`
		PageCount     int     `json:"pageCount"`
		Index         int     `json:"index"`
	} `json:"chapters"`
}

func (c *Client) GetManga(ctx context.Context, id string) (*MangaDetail, error) {
	u := fmt.Sprintf("%s/api/manga/info?mangaId=%s", BaseURL, id)

	resp, err := getJSON[mangaInfoResponse](ctx, c, u)
	if err != nil {
		return nil, err
	}

	p := resp

	detail := &MangaDetail{
		ID:           p.ID,
		Title:        p.Title,
		EnglishTitle: p.EnglishTitle,
		PosterURL:    fullImageURL(p.Poster.Image),
		Type:         p.Type,
		Status:       p.Status,
		Year:         p.Year,
		Synopsis:     p.Synopsis,
	}

	for _, s := range p.Scanlators {
		detail.Scanlators = append(detail.Scanlators, Scanlator{
			ID:   s.ID,
			Name: s.Name,
		})
	}

	for _, ch := range p.Chapters {
		detail.Chapters = append(detail.Chapters, Chapter{
			ID:                ch.ID,
			ScanlationMangaID: ch.ScanID,
			Title:             ch.Title,
			Number:            ch.Number,
			PageCount:         ch.PageCount,
			Index:             ch.Index,
		})
	}

	return detail, nil
}
