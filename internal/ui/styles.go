// Package ui provides UI components and styling for WUT
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions for consistent theming
var (
	ColorGreen  = lipgloss.Color("#10B981")
	ColorRed    = lipgloss.Color("#EF4444")
	ColorYellow = lipgloss.Color("#F59E0B")
	ColorCyan   = lipgloss.Color("#06B6D4")
	ColorGray   = lipgloss.Color("#6B7280")
	ColorBlue   = lipgloss.Color("#3B82F6")
)

// Green returns a green-colored string
func Green(s string) string {
	return lipgloss.NewStyle().Foreground(ColorGreen).Render(s)
}

// Red returns a red-colored string
func Red(s string) string {
	return lipgloss.NewStyle().Foreground(ColorRed).Render(s)
}

// Yellow returns a yellow-colored string
func Yellow(s string) string {
	return lipgloss.NewStyle().Foreground(ColorYellow).Render(s)
}

// Cyan returns a cyan-colored string
func Cyan(s string) string {
	return lipgloss.NewStyle().Foreground(ColorCyan).Render(s)
}

// HiBlack returns a high-intensity black (gray) colored string
func HiBlack(s string) string {
	return lipgloss.NewStyle().Foreground(ColorGray).Render(s)
}

// Greenf returns a formatted green-colored string
func Greenf(format string, a ...interface{}) string {
	return Green(fmt.Sprintf(format, a...))
}

// Redf returns a formatted red-colored string
func Redf(format string, a ...interface{}) string {
	return Red(fmt.Sprintf(format, a...))
}

// Yellowf returns a formatted yellow-colored string
func Yellowf(format string, a ...interface{}) string {
	return Yellow(fmt.Sprintf(format, a...))
}

// Cyanf returns a formatted cyan-colored string
func Cyanf(format string, a ...interface{}) string {
	return Cyan(fmt.Sprintf(format, a...))
}

// HiBlackf returns a formatted high-intensity black (gray) colored string
func HiBlackf(format string, a ...interface{}) string {
	return HiBlack(fmt.Sprintf(format, a...))
}
