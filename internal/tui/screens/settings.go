package screens

import (
	"fmt"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Model and related methods for the SettingsScreen ---

type SettingsScreen struct {
	// Add fields as needed, e.g., config, state, etc.
	config *config.Config
	inputs []textinput.Model // one input per config field
	focus  int               // which input is focused (0-based)
	keys   []string          // json field names in order (for the label)
}

func NewSettingsScreen(cfg *config.Config) *SettingsScreen {

	fields := []struct {
		key     string
		pointer *string // for string fields; nil for int
	}{
		// label             pointer to config field
		{"Kindle Email", &cfg.KindleEmail},
		{"SMTP Host", &cfg.SMTPHost},
		{"SMTP Port", nil}, // int field, handle separately
		{"SMTP Username", &cfg.SMTPUsername},
		{"SMTP Password", &cfg.SMTPPassword},
		{"Language", &cfg.DefaultLanguage},
		// {"Concurrent Downloads", nil}, // int field, handle separately
		// {"MangaDex API Key", &cfg.MangaDexApiKey},
		{"API Base URL", &cfg.ApiBaseUrl},
	}

	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.key
		if f.pointer != nil {
			ti.SetValue(*f.pointer)
		} else {
			ti.SetValue(fmt.Sprintf("%d", cfg.SMTPPort))
		}
		if i == 0 {
			ti.Focus()
		}
		inputs[i] = ti
	}

	return &SettingsScreen{
		config: cfg,
		inputs: inputs,
		focus:  0,
	}
}

func (s *SettingsScreen) KeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next field")),
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev field")),
		key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("^S", "save")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")),
	}
}

// --- Tea.Model interface for SettingsScreen ---

func (s *SettingsScreen) Init() tea.Cmd {
	return nil
}

func (s *SettingsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(s.inputs) == 0 {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, screenstack.PopCmd()

		case "tab", "down":
			s.blurAll()
			s.focus = (s.focus + 1) % len(s.inputs)
			s.inputs[s.focus].Focus()
			return s, nil

		case "shift+tab", "up":
			s.blurAll()
			s.focus = (s.focus - 1 + len(s.inputs)) % len(s.inputs)
			s.inputs[s.focus].Focus()
			return s, nil

		case "ctrl+s":
			s.save()
			return s, screenstack.PopCmd()
		}
	}

	// Delegate keypresses to the focused input
	cmd := s.updateInputs(msg)
	return s, cmd
}

func (s *SettingsScreen) View() string {
	render := "Settings\n"
	for i, input := range s.inputs {
		cursor := "  "
		if i == s.focus {
			cursor = "❯ "
		}
		render += fmt.Sprintf("%s%s\n", cursor, input.View())
	}
	return render
}

// --- Helper methods for SettingsScreen ---

func (s *SettingsScreen) blurAll() {
	for i := range s.inputs {
		s.inputs[i].Blur()
	}
}

func (s *SettingsScreen) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range s.inputs {
		var cmd tea.Cmd
		s.inputs[i], cmd = s.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (s *SettingsScreen) save() {
	s.config.KindleEmail = s.inputs[0].Value()
	s.config.SMTPHost = s.inputs[1].Value()
	s.config.SMTPPort, _ = strconv.Atoi(s.inputs[2].Value())
	s.config.SMTPUsername = s.inputs[3].Value()
	s.config.SMTPPassword = s.inputs[4].Value()
	s.config.DefaultLanguage = s.inputs[5].Value()
	s.config.Save()
}
