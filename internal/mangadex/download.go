package mangadex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// --- at-home response ---

type atHomeResponse struct {
	Result  string        `json:"result"`
	BaseURL string        `json:"baseUrl"`
	Chapter atHomeChapter `json:"chapter"`
}

type atHomeChapter struct {
	Hash      string   `json:"hash"`
	Data      []string `json:"data"`
	DataSaver []string `json:"dataSaver"`
}

// ChapterPages holds the resolved download URLs for a single chapter.
type ChapterPages struct {
	BaseURL string
	Hash    string
	Pages   []string // high-quality filenames
}

// DownloadResult is sent to the caller for each downloaded page.
type DownloadResult struct {
	Volume  string
	Chapter string
	Page    int    // 1-based page index
	Total   int    // total pages in this chapter
	File    string // local file path
	Done    bool   // true when a chapter finishes (sent once per chapter)
	Error   error  // non-nil if this page failed
}

// --- helpers ---

type chapterWork struct {
	Volume string
	Ch     Chapter
}

func sanitizePath(s string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(s)
}

func fileExt(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ""
}

// --- API call ---

func (c *Client) GetChapterPages(ctx context.Context, chapterID string) (*ChapterPages, error) {
	path := "/at-home/server/" + chapterID
	body, err := c.RequestAtHome(ctx, "GET", path)
	if err != nil {
		return nil, err
	}

	var resp atHomeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing at-home response: %w", err)
	}
	if resp.Result != "ok" {
		return nil, fmt.Errorf("at-home returned result=%q", resp.Result)
	}

	return &ChapterPages{
		BaseURL: resp.BaseURL,
		Hash:    resp.Chapter.Hash,
		Pages:   resp.Chapter.Data,
	}, nil
}

// --- image download ---

func (c *Client) downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header = c.headers()

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("download %s: HTTP %d — %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", dest, err)
	}
	return nil
}

// --- volume download pipeline ---

func (c *Client) DownloadVolumes(
	ctx context.Context,
	mangaName string,
	volumes map[string][]Chapter,
	workDir string,
	concurrency int,
) <-chan DownloadResult {
	if concurrency < 1 {
		concurrency = 1
	}

	results := make(chan DownloadResult, concurrency*2)

	go func() {
		defer close(results)

		// flatten chapters into a work queue
		work := make(chan chapterWork)
		var totalChapters int

		go func() {
			for vol, chapters := range volumes {
				for _, ch := range chapters {
					select {
					case work <- chapterWork{Volume: vol, Ch: ch}:
						totalChapters++
					case <-ctx.Done():
						close(work)
						return
					}
				}
			}
			close(work)
		}()

		// launch workers
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for w := range work {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			case sem <- struct{}{}:
			}

			wg.Add(1)
			go func(cw chapterWork) {
				defer wg.Done()
				defer func() { <-sem }()

				c.downloadChapter(ctx, mangaName, cw.Volume, cw.Ch, workDir, results)
			}(w)
		}
		wg.Wait()
	}()

	return results
}

func (c *Client) downloadChapter(
	ctx context.Context,
	mangaName, volumeName string,
	ch Chapter,
	workDir string,
	results chan<- DownloadResult,
) {
	pages, err := c.GetChapterPages(ctx, ch.ID)
	if err != nil {
		results <- DownloadResult{
			Volume:  volumeName,
			Chapter: ch.Chapter,
			Done:    true,
			Error:   fmt.Errorf("fetching pages for chapter %s: %w", ch.Chapter, err),
		}
		return
	}

	dir := filepath.Join(workDir, sanitizePath(mangaName), sanitizePath(volumeName))
	total := len(pages.Pages)

	for i, filename := range pages.Pages {
		select {
		case <-ctx.Done():
			return
		default:
		}

		url := fmt.Sprintf("%s/data/%s/%s", pages.BaseURL, pages.Hash, filename)
		pageNum := i + 1
		dest := filepath.Join(dir, fmt.Sprintf("ch_%s_pg%03d%s", sanitizePath(ch.Chapter), pageNum, fileExt(filename)))

		dlErr := c.downloadFile(ctx, url, dest)

		results <- DownloadResult{
			Volume:  volumeName,
			Chapter: ch.Chapter,
			Page:    pageNum,
			Total:   total,
			File:    dest,
			Error:   dlErr,
		}
	}

	results <- DownloadResult{
		Volume:  volumeName,
		Chapter: ch.Chapter,
		Page:    total,
		Total:   total,
		Done:    true,
	}
}
