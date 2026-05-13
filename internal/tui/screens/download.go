package screens

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"kindle_cli/internal/atsu"
	"kindle_cli/internal/cleanup"
	"kindle_cli/internal/config"
	"kindle_cli/internal/email"
	"kindle_cli/internal/epub"
	"kindle_cli/pkg/screenstack"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Model ---

type downloadState int

const (
	stateConfirming downloadState = iota
	stateDownloading
	stateGenerating
	stateSending
	stateDone
	stateCancelled
	stateError
)

type DownloadScreen struct {
	config    *config.Config
	client    *atsu.Client
	mangaName string
	mangaID   string
	chapters  []atsu.Chapter

	volumeNum     int
	groupChapters bool

	state   downloadState
	spinner spinner.Model

	// Progress
	currentChapter  int
	currentPage     int
	currentTotal    int
	totalPages      int
	downloadedPages int
	errors          int

	// Pipeline
	pipelineIdx   int
	pipelineTotal int
	pipelineLabel string
	successCount  int
	failCount     int
	pipelineError string
	chapterErrors []string

	// Results channel
	results chan tea.Msg

	// Cancellation
	cancel context.CancelFunc

	// Output
	workDir string

	width  int
	height int
}

func NewDownloadScreen(cfg *config.Config, client *atsu.Client, mangaName, mangaID string, chapters []atsu.Chapter) *DownloadScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	var total int
	for _, ch := range chapters {
		total += ch.PageCount
	}

	return &DownloadScreen{
		config:     cfg,
		client:     client,
		mangaName:  mangaName,
		mangaID:    mangaID,
		chapters:   chapters,
		state:      stateConfirming,
		spinner:    sp,
		totalPages: total,
		results:    make(chan tea.Msg, 100),
	}
}

// --- Messages ---

type downloadProgressMsg struct {
	ChapterIdx  int
	ChapterName string
	PageNum     int
	TotalPages  int
	File        string
	Err         error
}

type pipelineProgressMsg struct {
	Idx   int
	Total int
	Label string
	Err   error
}

type pipelineDoneMsg struct {
	Success int
	Failed  int
	Err     error
}

type downloadStartMsg struct {
	TotalPages int
	Errors     int
}

// --- Tea.Model ---

func (d *DownloadScreen) Init() tea.Cmd {
	return tea.Batch(tea.WindowSize(), d.spinner.Tick)
}

func (d *DownloadScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		return d, nil

	case spinner.TickMsg:
		if d.state == stateDownloading || d.state == stateGenerating || d.state == stateSending {
			var cmd tea.Cmd
			d.spinner, cmd = d.spinner.Update(msg)
			return d, cmd
		}

	case tea.KeyMsg:
		return d.handleKey(msg)

	case downloadProgressMsg:
		return d.handleDownloadProgress(msg)

	case downloadStartMsg:
		// Download goroutine is done; start the pipeline
		d.downloadedPages = msg.TotalPages
		d.results = make(chan tea.Msg, 100) // new channel before goroutine starts
		go d.runPipeline()
		return d, d.waitForResult()

	case pipelineProgressMsg:
		return d.handlePipelineProgress(msg)

	case pipelineDoneMsg:
		return d.handlePipelineDone(msg)
	}
	return d, nil
}

// --- Key handling ---

func (d *DownloadScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch d.state {
	case stateConfirming:
		switch msg.String() {
		case "enter":
			d.state = stateDownloading
			ctx, cancel := context.WithCancel(context.Background())
			d.cancel = cancel
			go d.downloadAll(ctx)
			return d, tea.Batch(d.spinner.Tick, d.waitForResult())

		case "esc":
			return d, screenstack.PopCmd()
		}

	case stateDownloading:
		if msg.String() == "esc" {
			d.cancel()
			d.state = stateCancelled
			return d, nil
		}

	case stateGenerating, stateSending:
		// Can't cancel during pipeline
		return d, nil

	case stateDone, stateCancelled, stateError:
		switch msg.String() {
		case "enter", "esc":
			return d, screenstack.BackToRootCmd()
		}
	}
	return d, nil
}

// --- Download ---

func (d *DownloadScreen) handleDownloadProgress(msg downloadProgressMsg) (tea.Model, tea.Cmd) {
	d.downloadedPages++
	d.currentPage = msg.PageNum
	d.currentTotal = msg.TotalPages
	if msg.Err != nil {
		d.errors++
	}
	return d, d.waitForResult()
}

func (d *DownloadScreen) downloadAll(ctx context.Context) {
	dir, err := os.MkdirTemp("", "kindle_cli_*")
	if err != nil {
		d.results <- downloadStartMsg{Errors: 1}
		close(d.results)
		return
	}
	d.workDir = dir

	totalDownloaded := 0
	totalErrors := 0

	for i, ch := range d.chapters {
		select {
		case <-ctx.Done():
			d.results <- downloadStartMsg{TotalPages: totalDownloaded, Errors: totalErrors}
			close(d.results)
			return
		default:
		}

		pages, err := d.client.GetChapterPages(ctx, d.mangaID, ch.ID)
		if err != nil {
			totalErrors++
			d.results <- downloadProgressMsg{
				ChapterIdx:  i + 1,
				ChapterName: ch.Title,
				Err:         fmt.Errorf("fetching chapter: %w", err),
			}
			continue
		}

		chapterDir := filepath.Join(dir, sanitizeName(d.mangaName), sanitizeName(fmt.Sprintf("ch_%.1f", ch.Number)))
		if err := os.MkdirAll(chapterDir, 0755); err != nil {
			totalErrors++
			d.results <- downloadProgressMsg{
				ChapterIdx:  i + 1,
				ChapterName: ch.Title,
				Err:         fmt.Errorf("creating directory: %w", err),
			}
			continue
		}

		for pi, page := range pages.Pages {
			select {
			case <-ctx.Done():
				d.results <- downloadStartMsg{TotalPages: totalDownloaded, Errors: totalErrors}
				close(d.results)
				return
			default:
			}

			ext := filepath.Ext(page.ImageURL)
			if ext == "" {
				ext = ".webp"
			}
			dest := filepath.Join(chapterDir, fmt.Sprintf("pg%03d%s", pi+1, ext))

			dlErr := downloadImage(ctx, page.ImageURL, dest)
			if dlErr != nil {
				totalErrors++
			} else {
				totalDownloaded++
			}

			d.results <- downloadProgressMsg{
				ChapterIdx:  i + 1,
				ChapterName: ch.Title,
				PageNum:     pi + 1,
				TotalPages:  len(pages.Pages),
				File:        dest,
				Err:         dlErr,
			}
		}
	}

	d.results <- downloadStartMsg{TotalPages: totalDownloaded, Errors: totalErrors}
	close(d.results)
}

// --- Pipeline (EPUB → email → cleanup) ---

func (d *DownloadScreen) runPipeline() {
	d.state = stateGenerating
	epubDir := filepath.Join(d.workDir, "epub")
	if err := os.MkdirAll(epubDir, 0755); err != nil {
		d.results <- pipelineDoneMsg{Err: fmt.Errorf("creating epub dir: %w", err)}
		close(d.results)
		return
	}

	if d.groupChapters {
		d.runGroupedPipeline(epubDir)
	} else {
		d.runPerChapterPipeline(epubDir)
	}
}

func (d *DownloadScreen) runPerChapterPipeline(epubDir string) {
	generated := 0
	for i, ch := range d.chapters {
		imageDir := filepath.Join(d.workDir, sanitizeName(d.mangaName), sanitizeName(fmt.Sprintf("ch_%.1f", ch.Number)))

		if _, err := os.Stat(imageDir); os.IsNotExist(err) {
			d.results <- pipelineProgressMsg{
				Idx: i + 1, Total: len(d.chapters),
				Label: fmt.Sprintf("Generating EPUB %d/%d: %s", i+1, len(d.chapters), ch.Title),
				Err:   fmt.Errorf("no images found"),
			}
			continue
		}

		epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_ch%.1f.epub")
		err := epub.Generate(d.mangaName, ch, imageDir, epubPath)
		d.results <- pipelineProgressMsg{
			Idx: i + 1, Total: len(d.chapters),
			Label: fmt.Sprintf("Generating EPUB %d/%d: %s", i+1, len(d.chapters), ch.Title),
			Err:   err,
		}
		if err == nil {
			generated++
		}
	}

	if generated == 0 {
		cleanup.Dir(d.workDir)
		d.results <- pipelineDoneMsg{Err: fmt.Errorf("no EPUBs generated")}
		close(d.results)
		return
	}

	d.runSendSavePhase(epubDir)
}

func (d *DownloadScreen) runGroupedPipeline(epubDir string) {
	imageDirs := make([]string, 0, len(d.chapters))
	for _, ch := range d.chapters {
		imageDir := filepath.Join(d.workDir, sanitizeName(d.mangaName), sanitizeName(fmt.Sprintf("ch_%.1f", ch.Number)))
		if _, err := os.Stat(imageDir); err == nil {
			imageDirs = append(imageDirs, imageDir)
		}
	}

	if len(imageDirs) == 0 {
		cleanup.Dir(d.workDir)
		d.results <- pipelineDoneMsg{Err: fmt.Errorf("no images found")}
		close(d.results)
		return
	}

	d.results <- pipelineProgressMsg{
		Idx: 1, Total: 1,
		Label: fmt.Sprintf("Generating %s_volume-%d%s", sanitizeName(d.mangaName), d.volumeNum, kindle.OutputExt(d.config.OutputFormat)),
	}

	epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_volume-%d.epub")
	err := epub.GenerateMerged(d.mangaName, d.volumeNum, d.chapters, d.workDir, epubPath)
	if err != nil {
		cleanup.Dir(d.workDir)
		d.results <- pipelineDoneMsg{Err: fmt.Errorf("generating merged epub: %w", err)}
		close(d.results)
		return
	}

	d.runSendSavePhase(epubDir)
}

func (d *DownloadScreen) runSendSavePhase(epubDir string) {
	var sent, failed int
	if d.config.SendToKindle {
		sent, failed = d.runSendPhase(epubDir)
	} else {
		sent, failed = d.runSavePhase(epubDir)
	}

	cleanup.Dir(d.workDir)
	d.results <- pipelineDoneMsg{Success: sent, Failed: failed}
	close(d.results)
}

func (d *DownloadScreen) runSendPhase(epubDir string) (sent, failed int) {
	d.state = stateSending

	if d.groupChapters {
		epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_volume-%d.epub")
		subject := fmt.Sprintf("%s - Volume %d", d.mangaName, d.volumeNum)
		err := email.SendToKindle(d.config, subject, epubPath)
		d.results <- pipelineProgressMsg{
			Idx: 1, Total: 1,
			Label: fmt.Sprintf("Sending %s_volume-%d%s", sanitizeName(d.mangaName), d.volumeNum, kindle.OutputExt(d.config.OutputFormat)),
			Err:   err,
		}
		if err == nil {
			return 1, 0
		}
		return 0, 1
	}

	for i, ch := range d.chapters {
		epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_ch%.1f.epub")
		if _, err := os.Stat(epubPath); os.IsNotExist(err) {
			failed++
			continue
		}
		subject := fmt.Sprintf("%s - %s", d.mangaName, ch.Title)
		err := email.SendToKindle(d.config, subject, epubPath)
		d.results <- pipelineProgressMsg{
			Idx: i + 1, Total: len(d.chapters),
			Label: fmt.Sprintf("Sending %d/%d: %s", i+1, len(d.chapters), ch.Title),
			Err:   err,
		}
		if err == nil {
			sent++
		} else {
			failed++
		}
	}
	return
}

func (d *DownloadScreen) runSavePhase(epubDir string) (saved, failed int) {
	destDir := d.config.DownloadDir
	if destDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		destDir = filepath.Join(home, "Downloads", "kindle_cli")
	}
	destDir = filepath.Join(destDir, sanitizeName(d.mangaName))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		d.results <- pipelineDoneMsg{Err: fmt.Errorf("creating save dir: %w", err)}
		return
	}

	d.state = stateSending

	if d.groupChapters {
		epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_volume-%d.epub")
		destPath := filepath.Join(destDir, filepath.Base(epubPath))
		var saveErr error
		if err := copyFile(epubPath, destPath); err != nil {
			saveErr = fmt.Errorf("copying epub: %w", err)
		}
		d.results <- pipelineProgressMsg{
			Idx: 1, Total: 1,
			Label: fmt.Sprintf("Saving %s", filepath.Base(epubPath)),
			Err:   saveErr,
		}
		if saveErr == nil {
			return 1, 0
		}
		return 0, 1
	}

	for i, ch := range d.chapters {
		epubPath := filepath.Join(epubDir, fmt.Sprintf("%s_ch%.1f.epub")
		if _, err := os.Stat(epubPath); os.IsNotExist(err) {
			failed++
			continue
		}
		destPath := filepath.Join(destDir, filepath.Base(epubPath))
		var saveErr error
		if err := copyFile(epubPath, destPath); err != nil {
			saveErr = fmt.Errorf("copying epub: %w", err)
		}
		d.results <- pipelineProgressMsg{
			Idx: i + 1, Total: len(d.chapters),
			Label: fmt.Sprintf("Saving %d/%d: %s", i+1, len(d.chapters), ch.Title),
			Err:   saveErr,
		}
		if saveErr == nil {
			saved++
		} else {
			failed++
		}
	}
	return
}

func (d *DownloadScreen) handlePipelineProgress(msg pipelineProgressMsg) (tea.Model, tea.Cmd) {
	d.pipelineIdx = msg.Idx
	d.pipelineTotal = msg.Total
	d.pipelineLabel = msg.Label
	if msg.Err != nil {
		d.failCount++
		d.pipelineError = msg.Err.Error()
		d.chapterErrors = append(d.chapterErrors, msg.Err.Error())
	} else {
		d.pipelineError = ""
		d.successCount++
	}
	return d, d.waitForResult()
}

func (d *DownloadScreen) handlePipelineDone(msg pipelineDoneMsg) (tea.Model, tea.Cmd) {
	// Override with final pipeline counts
	d.successCount = msg.Success
	d.failCount = msg.Failed
	if msg.Err != nil {
		d.state = stateError
		d.pipelineError = msg.Err.Error()
	} else if d.failCount > 0 && d.successCount == 0 {
		d.state = stateError
	} else {
		d.state = stateDone
	}
	return d, nil
}

// --- View ---

func (d *DownloadScreen) View() string {
	var b strings.Builder

	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	b.WriteString(fmt.Sprintf("\n  %s\n", bold.Render("Kindle Manga")))
	b.WriteString(fmt.Sprintf("  %s - %d chapters (%d pages)\n\n", d.mangaName, len(d.chapters), d.totalPages))

	switch d.state {
	case stateConfirming:
		b.WriteString(fmt.Sprintf("  %s\n\n", dim.Render("Press Enter to start download, Esc to cancel.")))

	case stateDownloading:
		barWidth := 40
		progress := 0
		if d.totalPages > 0 {
			progress = d.downloadedPages * barWidth / d.totalPages
		}
		bar := strings.Repeat("█", progress) + strings.Repeat("░", barWidth-progress)
		pct := 0
		if d.totalPages > 0 {
			pct = d.downloadedPages * 100 / d.totalPages
		}
		b.WriteString(fmt.Sprintf("  %s %s %d%%\n\n", d.spinner.View(), bar, pct))

		chIdx := d.currentChapter
		if chIdx > 0 && chIdx <= len(d.chapters) {
			b.WriteString(fmt.Sprintf("  Chapter %d/%d: %s\n", chIdx, len(d.chapters), d.chapters[chIdx-1].Title))
		}
		if d.currentPage > 0 {
			b.WriteString(fmt.Sprintf("  Page %d/%d\n", d.currentPage, d.currentTotal))
		}
		b.WriteString(fmt.Sprintf("\n  %s %d / %d", dim.Render("Downloaded:"), d.downloadedPages, d.totalPages))
		if d.errors > 0 {
			b.WriteString(fmt.Sprintf("  ·  %s %d", red.Render("Errors:"), d.errors))
		}
		b.WriteString(fmt.Sprintf("\n\n  %s", dim.Render("Esc to cancel")))

	case stateGenerating, stateSending:
		barWidth := 40
		progress := 0
		if d.pipelineTotal > 0 {
			progress = d.pipelineIdx * barWidth / d.pipelineTotal
		}
		bar := strings.Repeat("█", progress) + strings.Repeat("░", barWidth-progress)

		phaseLabel := "Processing"
		if d.state == stateGenerating {
			phaseLabel = "Generating EPUB"
		} else if d.config.SendToKindle {
			phaseLabel = "Sending to Kindle"
		} else {
			phaseLabel = "Saving EPUBs"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n\n", d.spinner.View(), phaseLabel))
		b.WriteString(fmt.Sprintf("  %s\n\n", bar))
		if d.pipelineLabel != "" {
			b.WriteString(fmt.Sprintf("  %s\n", d.pipelineLabel))
		}
		if d.pipelineError != "" {
			b.WriteString(fmt.Sprintf("  %s %s\n", red.Render("Error:"), d.pipelineError))
		}
		b.WriteString(fmt.Sprintf("\n  %d / %d processed", d.pipelineIdx, d.pipelineTotal))
		if d.failCount > 0 {
			b.WriteString(fmt.Sprintf("  ·  %s %d", red.Render("Failed:"), d.failCount))
		}
		b.WriteString("\n")

	case stateDone:
		b.WriteString(green.Render(fmt.Sprintf("  ✓ All done!\n\n")))
		if d.config.SendToKindle {
			b.WriteString(fmt.Sprintf("  %d chapters sent to %s\n", d.successCount, d.config.KindleEmail))
		} else {
			b.WriteString(fmt.Sprintf("  %d chapters saved\n", d.successCount))
		}
		if d.failCount > 0 {
			b.WriteString(fmt.Sprintf("  %s %d failed\n", red.Render("·"), d.failCount))
		}
		if d.errors > 0 {
			b.WriteString(fmt.Sprintf("  %s %d download errors\n", red.Render("·"), d.errors))
		}
		showCount := len(d.chapterErrors)
		if showCount > 2 {
			showCount = 2
		}
		for i := 0; i < showCount; i++ {
			b.WriteString(fmt.Sprintf("  %s\n", dim.Render(d.chapterErrors[i])))
		}
		if len(d.chapterErrors) > 2 {
			b.WriteString(fmt.Sprintf("  ... and %d more errors\n", len(d.chapterErrors)-2))
		}
		b.WriteString("\n  Temp files cleaned up.\n")
		b.WriteString(fmt.Sprintf("\n  %s", dim.Render("Enter or Esc to go back")))

	case stateError:
		b.WriteString(red.Render(fmt.Sprintf("\n  ✗ Failed\n\n")))
		if d.successCount > 0 {
			if d.config.SendToKindle {
				b.WriteString(fmt.Sprintf("  %d chapters sent\n", d.successCount))
			} else {
				b.WriteString(fmt.Sprintf("  %d chapters saved\n", d.successCount))
			}
		}
		if d.failCount > 0 {
			b.WriteString(fmt.Sprintf("  %d chapters failed\n", d.failCount))
		}
		if d.errors > 0 {
			b.WriteString(fmt.Sprintf("  %d download errors\n", d.errors))
		}
		showCount := len(d.chapterErrors)
		if showCount > 3 {
			showCount = 3
		}
		for i := 0; i < showCount; i++ {
			b.WriteString(fmt.Sprintf("  %s\n", dim.Render(d.chapterErrors[i])))
		}
		if len(d.chapterErrors) > 3 {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(d.chapterErrors)-3))
		}
		b.WriteString(fmt.Sprintf("\n  %s", dim.Render("Enter or Esc to go back")))

	case stateCancelled:
		b.WriteString(fmt.Sprintf("\n  Cancelled after %d pages\n\n", d.downloadedPages))
		b.WriteString(fmt.Sprintf("  Temp files cleaned up.\n"))
		b.WriteString(fmt.Sprintf("\n  %s", dim.Render("Enter or Esc to go back")))
	}

	return b.String()
}

// --- Commands ---

func (d *DownloadScreen) waitForResult() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-d.results
		if !ok {
			return pipelineDoneMsg{Err: fmt.Errorf("unexpected end")}
		}
		return msg
	}
}

// --- Helpers ---

func downloadImage(ctx context.Context, url, dest string) error {
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func sanitizeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, s)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
