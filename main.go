package main

import (
	"fmt"
	"kindle_cli/internal/config"
	"kindle_cli/internal/tui/screens"
	"kindle_cli/pkg/screenstack"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	stack := screenstack.NewStack()

	stack.Push(screens.NewMenuScreen(cfg))
	p := tea.NewProgram(stack, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
