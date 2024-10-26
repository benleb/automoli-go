package style

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	Gray = func(shade int) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%x%x%x", shade, shade, shade)))
	}

	BoldStyle = Gray(238).Bold(true) // #eeeeee
	Bold      = BoldStyle.Render

	LightGray = Gray(9)

	HABlue  = lipgloss.Color("#1DAEEF")
	HAStyle = lipgloss.NewStyle().Foreground(HABlue)

	DarkDivider        = Gray(5).SetString("⁞")
	DarkerDivider      = Gray(3).SetString("|")
	DarkIndicatorLeft  = LightGray.SetString("←")
	DarkIndicatorRight = LightGray.SetString("→")
)

func ColorizeHABlue(text string) string {
	return HAStyle.SetString(text).Render()
}

func HABlueFrame(text string) string {
	return ColorizeHABlue("<") + text + ColorizeHABlue(">")
}
