package styles

import "github.com/charmbracelet/lipgloss"

var (
	TITLE = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7d56f4"))

	INFO = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#888888"))

	SUCCESS = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#28a745"))

	ERROR = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ee4b2b"))
)
