package screenstack

import "github.com/charmbracelet/bubbles/key"

// Helpable is an interface that models can implement to provide key bindings for the help menu.
type Helpable interface {
	KeyBindings() []key.Binding
}
