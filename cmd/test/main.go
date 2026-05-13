package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"kindle_cli/internal/atsu"
)

func main() {
	fmt.Println("=== Atsu.moe Client Test ===")
	fmt.Println()

	client := atsu.NewClient([]string{"en"})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ── Step 1: Search ──
	query := "Solo Leveling"
	fmt.Printf("[1] Searching for '%s'... ", query)
	mangas, total, err := client.Search(ctx, query, 1, 10)
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	if len(mangas) == 0 {
		fmt.Println("FAIL\n    no results")
		os.Exit(1)
	}
	fmt.Printf("OK (%d results)\n", total)
	for _, m := range mangas[:min(3, len(mangas))] {
		fmt.Printf("    %s [%s]  %s  %s\n", m.Title, m.ID, m.Type, m.Status)
	}

	// ── Step 2: Get manga detail ──
	manga := mangas[0]
	fmt.Printf("\n[2] Getting detail for '%s' (%s)... ", manga.Title, manga.ID)
	detail, err := client.GetManga(ctx, manga.ID)
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d chapters, %d scanlators)\n", len(detail.Chapters), len(detail.Scanlators))
	for _, s := range detail.Scanlators {
		fmt.Printf("    Scanlator: %s (%s)\n", s.Name, s.ID)
	}
	for _, ch := range detail.Chapters[:min(5, len(detail.Chapters))] {
		fmt.Printf("    Ch. %s [%s] (%d pages)\n", ch.Title, ch.ID, ch.PageCount)
	}

	// ── Step 3: Get chapter pages ──
	if len(detail.Chapters) == 0 {
		fmt.Println("\n[3] No chapters to test")
		return
	}
	ch := detail.Chapters[0]
	fmt.Printf("\n[3] Getting pages for '%s' (mangaId=%s, chapterId=%s)... ", ch.Title, manga.ID, ch.ID)
	pages, err := client.GetChapterPages(ctx, manga.ID, ch.ID)
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d pages)\n", len(pages.Pages))
	for _, p := range pages.Pages[:min(3, len(pages.Pages))] {
		fmt.Printf("    [%d] %s (%dx%d)\n", p.Number, p.ImageURL, p.Width, p.Height)
	}

	// ── Step 4: Download images ──
	fmt.Printf("\n[4] Downloading first 5 images...\n")
	tmpDir, err := os.MkdirTemp("", "kindle_cli_test")
	if err != nil {
		fmt.Printf("    FAIL: creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	ok := 0
	for i, p := range pages.Pages {
		if i >= 5 {
			break
		}
		dest := filepath.Join(tmpDir, fmt.Sprintf("pg%03d.webp", p.Number))
		if err := downloadFile(ctx, p.ImageURL, dest); err != nil {
			fmt.Printf("    FAIL  pg %d: %v\n", p.Number, err)
		} else {
			ok++
			fmt.Printf("    OK    pg %d → %s\n", p.Number, filepath.Base(dest))
		}
	}

	if ok == 0 {
		fmt.Println("\nFAIL: no images downloaded")
		os.Exit(1)
	}
	fmt.Printf("\n=== All tests passed (%d images downloaded to %s) ===\n", ok, tmpDir)
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://atsu.moe/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
