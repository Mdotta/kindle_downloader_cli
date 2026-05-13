package screens

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kindle_cli/internal/atsu"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Model ---

type ChaptersScreen struct {
	config    *config.Config
	client    *atsu.Client
	mangaID   string
	mangaName string

	detail     *atsu.MangaDetail
	loadErr    string
	loading    bool
	spinner    spinner.Model

	// Scanlator filtering
	scanlators    []atsu.Scanlator
	scanlatorIdx  int // -1 = all
	filterScanID  string

	// Chapter list
	chapters  []atsu.Chapter
	selected  map[string]bool // chapterID -> selected
	cursor    int
	offset    int // scroll offset
	width     int
	height    int
	listHeight int
}

func NewChaptersScreen(cfg *config.Config, client *atsu.Client, mangaID, mangaName string) *ChaptersScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	return &ChaptersScreen{
		config:       cfg,
		client:       client,
		mangaID:      mangaID,
		mangaName:    mangaName,
		loading:      true,
		spinner:      sp,
		scanlatorIdx: -1, // all
		selected:     make(map[string]bool),
	}
}

// --- Messages ---

type chaptersLoadedMsg struct {
	detail *atsu.MangaDetail
	err    error
}

// --- Tea.Model ---

func (c *ChaptersScreen) Init() tea.Cmd {
	return tea.Batch(c.loadChapters(), c.spinner.Tick, tea.WindowSize())
}

func (c *ChaptersScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chaptersLoadedMsg:
		c.loading = false
		if msg.err != nil {
			c.loadErr = msg.err.Error()
			return c, nil
		}
		c.detail = msg.detail
		c.scanlators = msg.detail.Scanlators
		c.chapters = msg.detail.Chapters
		if c.chapters == nil {
			c.chapters = []atsu.Chapter{}
		}
		return c, nil

	case spinner.TickMsg:
		if c.loading {
			var cmd tea.Cmd
			c.spinner, cmd = c.spinner.Update(msg)
			return c, cmd
		}

	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
		c.listHeight = msg.Height - 9
		if c.listHeight < 4 {
			c.listHeight = 4
		}
		return c, nil

	case tea.KeyMsg:
		return c.handleKey(msg)
	}
	return c, nil
}

func (c *ChaptersScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if c.loading {
		return c, nil
	}

	switch msg.String() {
	case "esc":
		return c, screenstack.PopCmd()

	case "up", "k":
		if c.cursor > 0 {
			c.cursor--
			if c.cursor < c.offset {
				c.offset = c.cursor
			}
		}

	case "down", "j":
		if c.cursor < len(c.filteredChapters())-1 {
			c.cursor++
			if c.cursor >= c.offset+c.listHeight {
				c.offset = c.cursor - c.listHeight + 1
			}
		}

	case "shift+tab", "left", "h":
		// Cycle scanlator backward
		c.scanlatorIdx--
		if c.scanlatorIdx < -1 {
			c.scanlatorIdx = len(c.scanlators) - 1
		}
		c.updateFilter()
		c.cursor = 0
		c.offset = 0
		c.selected = make(map[string]bool)

	case "tab", "right", "l":
		// Cycle scanlator forward
		c.scanlatorIdx++
		if c.scanlatorIdx >= len(c.scanlators) {
			c.scanlatorIdx = -1
		}
		c.updateFilter()
		c.cursor = 0
		c.offset = 0
		c.selected = make(map[string]bool)

	case " ":
		// Toggle selection
		filtered := c.filteredChapters()
		if c.cursor < len(filtered) {
			id := filtered[c.cursor].ID
			if c.selected[id] {
				delete(c.selected, id)
			} else {
				c.selected[id] = true
			}
		}

	case "enter":
		sel := c.getSelected()
		if len(sel) > 0 {
			confirmScreen := NewConfirmScreen(c.config, c.client, c.mangaName, c.mangaID, sel)
			return c, screenstack.PushCmd(confirmScreen)
		}

	case "a", "A":
		// Select all
		for _, ch := range c.filteredChapters() {
			c.selected[ch.ID] = true
		}

	case "d", "D":
		// Deselect all
		c.selected = make(map[string]bool)
	}

	return c, nil
}

func (c *ChaptersScreen) View() string {
	if c.loading {
		return fmt.Sprintf("\n  %s Loading chapters for %s...\n", c.spinner.View(), c.mangaName)
	}

	if c.loadErr != "" {
		return fmt.Sprintf("\n  Error: %s\n\n  Press Esc to go back.\n", c.loadErr)
	}

	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("\n  %s", c.mangaName))
	if c.detail != nil {
		b.WriteString(fmt.Sprintf(" — %s · %s", c.detail.Type, c.detail.Status))
	}
	b.WriteString("\n")

	// Scanlator filter
	if len(c.scanlators) > 1 {
		b.WriteString("\n  Scanlator: ")
		for i, s := range c.scanlators {
			if i == c.scanlatorIdx {
				b.WriteString(fmt.Sprintf("[%s] ", s.Name))
			} else {
				b.WriteString(fmt.Sprintf("%s ", s.Name))
			}
		}
		if c.scanlatorIdx == -1 {
			b.WriteString("[All]")
		} else {
			b.WriteString("All")
		}
		b.WriteString("  (Tab to switch)\n")
	} else if len(c.scanlators) == 1 {
		b.WriteString(fmt.Sprintf("\n  Scanlator: %s\n", c.scanlators[0].Name))
	}

	// Counts
	filtered := c.filteredChapters()
	selCount := c.selectedCount()
	b.WriteString(fmt.Sprintf("\n  %d chapters", len(filtered)))
	if selCount > 0 {
		totalPages := c.selectedPages()
		b.WriteString(fmt.Sprintf(" · %d selected (%d pages)", selCount, totalPages))
	}
	b.WriteString("\n\n")

	// Chapter list
	visibleEnd := c.offset + c.listHeight
	if visibleEnd > len(filtered) {
		visibleEnd = len(filtered)
	}

	checkMark := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render
	cursorMark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	for i := c.offset; i < visibleEnd; i++ {
		ch := filtered[i]

		// Checkbox
		checked := "[ ]"
		if c.selected[ch.ID] {
			checked = checkMark("[✓]")
		}

		// Cursor
		if i == c.cursor {
			prefix := cursorMark.Render(">")
			b.WriteString(fmt.Sprintf(" %s %s %s  %s  (%d pages)\n",
				prefix, checked, fmt.Sprintf("%5.1f", ch.Number), ch.Title, ch.PageCount))
		} else {
			b.WriteString(fmt.Sprintf("   %s %s  %s  (%d pages)\n",
				checked, fmt.Sprintf("%5.1f", ch.Number), ch.Title, ch.PageCount))
		}
	}

	// Fill remaining space
	for i := visibleEnd - c.offset; i < c.listHeight; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n  ↑↓ navigate  Space select  A all  D none  Enter confirm  ←→ scanlator  Esc back\n")

	return b.String()
}

// --- Commands ---

func (c *ChaptersScreen) loadChapters() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		detail, err := c.client.GetManga(ctx, c.mangaID)
		return chaptersLoadedMsg{detail, err}
	}
}

// --- Helpers ---

func (c *ChaptersScreen) updateFilter() {
	if c.scanlatorIdx >= 0 && c.scanlatorIdx < len(c.scanlators) {
		c.filterScanID = c.scanlators[c.scanlatorIdx].ID
	} else {
		c.filterScanID = ""
	}
}

func (c *ChaptersScreen) filteredChapters() []atsu.Chapter {
	if c.filterScanID == "" {
		return c.chapters
	}
	var filtered []atsu.Chapter
	for _, ch := range c.chapters {
		if ch.ScanlationMangaID == c.filterScanID {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func (c *ChaptersScreen) selectedCount() int {
	count := 0
	for _, ch := range c.filteredChapters() {
		if c.selected[ch.ID] {
			count++
		}
	}
	return count
}

func (c *ChaptersScreen) selectedPages() int {
	total := 0
	for _, ch := range c.filteredChapters() {
		if c.selected[ch.ID] {
			total += ch.PageCount
		}
	}
	return total
}

func (c *ChaptersScreen) getSelected() []atsu.Chapter {
	var sel []atsu.Chapter
	for _, ch := range c.filteredChapters() {
		if c.selected[ch.ID] {
			sel = append(sel, ch)
		}
	}
	return sel
}
