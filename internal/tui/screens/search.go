package screens

import (
	"context"
	"fmt"
	"kindle_cli/internal/atsu"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Model ---

type SearchScreen struct {
	config *config.Config
	client *atsu.Client

	searchInput textinput.Model
	focusInput  bool // true = input focused, false = list focused

	resultList   list.Model
	spinner      spinner.Model
	loading      bool
	searchError  string
	currentPage  int
	totalItems   int
	itemsPerPage int
	totalPages   int

	width  int
	height int
}

func NewSearchScreen(cfg *config.Config, client *atsu.Client) *SearchScreen {
	ti := textinput.New()
	ti.Placeholder = "Type manga name and press Enter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("12")).BorderForeground(lipgloss.Color("12"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("8"))

	l := list.New([]list.Item{}, delegate, 80, 20)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowTitle(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	return &SearchScreen{
		config:       cfg,
		client:       client,
		searchInput:  ti,
		focusInput:   true,
		resultList:   l,
		spinner:      sp,
		itemsPerPage: 20,
		currentPage:  1,
	}
}

func (s *SearchScreen) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "switch focus")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "search / select")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")),
	}
}

// --- Messages ---

type searchResultsMsg struct {
	results []atsu.Manga
	total   int
	page    int
	err     error
}

type tickMsg time.Time

// --- Items ---

type MangaItem struct {
	TitleStr       string
	DescriptionStr string
	MangaID        string
}

func (m MangaItem) Title() string       { return m.TitleStr }
func (m MangaItem) Description() string { return m.DescriptionStr }
func (m MangaItem) FilterValue() string { return m.TitleStr }

// --- Tea.Model ---

func (s *SearchScreen) Init() tea.Cmd {
	return textinput.Blink
}

func (s *SearchScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.resultList.SetWidth(msg.Width)
		h := msg.Height - 6
		if h < 4 {
			h = 4
		}
		s.resultList.SetHeight(h)
		s.searchInput.Width = msg.Width - 4
		return s, nil

	case spinner.TickMsg:
		if s.loading {
			var cmd tea.Cmd
			s.spinner, cmd = s.spinner.Update(msg)
			return s, cmd
		}
		return s, nil

	case searchResultsMsg:
		s.loading = false
		if msg.err != nil {
			s.searchError = msg.err.Error()
			return s, nil
		}
		s.searchError = ""
		s.currentPage = msg.page
		s.totalItems = msg.total
		s.totalPages = (s.totalItems + s.itemsPerPage - 1) / s.itemsPerPage

		items := make([]list.Item, len(msg.results))
		for i, m := range msg.results {
			items[i] = MangaItem{
				TitleStr:       m.Title,
				DescriptionStr: fmt.Sprintf("%s · %s · %d", m.Type, m.Status, m.Year),
				MangaID:        m.ID,
			}
		}
		s.resultList.SetItems(items)
		s.resultList.ResetSelected()
		return s, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "esc":
			if s.loading {
				return s, nil
			}
			return s, screenstack.PopCmd()

		case "tab":
			s.focusInput = !s.focusInput
			if s.focusInput {
				s.searchInput.Focus()
				s.resultList.SetShowHelp(false)
			} else {
				s.searchInput.Blur()
			}
			return s, nil
		}

		// Mode-specific keys
		if s.focusInput {
			return s.updateInputMode(msg)
		}
		return s.updateListMode(msg)
	}

	// Delegate spinner ticks
	return s, nil
}

func (s *SearchScreen) updateInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if s.searchInput.Value() == "" {
			return s, nil
		}
		s.loading = true
		s.searchError = ""
		s.currentPage = 1
		return s, tea.Batch(s.doSearch(1), s.spinner.Tick)
	}

	var cmd tea.Cmd
	s.searchInput, cmd = s.searchInput.Update(msg)
	return s, cmd
}

func (s *SearchScreen) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		item, ok := s.resultList.SelectedItem().(MangaItem)
		if !ok {
			return s, nil
		}
		chScreen := NewChaptersScreen(s.config, s.client, item.MangaID, item.TitleStr)
		return s, screenstack.PushCmd(chScreen)

	case "right", "n":
		if s.currentPage < s.totalPages {
			s.loading = true
			s.currentPage++
			return s, tea.Batch(s.doSearch(s.currentPage), s.spinner.Tick)
		}

	case "left", "p":
		if s.currentPage > 1 {
			s.loading = true
			s.currentPage--
			return s, tea.Batch(s.doSearch(s.currentPage), s.spinner.Tick)
		}
	}

	var cmd tea.Cmd
	s.resultList, cmd = s.resultList.Update(msg)
	return s, cmd
}

func (s *SearchScreen) View() string {
	// Input
	var inputView string
	if s.focusInput {
		inputView = "> " + s.searchInput.View()
	} else {
		inputView = "  " + s.searchInput.View()
	}

	// Status line
	status := ""
	if s.loading {
		status = s.spinner.View() + " Searching..."
	} else if s.searchError != "" {
		status = "Error: " + s.searchError
	} else if s.totalItems > 0 {
		status = fmt.Sprintf("%d results · page %d/%d (←/→ to navigate)", s.totalItems, s.currentPage, s.totalPages)
	}

	// Pagination
	if s.totalItems > 0 && len(s.resultList.Items()) > 0 {
		s.resultList.Title = fmt.Sprintf(" Page %d/%d ", s.currentPage, s.totalPages)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		"",
		inputView,
		"",
		status,
		"",
		s.resultList.View(),
	)

	return view
}

// --- Commands ---

func (s *SearchScreen) doSearch(page int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		results, total, err := s.client.Search(ctx, s.searchInput.Value(), page, s.itemsPerPage)
		return searchResultsMsg{results, total, page, err}
	}
}
