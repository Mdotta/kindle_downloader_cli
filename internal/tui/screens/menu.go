package screens

import (
	"kindle_cli/internal/atsu"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Model and related methods for the MenuScreen ---
type MenuScreen struct {
	config         *config.Config
	client         *atsu.Client
	counter        int
	options        []string
	optionCommands []tea.Cmd
}

func NewMenuScreen(cfg *config.Config, client *atsu.Client) *MenuScreen {
	return &MenuScreen{
		config: cfg,
		client: client,
	}
}

func (m *MenuScreen) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next field")),
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev field")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "select")),
	}
}

// --- Tea.Model interface for MenuScreen ---

func (m *MenuScreen) Init() tea.Cmd {
	// Initialize any necessary state or perform setup here
	m.counter = 0
	m.options = []string{"Search Manga", "Settings", "Quit"}
		m.optionCommands = []tea.Cmd{
			screenstack.PushCmd(NewSearchScreen(m.config, m.client)),
			screenstack.PushCmd(NewSettingsScreen(m.config)),
			screenstack.QuitCmd(),
		}
	return nil
}

func (m *MenuScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle messages and update the model state accordingly
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			return m.MoveUp()
		case "down":
			return m.MoveDown()
		case "enter":
			return m.SelectOption()
		}
		// Handle other message types (e.g., custom messages for menu actions)
	}
	return m, nil
}

func (m *MenuScreen) View() string {
	// Return the string representation of the menu screen
	render := ""
	for i, option := range m.options {
		prefix := "  "
		if i == m.counter {
			prefix = "> "
		}
		render += prefix + option + "\n"
	}
	return render
}

// --- Helper methods for menu actions ---

func (m *MenuScreen) MoveUp() (tea.Model, tea.Cmd) {
	// Implement logic to move the selection up in the menu
	if m.counter > 0 {
		m.counter--
	}
	return m, nil
}

func (m *MenuScreen) MoveDown() (tea.Model, tea.Cmd) {
	// Implement logic to move the selection down in the menu
	if m.counter < len(m.options)-1 {
		m.counter++
	}
	return m, nil
}

func (m *MenuScreen) SelectOption() (tea.Model, tea.Cmd) {
	// Implement logic to select the current option and possibly push a new screen onto the stack
	return m, m.optionCommands[m.counter]
}
