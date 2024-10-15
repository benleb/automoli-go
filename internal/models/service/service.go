package service

import (
	"strings"

	"github.com/benleb/automoli-go/internal/style"
	"github.com/charmbracelet/lipgloss"
)

type Service string

const (
	TurnOn  Service = "turn_on"
	TurnOff Service = "turn_off"
	Toggle  Service = "toggle"
)

func (s Service) String() string {
	// return lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888")).SetString(string(s)).String()
	return string(s)
}

func (s Service) FmtString() string {
	serviceName := s.String()

	if nameParts := strings.Split(serviceName, "_"); len(nameParts) > 1 {
		serviceName = nameParts[0] + style.LightGray.Render("_") + nameParts[1]
	}

	return lipgloss.NewStyle().Italic(true).SetString(style.Gray(6).Render("…") + serviceName).String()
}

func (s Service) FmtStringStriketrough() string {
	// return style.Gray(6).Render("…") + lipgloss.NewStyle().Foreground(lipgloss.Color("#bbb")).Italic(true).Render(string(s))
	return lipgloss.NewStyle().Italic(true).SetString(style.Gray(6).Render("…")).String() +
		lipgloss.NewStyle().Italic(true).Faint(true).Strikethrough(true).SetString(s.String()).String()
}
