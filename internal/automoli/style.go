package automoli

import (
	"fmt"
	"math/rand"

	"github.com/benleb/automoli-go/internal/icons"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	// style configuration for room configuration printed at startup.

	// list general.
	list = lipgloss.NewStyle().
		MarginLeft(0).
		MarginRight(0).
		PaddingTop(1)

	listHeader = lipgloss.NewStyle().
			MarginLeft(1).
			MarginRight(2).
			Width(8).
			Align(lipgloss.Right).
			AlignVertical(lipgloss.Top).
			Foreground(lipgloss.Color("#333555")).
			Render

	listItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#969B86", Dark: "#ccc"})

	listItem = listItemStyle.Render

	// lights lists.
	bulbOn = lipgloss.NewStyle().SetString(icons.LightOn).
		PaddingLeft(1).
		String()
	listItemOn = func(s string) string {
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "#eee", Light: "#111"}).
			UnsetWidth().
			Render(s) + bulbOn
	}

	// sensors lists.
	motionOn = lipgloss.NewStyle().SetString(icons.Motion).
			PaddingLeft(1).
			String()
	listItemMotionOn = func(s string) string {
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "#eee", Light: "#111"}).
			UnsetWidth().
			Render(s) + motionOn
	}

	// daytimes lists.
	activeDaytimeIndicator = lipgloss.NewStyle().SetString("→").
				PaddingLeft(1).
				PaddingRight(1)
	listItemActive = func(s string, c lipgloss.Color) string {
		return activeDaytimeIndicator.Foreground(c).String() + lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "#eee", Light: "#111"}).
			Render(s)
	}
	listDaytimeItem = lipgloss.NewStyle().
			PaddingLeft(3).
			Foreground(lipgloss.AdaptiveColor{Light: "#969B86", Dark: "#AAA"}).
			Render
)

const ASCIIHeader = `
     ___           ___           ___           ___           ___           ___           ___
    /\  \         /\__\         /\  \         /\  \         /\__\         /\  \         /\__\      ___ 
   /::\  \       /:/  /         \:\  \       /::\  \       /::|  |       /::\  \       /:/  /     /\  \
  /:/\:\  \     /:/  /           \:\  \     /:/\:\  \     /:|:|  |      /:/\:\  \     /:/  /      \:\  \
 /::\~\:\  \   /:/  /  ___       /::\  \   /:/  \:\  \   /:/|:|__|__   /:/  \:\  \   /:/  /       /::\__\
/:/\:\ \:\__\ /:/__/  /\__\     /:/\:\__\ /:/__/ \:\__\ /:/ |::::\__\ /:/__/ \:\__\ /:/__/     __/:/\/__/
\/__\:\/:/  / \:\  \ /:/  /    /:/  \/__/ \:\  \ /:/  / \/__/~~/:/  / \:\  \ /:/  / \:\  \    /\/:/  /
     \::/  /   \:\  /:/  /    /:/  /       \:\  /:/  /        /:/  /   \:\  /:/  /   \:\  \   \::/__/
     /:/  /     \:\/:/  /     \/__/         \:\/:/  /        /:/  /     \:\/:/  /     \:\  \   \:\__\
    /:/  /       \::/  /                     \::/  /        /:/  /       \::/  /       \:\__\   \/__/
    \/__/         \/__/                       \/__/         \/__/         \/__/         \/__/`

// GenerateColorFromString generates a color based on the given seed.
func GenerateColorFromString(seedPhrase string) lipgloss.Color {
	// ✨  🪄    ✨    ✨     ✨   ✨
	//   ✨  🦄    ✨    ✨     ✨
	// ✨  🪄  magic numbers!  🦄   🪄
	//     ✨   🪄  ✨    ✨     ✨
	// ✨  🦄    ✨    ✨      🦄  ✨

	// initial magic color seed
	magicColorSeed := int64(17)

	// create a magic seed number to generate a random color
	magicSeedNumber := magicColorSeed

	// convert the seed phrase to runes (Unicode characters)
	runes := []rune(seedPhrase)

	// get something like the faculty of the seed number
	for i := range runes {
		magicSeedNumber *= int64(runes[i])
	}

	// create a new random number generator with the magic seed number
	rng := rand.New(rand.NewSource(magicSeedNumber)) //nolint:gosec

	// generate a magic random - but deterministic - color based on the magic seed number
	magicColor := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", rng.Intn(256), rng.Intn(256), rng.Intn(256)))

	log.Debugf("✨🪄  %s ✨ initial: %d 🦄  ✨ | seed: %+v 🪄🦄", lipgloss.NewStyle().Foreground(magicColor).Render(seedPhrase), magicColorSeed, magicSeedNumber)

	return magicColor
}
