package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions for modern, cohesive Deep Navy / Electric Blue UI theming
var (
	// Primary Branding Colors
	ColorPrimary   = lipgloss.Color("#3B82F6") // Electric Blue
	ColorSecondary = lipgloss.Color("#8B5CF6") // Violet / Deep Navy hint
	ColorAccent    = lipgloss.Color("#06B6D4") // Cyan

	// Semantic Colors
	ColorSuccess = lipgloss.Color("#10B981") // Emerald Green
	ColorWarning = lipgloss.Color("#F59E0B") // Amber
	ColorError   = lipgloss.Color("#EF4444") // Red
	ColorMuted   = lipgloss.Color("#6B7280") // Gray (Muted Text)
	ColorText    = lipgloss.Color("#E5E7EB") // Light Gray (Normal text)
)

var (
	// Base text styles
	StylePrimary   = lipgloss.NewStyle().Foreground(ColorPrimary)
	StyleSecondary = lipgloss.NewStyle().Foreground(ColorSecondary)
	StyleAccent    = lipgloss.NewStyle().Foreground(ColorAccent)
	StyleSuccess   = lipgloss.NewStyle().Foreground(ColorSuccess)
	StyleWarning   = lipgloss.NewStyle().Foreground(ColorWarning)
	StyleError     = lipgloss.NewStyle().Foreground(ColorError)
	StyleMuted     = lipgloss.NewStyle().Foreground(ColorMuted)

	// Complex UI Component styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleSubTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	StyleHighlight = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3A8A")). // Dark blue background
			Foreground(lipgloss.Color("#60A5FA")). // Light blue foreground
			Padding(0, 1)
)

// Helper functions for easy color formatting
func Primary(s string) string   { return StylePrimary.Render(s) }
func Secondary(s string) string { return StyleSecondary.Render(s) }
func Accent(s string) string    { return StyleAccent.Render(s) }
func Success(s string) string   { return StyleSuccess.Render(s) }
func Warning(s string) string   { return StyleWarning.Render(s) }
func Error(s string) string     { return StyleError.Render(s) }
func Muted(s string) string     { return StyleMuted.Render(s) }
func Title(s string) string     { return StyleTitle.Render(s) }

// Formatted helpers
func Primaryf(format string, a ...interface{}) string   { return Primary(fmt.Sprintf(format, a...)) }
func Secondaryf(format string, a ...interface{}) string { return Secondary(fmt.Sprintf(format, a...)) }
func Accentf(format string, a ...interface{}) string    { return Accent(fmt.Sprintf(format, a...)) }
func Successf(format string, a ...interface{}) string   { return Success(fmt.Sprintf(format, a...)) }
func Warningf(format string, a ...interface{}) string   { return Warning(fmt.Sprintf(format, a...)) }
func Errorf(format string, a ...interface{}) string     { return Error(fmt.Sprintf(format, a...)) }
func Mutedf(format string, a ...interface{}) string     { return Muted(fmt.Sprintf(format, a...)) }

// Legacy aliases mapping to the new cohesive theme (to keep existing code happy)
func Green(s string) string   { return Success(s) }
func Red(s string) string     { return Error(s) }
func Yellow(s string) string  { return Warning(s) }
func Cyan(s string) string    { return Accent(s) }
func HiBlack(s string) string { return Muted(s) }
func Blue(s string) string    { return Primary(s) }

func Greenf(format string, a ...any) string   { return Successf(format, a...) }
func Redf(format string, a ...any) string     { return Errorf(format, a...) }
func Yellowf(format string, a ...any) string  { return Warningf(format, a...) }
func Cyanf(format string, a ...any) string    { return Accentf(format, a...) }
func HiBlackf(format string, a ...any) string { return Mutedf(format, a...) }
