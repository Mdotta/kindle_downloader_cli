package screens

import (
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type SearchScreen struct {
	config         *config.Config
	searchInput    textinput.Model
	confirmedValue string
}

func NewSearchScreen(cfg *config.Config) *SearchScreen {
	return &SearchScreen{
		config: cfg,
	}
}

func (s *SearchScreen) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "select")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")),
	}
}

func (s *SearchScreen) Init() tea.Cmd {
	s.searchInput = textinput.New()
	s.searchInput.Placeholder = "Enter manga name"
	s.searchInput.Focus()
	s.confirmedValue = ""
	return nil
}

func (s *SearchScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			s.confirmedValue = s.searchInput.Value()
			return s, nil
		case "esc":
			return s, screenstack.PopCmd()
		}
	}
	s.searchInput, _ = s.searchInput.Update(msg)
	return s, nil
}

func (s *SearchScreen) View() string {
	render := "Search Screen\n"
	render += "Enter manga name: " + s.searchInput.View()
	render += "\n"
	return render
}
