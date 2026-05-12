package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kindle_cli/internal/mangadex"
)

func main() {
	fmt.Println("=== MangaDex Client Test ===")
	fmt.Println()

	client := mangadex.NewClient("https://api.mangadex.org", "en")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ── Step 1: Search ──
	fmt.Print("[1] Searching for 'frieren'... ")
	mangas, err := client.SearchManga(ctx, "Sousou no Frieren")
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	if len(mangas) == 0 {
		fmt.Println("FAIL\n    no results")
		os.Exit(1)
	}
	fmt.Println("OK")
	for _, m := range mangas[:min(3, len(mangas))] {
		fmt.Printf("    %s [%s]  %v\n", m.Title, m.ID, m.CoverURL)
	}

	// ── Step 2: Get chapters ──
	manga := mangas[0]
	fmt.Printf("\n[2] Getting chapters for '%s' (%s)... ", manga.Title, manga.ID)
	volumes, err := client.GetChapters(ctx, manga.ID)
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d volumes)\n", len(volumes))
	for vol, chs := range volumes {
		fmt.Printf("    Vol. %s: %d chapters\n", vol, len(chs))
	}

	// ── Step 3: Get chapter pages ──
	var firstID string
	var firstVol string
	var firstCh string
	for vol, chs := range volumes {
		if vol == "No Volume" {
			continue
		}
		if len(chs) > 0 {
			firstVol = vol
			firstID = chs[0].ID
			firstCh = chs[0].Chapter
			break
		}
	}
	if firstID == "" {
		fmt.Println("\n[3] No volumed chapters to test — skipping download tests")
		return
	}

	fmt.Printf("\n[3] Getting pages for Vol.%s Ch.%s... ", firstVol, firstCh)
	pages, err := client.GetChapterPages(ctx, firstID)
	if err != nil {
		fmt.Printf("FAIL\n    %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%d pages)\n", len(pages.Pages))
	fmt.Printf("    baseUrl: %s\n    hash: %s\n", pages.BaseURL, pages.Hash)
	for i, p := range pages.Pages[:min(3, len(pages.Pages))] {
		fmt.Printf("    [%d] %s\n", i+1, p)
	}

	// ── Step 4: Download a single chapter ──
	fmt.Printf("\n[4] Downloading Vol.%s Ch.%s (%d pages)...\n", firstVol, firstCh, len(pages.Pages))
	tmpDir, err := os.MkdirTemp("", "kindle_cli_test")
	if err != nil {
		fmt.Printf("    FAIL: creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Only download the first chapter of the first volume
	singleVolume := map[string][]mangadex.Chapter{
		firstVol: volumes[firstVol][:1],
	}

	results := client.DownloadVolumes(ctx, manga.Title, singleVolume, tmpDir, 2)
	ok := 0
	failed := 0
	for r := range results {
		if r.Error != nil {
			failed++
			fmt.Printf("    FAIL  pg %d/%d: %v\n", r.Page, r.Total, r.Error)
		} else if r.Done {
			fmt.Printf("    DONE  Vol.%s Ch.%s — %d pages, %d failed\n", r.Volume, r.Chapter, r.Total, failed)
		} else {
			ok++
			fmt.Printf("    OK    pg %d/%d => %s\n", r.Page, r.Total, filepath.Base(r.File))
		}
	}

	if failed > 0 {
		fmt.Printf("\nFAIL: %d/%d pages failed\n", failed, ok+failed)
		os.Exit(1)
	}
	fmt.Printf("\n=== All tests passed (%d pages downloaded) ===\n", ok)
}
