package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	BoldStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#eee")).Bold(true)
	Bold      = BoldStyle.Render

	LightGray = lipgloss.NewStyle().Foreground(lipgloss.Color("#999"))

	Gray = func(shade int) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%x%x%x", shade, shade, shade)))
	}

	HABlue  = lipgloss.Color("#1DAEEF")
	HAStyle = lipgloss.NewStyle().Foreground(HABlue)

	DarkDivider        = Gray(6).SetString("|")
	DarkerDivider      = Gray(3).SetString("|")
	DarkIndicatorLeft  = LightGray.SetString("←")
	DarkIndicatorRight = LightGray.SetString("→")
)
