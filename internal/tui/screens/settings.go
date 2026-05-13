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
	config *config.Config
	inputs []textinput.Model
	focus  int
}

func NewSettingsScreen(cfg *config.Config) *SettingsScreen {
	fields := []struct {
		key     string
		pointer *string
		intPtr  *int
		boolPtr *bool
	}{
		{"Kindle Email", &cfg.KindleEmail, nil, nil},
		{"SMTP Host", &cfg.SMTPHost, nil, nil},
		{"SMTP Port", nil, &cfg.SMTPPort, nil},
		{"SMTP Username", &cfg.SMTPUsername, nil, nil},
		{"SMTP Password", &cfg.SMTPPassword, nil, nil},
		{"Language", &cfg.DefaultLanguage, nil, nil},
		{"API Base URL", &cfg.ApiBaseUrl, nil, nil},
		{"Send to Kindle (space=toggle)", nil, nil, &cfg.SendToKindle},
		{"Download Dir", &cfg.DownloadDir, nil, nil},
	}

	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.key
		if f.pointer != nil {
			ti.SetValue(*f.pointer)
		} else if f.intPtr != nil {
			ti.SetValue(fmt.Sprintf("%d", *f.intPtr))
		} else if f.boolPtr != nil {
			if *f.boolPtr {
				ti.SetValue("yes")
			} else {
				ti.SetValue("no")
			}
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

		case " ":
			// Toggle boolean fields (Send to Kindle is at index 7)
			if s.focus == 7 {
				s.config.SendToKindle = !s.config.SendToKindle
				if s.config.SendToKindle {
					s.inputs[7].SetValue("yes")
				} else {
					s.inputs[7].SetValue("no")
				}
				return s, nil
			}
			cmd := s.updateInputs(msg)
			return s, cmd

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
	labels := []string{
		"Kindle Email",
		"SMTP Host",
		"SMTP Port",
		"SMTP Username",
		"SMTP Password",
		"Language",
		"API Base URL",
		"Send to Kindle",
		"Download Dir",
	}
	for i, input := range s.inputs {
		cursor := "  "
		if i == s.focus {
			cursor = "❯ "
		}
		label := ""
		if i < len(labels) {
			label = fmt.Sprintf("%-16s", labels[i])
		}
		render += fmt.Sprintf("%s%s %s\n", cursor, label, input.View())
	}
	render += "\nTab/↑↓ navigate · Space=toggle · Ctrl+S save · Esc back\n"
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
	s.config.ApiBaseUrl = s.inputs[6].Value()
	// Send to Kindle (index 7) — already set via toggle
	s.config.DownloadDir = s.inputs[8].Value()
	s.config.Save()
}
