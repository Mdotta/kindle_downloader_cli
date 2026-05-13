package screenstack

import (
	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Model and initializer ---
type Stack struct {
	screens []tea.Model
	help    help.Model
}

func NewStack() *Stack {
	return &Stack{help: help.New()}
}

// --- Stack management ---

func (s *Stack) Push(screen tea.Model) {
	s.screens = append(s.screens, screen)
}

func (s *Stack) Pop() tea.Model {
	if len(s.screens) == 0 {
		return nil
	}
	screen := s.screens[len(s.screens)-1]
	s.screens = s.screens[:len(s.screens)-1]
	return screen
}

func (s *Stack) Top() tea.Model {
	if len(s.screens) == 0 {
		return nil
	}
	return s.screens[len(s.screens)-1]
}

func (s *Stack) Len() int {
	return len(s.screens)
}

// --- Messages ---

// BackMsg is a message that indicates the user wants to go back to the previous screen.
type BackMsg struct{}

// PushScreenMsg is a message that indicates the user wants to push a new screen onto the stack.
type PushScreenMsg struct {
	Screen tea.Model
}

// ReplaceScreenMsg is a message that indicates the user wants to replace the current screen with a new one.
type ReplaceScreenMsg struct {
	Screen tea.Model
}

type BackToRootMsg struct{}

// QuitMsg is a message that indicates the user wants to quit the application.
type QuitMsg struct{}

// --- Commands ---

func QuitCmd() tea.Cmd {
	return func() tea.Msg {
		return QuitMsg{}
	}
}

func PopCmd() tea.Cmd {
	return func() tea.Msg {
		return BackMsg{}
	}
}

func PushCmd(screen tea.Model) tea.Cmd {
	return func() tea.Msg {
		return PushScreenMsg{Screen: screen}
	}
}

func BackToRootCmd() tea.Cmd {
	return func() tea.Msg {
		return BackToRootMsg{}
	}
}

func ReplaceCmd(screen tea.Model) tea.Cmd {
	return func() tea.Msg {
		return ReplaceScreenMsg{Screen: screen}
	}
}

// --- Tea.Model interface implementation ---

func (s *Stack) Init() tea.Cmd {
	if s.Len() > 0 {
		return s.Top().Init()
	}
	return nil
}

func (s *Stack) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case QuitMsg:
		return s, tea.Quit
	case BackToRootMsg:
		firstScreen := s.screens[0]
		s.screens = nil
		s.Push(firstScreen)
		return s, s.Top().Init()
	case BackMsg:
		s.Pop()
		return s, nil
	case PushScreenMsg:
		s.Push(msg.Screen)
		return s, s.Top().Init()
	case ReplaceScreenMsg:
		if s.Len() > 0 {
			s.Pop()
		}
		s.Push(msg.Screen)
		return s, s.Top().Init()
	}

	if s.Len() > 0 {
		top := s.Top()
		newModel, cmd := top.Update(msg)
		s.screens[len(s.screens)-1] = newModel // update screen in-place
		return s, cmd
	}
	return s, nil
}

func (s *Stack) View() string {
	if s.Len() == 0 {
		return ""
	}

	view := s.Top().View()

	if h, ok := s.Top().(Helpable); ok {
		view += "\n" + s.help.ShortHelpView(h.KeyBindings())
	}
	return view
}
