package screens

import (
	"kindle_cli/internal/atsu"
	"kindle_cli/internal/config"
	"kindle_cli/pkg/screenstack"

	tea "github.com/charmbracelet/bubbletea"
)

type ChaptersScreen struct {
	config    *config.Config
	client    *atsu.Client
	mangaID   string
	mangaName string
}

func NewChaptersScreen(cfg *config.Config, client *atsu.Client, mangaID, mangaName string) *ChaptersScreen {
	return &ChaptersScreen{
		config:    cfg,
		client:    client,
		mangaID:   mangaID,
		mangaName: mangaName,
	}
}

func (c *ChaptersScreen) Init() tea.Cmd {
	return nil
}

func (c *ChaptersScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return c, screenstack.PopCmd()
		}
	}
	return c, nil
}

func (c *ChaptersScreen) View() string {
	return "Chapters for: " + c.mangaName + "\n\nPress Esc to go back."
}
