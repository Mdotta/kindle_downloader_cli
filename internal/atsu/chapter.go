package atsu

import (
	"context"
	"fmt"
)

type chapterResponse struct {
	ReadChapter struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Pages []struct {
			ID     string `json:"id"`
			Image  string `json:"image"`
			Number int    `json:"number"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"pages"`
	} `json:"readChapter"`
}

func (c *Client) GetChapterPages(ctx context.Context, mangaID, chapterID string) (*ChapterPages, error) {
	u := fmt.Sprintf("%s/api/read/chapter?mangaId=%s&chapterId=%s", BaseURL, mangaID, chapterID)

	resp, err := getJSON[chapterResponse](ctx, c, u)
	if err != nil {
		return nil, err
	}

	rc := resp.ReadChapter
	result := &ChapterPages{
		ID:    rc.ID,
		Title: rc.Title,
	}

	for _, p := range rc.Pages {
		result.Pages = append(result.Pages, Page{
			ID:       p.ID,
			ImageURL: fullImageURL(p.Image),
			Number:   p.Number,
			Width:    p.Width,
			Height:   p.Height,
		})
	}

	return result, nil
}
