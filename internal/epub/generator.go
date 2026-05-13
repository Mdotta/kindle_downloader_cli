package epub

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kindle_cli/internal/atsu"

	epublib "github.com/bmaupin/go-epub"
	_ "golang.org/x/image/webp"
)

var imageQuality = 70

func Generate(mangaName string, chapter atsu.Chapter, imageDir, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return fmt.Errorf("reading image dir: %w", err)
	}

	images := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			images = append(images, entry.Name())
		}
	}
	sort.Strings(images)

	if len(images) == 0 {
		return fmt.Errorf("no images found in %s", imageDir)
	}

	title := fmt.Sprintf("%s - %s", mangaName, chapter.Title)
	e := epublib.NewEpub(title)
	e.SetAuthor("Manga")

	var tempFiles []string
	for i, img := range images {
		imgPath := filepath.Join(imageDir, img)

		jpgPath, err := convertToJPEG(imgPath)
		if err != nil {
			return fmt.Errorf("converting %s: %w", img, err)
		}
		if jpgPath != imgPath {
			tempFiles = append(tempFiles, jpgPath)
		}

		internalPath := fmt.Sprintf("page_%03d.jpg", i+1)
		imgSrc, err := e.AddImage(jpgPath, internalPath)

		if err != nil {
			cleanupTempFiles(tempFiles)
			return fmt.Errorf("adding image %s: %w", img, err)
		}

		pageNum := i + 1
		body := fmt.Sprintf(`<html><head><title>Page %d</title><style>@page{margin:0}body{margin:0;padding:0}img{width:100%%;height:auto;display:block}</style></head><body><img src="%s"/></body></html>`,
			pageNum, imgSrc)

		sectionTitle := fmt.Sprintf("Page %d", pageNum)
		if _, err := e.AddSection(body, sectionTitle, "", ""); err != nil {
			cleanupTempFiles(tempFiles)
			return fmt.Errorf("adding section %d: %w", pageNum, err)
		}
	}

	writeErr := e.Write(outputPath)
	cleanupTempFiles(tempFiles)
	return writeErr
}

func GenerateMerged(mangaName string, volumeNum int, chapters []atsu.Chapter, workDir, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	title := fmt.Sprintf("%s - Volume %d", mangaName, volumeNum)
	e := epublib.NewEpub(title)
	e.SetAuthor("Manga")

	var tempFiles []string
	pageNum := 0

	for _, ch := range chapters {
		imageDir := filepath.Join(workDir, sanitizeEPUBName(mangaName), sanitizeEPUBName(fmt.Sprintf("ch_%.1f", ch.Number)))

		entries, err := os.ReadDir(imageDir)
		if err != nil {
			continue // skip chapters with no images
		}

		images := make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				images = append(images, entry.Name())
			}
		}
		sort.Strings(images)

		for _, img := range images {
			imgPath := filepath.Join(imageDir, img)

			jpgPath, err := convertToJPEG(imgPath)
			if err != nil {
				cleanupTempFiles(tempFiles)
				return fmt.Errorf("converting %s/%s: %w", ch.Title, img, err)
			}
			if jpgPath != imgPath {
				tempFiles = append(tempFiles, jpgPath)
			}

			pageNum++
			internalPath := fmt.Sprintf("page_%04d.jpg", pageNum)
			imgSrc, err := e.AddImage(jpgPath, internalPath)
			if err != nil {
				cleanupTempFiles(tempFiles)
				return fmt.Errorf("adding image %s/%s: %w", ch.Title, img, err)
			}

			body := fmt.Sprintf(`<html><head><title>Page %d - %s</title><style>@page{margin:0}body{margin:0;padding:0}img{width:100%%;height:auto;display:block}</style></head><body><img src="%s"/></body></html>`,
				pageNum, ch.Title, imgSrc)

			sectionTitle := fmt.Sprintf("%s - Page %d", ch.Title, pageNum)
			if _, err := e.AddSection(body, sectionTitle, "", ""); err != nil {
				cleanupTempFiles(tempFiles)
				return fmt.Errorf("adding section: %w", err)
			}
		}
	}

	if pageNum == 0 {
		return fmt.Errorf("no images found for any chapter")
	}

	writeErr := e.Write(outputPath)
	cleanupTempFiles(tempFiles)
	return writeErr
}

func sanitizeEPUBName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, s)
}

func cleanupTempFiles(files []string) {
	for _, f := range files {
		os.Remove(f)
	}
}

func convertToJPEG(src string) (string, error) {
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".jpg" || ext == ".jpeg" {
		return src, nil
	}

	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decoding image: %w", err)
	}

	jpgPath := strings.TrimSuffix(src, filepath.Ext(src)) + ".jpg"
	out, err := os.Create(jpgPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: imageQuality}); err != nil {
		return "", fmt.Errorf("encoding jpeg: %w", err)
	}

	return jpgPath, nil
}
