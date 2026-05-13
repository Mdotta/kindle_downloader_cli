package screens

import (
	"fmt"
	"strconv"
	"strings"

	"kindle_cli/internal/atsu"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type confirmPhase int

const (
	phaseReview confirmPhase = iota
	phaseAskGroup
	phaseVolumeNum
)

type ConfirmScreen struct {
	config    *config.Config
	client    *atsu.Client
	mangaName string
	mangaID   string
	chapters  []atsu.Chapter

	phase  confirmPhase
	cursor int
	width  int
	height int

	// Grouping
	groupAsk    bool // yes/no toggle
	volumeInput textinput.Model
}

func NewConfirmScreen(cfg *config.Config, client *atsu.Client, mangaName, mangaID string, chapters []atsu.Chapter) *ConfirmScreen {
	ti := textinput.New()
	ti.Placeholder = "1"
	ti.CharLimit = 5
	ti.Width = 10

	return &ConfirmScreen{
		config:      cfg,
		client:      client,
		mangaName:   mangaName,
		mangaID:     mangaID,
		chapters:    chapters,
		phase:       phaseReview,
		volumeInput: ti,
	}
}

func (c *ConfirmScreen) Init() tea.Cmd {
	return tea.WindowSize()
}

func (c *ConfirmScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
		return c, nil

	case tea.KeyMsg:
		switch c.phase {
		case phaseReview:
			return c.updateReview(msg)
		case phaseAskGroup:
			return c.updateAskGroup(msg)
		case phaseVolumeNum:
			return c.updateVolumeNum(msg)
		}
	}
	return c, nil
}

func (c *ConfirmScreen) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return c, screenstack.PopCmd()

	case "up", "k":
		if c.cursor > 0 {
			c.cursor--
		}

	case "down", "j":
		if c.cursor < len(c.chapters)-1 {
			c.cursor++
		}

	case "enter":
		c.phase = phaseAskGroup
	}

	return c, nil
}

func (c *ConfirmScreen) updateAskGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		c.phase = phaseReview
		return c, nil

	case "y", "Y", " ":
		c.groupAsk = true
		c.phase = phaseVolumeNum
		c.volumeInput.Focus()
		return c, nil

	case "n", "N", "left", "right", "tab":
		c.groupAsk = !c.groupAsk

	case "enter":
		if c.groupAsk {
			c.phase = phaseVolumeNum
			c.volumeInput.Focus()
			return c, nil
		}
		return c.pushDownload(0, false)
	}
	return c, nil
}

func (c *ConfirmScreen) updateVolumeNum(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		c.phase = phaseAskGroup
		c.volumeInput.Blur()
		return c, nil

	case "enter":
		vol := 1
		if v, err := strconv.Atoi(strings.TrimSpace(c.volumeInput.Value())); err == nil && v > 0 {
			vol = v
		}
		return c.pushDownload(vol, true)
	}

	var cmd tea.Cmd
	c.volumeInput, cmd = c.volumeInput.Update(msg)
	return c, cmd
}

func (c *ConfirmScreen) pushDownload(volumeNum int, grouped bool) (tea.Model, tea.Cmd) {
	dl := NewDownloadScreen(c.config, c.client, c.mangaName, c.mangaID, c.chapters)
	dl.volumeNum = volumeNum
	dl.groupChapters = grouped
	return c, screenstack.PushCmd(dl)
}

func (c *ConfirmScreen) View() string {
	var total int
	for _, ch := range c.chapters {
		total += ch.PageCount
	}

	var b strings.Builder
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	b.WriteString(fmt.Sprintf("\n  %s\n", bold.Render("Confirm Download")))
	b.WriteString(fmt.Sprintf("  %s\n\n", dim.Render(strings.Repeat("─", 40))))
	b.WriteString(fmt.Sprintf("  Manga: %s\n", c.mangaName))
	b.WriteString(fmt.Sprintf("  Chapters: %d\n", len(c.chapters)))
	b.WriteString(fmt.Sprintf("  Total pages: %d\n\n", total))

	switch c.phase {
	case phaseReview:
		b.WriteString(fmt.Sprintf("  %s\n", dim.Render("Chapters to download:")))
		listHeight := c.height - 14
		if listHeight < 2 {
			listHeight = 2
		}
		if listHeight > len(c.chapters) {
			listHeight = len(c.chapters)
		}
		start := c.cursor - listHeight/2
		if start < 0 {
			start = 0
		}
		if start > len(c.chapters)-listHeight {
			start = len(c.chapters) - listHeight
		}
		if start < 0 {
			start = 0
		}
		for i := start; i < start+listHeight; i++ {
			prefix := "  "
			if i == c.cursor {
				prefix = "> "
			}
			ch := c.chapters[i]
			b.WriteString(fmt.Sprintf("%sCh. %s (%d pages)\n", prefix, ch.Title, ch.PageCount))
		}

		b.WriteString(fmt.Sprintf("\n  %s\n", dim.Render(strings.Repeat("─", 40))))
		b.WriteString("\n  Enter to continue  ·  Esc to cancel\n")

	case phaseAskGroup:
		b.WriteString(fmt.Sprintf("  %s\n\n", dim.Render("Group all chapters into a single volume?")))
		yesMarker := "  "
		noMarker := "  "
		if c.groupAsk {
			yesMarker = "> "
		}
		if !c.groupAsk {
			noMarker = "> "
		}
		b.WriteString(fmt.Sprintf("  %s[ Yes ]  (one EPUB with all chapters)\n", yesMarker))
		b.WriteString(fmt.Sprintf("  %s[ No  ]  (one EPUB per chapter)\n", noMarker))
		b.WriteString(fmt.Sprintf("\n  %s\n", dim.Render(strings.Repeat("─", 40))))
		b.WriteString("\n  ←→ toggle  ·  Enter confirm  ·  Esc back\n")

	case phaseVolumeNum:
		b.WriteString(fmt.Sprintf("  %s\n\n", dim.Render("Enter volume number:")))
		b.WriteString(fmt.Sprintf("  > %s\n", c.volumeInput.View()))
		b.WriteString(fmt.Sprintf("\n\n  EPUB: %s_volume-{N}.epub\n", c.mangaName))
		b.WriteString(fmt.Sprintf("\n  %s\n", dim.Render(strings.Repeat("─", 40))))
		b.WriteString("\n  Enter confirm  ·  Esc back\n")
	}

	return b.String()
}
