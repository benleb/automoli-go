package icons

import "github.com/charmbracelet/lipgloss"

const (
	// light related messages.
	LightOn   = "💡"
	LightOff  = "🌑"
	AlreadyOn = "🔛"

	// motion/trigger related messages.
	Trigger = "🫨 "
	Motion  = "💃"

	// reactions & related messages.
	Blind = "🙈"
	Sleep = "💤"
	Hae   = "⁉️ ‽"
	Block = "🚫"

	// connection related messages.
	ConnectionFailed = "🔴"
	ConnectionOK     = "🟢"
	ConnectionChain  = "🔗"
	ReconnectCircle  = "↻"

	// daytime related messages.
	Alarm = "⏰"

	// other messages.
	Cross     = "✖️"
	Tick      = "✔"
	Checklist = "📋"

	Bath    = "🛀"
	Broom   = "🧹"
	Door    = "🚪"
	Glasses = "👓"
	Key     = "🔑"
	Rocket  = "🚀"
	Shrug   = "🤷‍♀️"
	Splash  = "💦"
	Home    = "🏠"
	Call    = "📞"

	Stopwatch = "⏱️"
	Sub       = "🚇"
	Watchdog  = "🐕"

	// go stylecheck linter ST1018.
	WeightLift = "🏋️\u200d"
	Detective  = "🕵️\u200d"
)

var (
	GreenTick = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).SetString(" " + Tick)
	RedCross  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).SetString(Cross)
)
