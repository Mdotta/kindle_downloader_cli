package screenstack

import "github.com/charmbracelet/bubbles/key"

type Helpable interface {
	KeyBindings() []key.Binding
}
