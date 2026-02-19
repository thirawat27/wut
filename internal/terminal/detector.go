// Package terminal provides terminal detection and capability analysis
package terminal

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Capabilities represents terminal capabilities
type Capabilities struct {
	// Basic features
	IsTTY           bool
	SupportsColor   bool
	Supports256Color bool
	SupportsTrueColor bool
	
	// Unicode and emoji
	SupportsUnicode bool
	SupportsEmoji   bool
	SupportsNerdFonts bool
	
	// Advanced features
	SupportsHyperlinks bool
	SupportsImages     bool
	SupportsMouse      bool
	
	// Terminal info
	Name        string
	Program     string
	Version     string
	
	// Screen dimensions
	Width  int
	Height int
}

// Detector detects terminal capabilities
type Detector struct {
	env map[string]string
}

// NewDetector creates a new terminal detector
func NewDetector() *Detector {
	// Collect relevant environment variables
	env := make(map[string]string)
	relevantVars := []string{
		"TERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION",
		"COLORTERM", "COLORFGBG", "TERMINFO",
		"WT_SESSION", "WT_PROFILE_ID",  // Windows Terminal
		"ITERM_SESSION_ID", "ITERM_PROFILE", // iTerm2
		"ALACRITTY_LOG", "ALACRITTY_WINDOW_ID", // Alacritty
		"WEZTERM_EXECUTABLE", "WEZTERM_PANE", // WezTerm
		"KITTY_PID", "KITTY_WINDOW_ID", // Kitty
		"GNOME_TERMINAL_SERVICE", // GNOME Terminal
		"KONSOLE_VERSION", // Konsole
		"TMUX", "TMUX_PANE", // Tmux
		"STY", // Screen
		"NO_COLOR", "FORCE_COLOR",
		"LANG", "LC_ALL", "LC_CTYPE",
		"COMSPEC", "PROMPT", // Windows
	}
	
	for _, key := range relevantVars {
		if val := os.Getenv(key); val != "" {
			env[key] = val
		}
	}
	
	return &Detector{env: env}
}

// Detect detects terminal capabilities
func (d *Detector) Detect() *Capabilities {
	caps := &Capabilities{
		IsTTY: isTTY(),
	}
	
	// Detect terminal program
	caps.Name = d.detectTerminalName()
	caps.Program = d.env["TERM_PROGRAM"]
	caps.Version = d.env["TERM_PROGRAM_VERSION"]
	
	// Detect color support
	caps.SupportsColor = d.detectColorSupport()
	caps.Supports256Color = d.detect256ColorSupport()
	caps.SupportsTrueColor = d.detectTrueColorSupport()
	
	// Detect Unicode support
	caps.SupportsUnicode = d.detectUnicodeSupport()
	caps.SupportsEmoji = d.detectEmojiSupport()
	caps.SupportsNerdFonts = d.detectNerdFontSupport()
	
	// Detect advanced features
	caps.SupportsHyperlinks = d.detectHyperlinkSupport()
	
	return caps
}

// detectTerminalName detects the terminal name
func (d *Detector) detectTerminalName() string {
	// Check specific terminal environment variables
	if d.env["WT_SESSION"] != "" {
		return "windows-terminal"
	}
	if d.env["ITERM_SESSION_ID"] != "" {
		return "iterm2"
	}
	if d.env["ALACRITTY_LOG"] != "" || d.env["ALACRITTY_WINDOW_ID"] != "" {
		return "alacritty"
	}
	if d.env["WEZTERM_EXECUTABLE"] != "" {
		return "wezterm"
	}
	if d.env["KITTY_PID"] != "" {
		return "kitty"
	}
	if d.env["GNOME_TERMINAL_SERVICE"] != "" {
		return "gnome-terminal"
	}
	if d.env["KONSOLE_VERSION"] != "" {
		return "konsole"
	}
	if d.env["TMUX"] != "" {
		return "tmux"
	}
	if d.env["STY"] != "" {
		return "screen"
	}
	
	// Check TERM variable
	term := d.env["TERM"]
	if strings.Contains(term, "xterm") {
		return "xterm"
	}
	if strings.Contains(term, "vt100") {
		return "vt100"
	}
	if strings.Contains(term, "linux") {
		return "linux-console"
	}
	if strings.Contains(term, "dumb") {
		return "dumb"
	}
	
	// Windows-specific
	if runtime.GOOS == "windows" {
		if os.Getenv("WT_SESSION") != "" {
			return "windows-terminal"
		}
		if os.Getenv("ConEmuPID") != "" {
			return "conemu"
		}
		if os.Getenv("CMDER_ROOT") != "" {
			return "cmder"
		}
		return "windows-console"
	}
	
	return "unknown"
}

// detectColorSupport detects basic color support
func (d *Detector) detectColorSupport() bool {
	// Check NO_COLOR
	if d.env["NO_COLOR"] != "" {
		return false
	}
	
	// Check TERM
	term := d.env["TERM"]
	if term == "dumb" {
		return false
	}
	
	// Check COLORTERM
	if d.env["COLORTERM"] != "" {
		return true
	}
	
	// Check for color-capable terminals
	if strings.Contains(term, "color") || strings.Contains(term, "xterm") ||
	   strings.Contains(term, "screen") || strings.Contains(term, "tmux") ||
	   strings.Contains(term, "rxvt") || strings.Contains(term, "vt100") {
		return true
	}
	
	// Windows 10+ supports colors
	if runtime.GOOS == "windows" {
		return true
	}
	
	return false
}

// detect256ColorSupport detects 256 color support
func (d *Detector) detect256ColorSupport() bool {
	term := d.env["TERM"]
	
	// Check for 256color in TERM
	if strings.Contains(term, "256color") {
		return true
	}
	
	// Most modern terminals support 256 colors
	if d.env["COLORTERM"] != "" {
		return true
	}
	
	// Check terminal programs known to support 256 colors
	knownTerminals := []string{
		"windows-terminal", "iterm2", "alacritty", "wezterm",
		"kitty", "gnome-terminal", "konsole", "tmux", "screen",
	}
	name := d.detectTerminalName()
	for _, t := range knownTerminals {
		if name == t {
			return true
		}
	}
	
	return false
}

// detectTrueColorSupport detects true color (24-bit) support
func (d *Detector) detectTrueColorSupport() bool {
	// Check COLORTERM
	colorterm := d.env["COLORTERM"]
	if colorterm == "truecolor" || colorterm == "24bit" {
		return true
	}
	
	// Known terminals with true color support
	trueColorTerminals := []string{
		"windows-terminal", "iterm2", "alacritty", "wezterm",
		"kitty", "gnome-terminal", "konsole", "tmux",
	}
	name := d.detectTerminalName()
	for _, t := range trueColorTerminals {
		if name == t {
			return true
		}
	}
	
	return false
}

// detectUnicodeSupport detects Unicode support
func (d *Detector) detectUnicodeSupport() bool {
	// Check LANG
	lang := d.env["LANG"]
	if strings.Contains(lang, "UTF-8") || strings.Contains(lang, "utf8") ||
	   strings.Contains(lang, "UTF8") {
		return true
	}
	
	// Check LC_ALL and LC_CTYPE
	for _, key := range []string{"LC_ALL", "LC_CTYPE"} {
		val := d.env[key]
		if strings.Contains(val, "UTF") {
			return true
		}
	}
	
	// Most modern terminals support Unicode
	if d.detectTerminalName() != "dumb" && d.detectTerminalName() != "unknown" {
		return true
	}
	
	return false
}

// detectEmojiSupport detects emoji support
func (d *Detector) detectEmojiSupport() bool {
	// Most modern terminals with Unicode support also support emoji
	if !d.detectUnicodeSupport() {
		return false
	}
	
	// Known terminals with good emoji support
	emojiTerminals := []string{
		"windows-terminal", "iterm2", "alacritty", "wezterm",
		"kitty", "gnome-terminal", "konsole",
	}
	name := d.detectTerminalName()
	for _, t := range emojiTerminals {
		if name == t {
			return true
		}
	}
	
	return false
}

// detectNerdFontSupport detects Nerd Font support
func (d *Detector) detectNerdFontSupport() bool {
	// This is hard to detect reliably
	// Check for terminals that commonly use Nerd Fonts
	nerdFontTerminals := []string{
		"alacritty", "wezterm", "kitty", "iterm2",
	}
	name := d.detectTerminalName()
	for _, t := range nerdFontTerminals {
		if name == t {
			return true
		}
	}
	
	return false
}

// detectHyperlinkSupport detects OSC 8 hyperlink support
func (d *Detector) detectHyperlinkSupport() bool {
	// Known terminals with hyperlink support
	hyperlinkTerminals := []string{
		"windows-terminal", "iterm2", "alacritty", "wezterm",
		"kitty", "gnome-terminal", "konsole",
	}
	name := d.detectTerminalName()
	for _, t := range hyperlinkTerminals {
		if name == t {
			return true
		}
	}
	
	return false
}

// isTTY checks if stdout is a TTY
func isTTY() bool {
	if runtime.GOOS == "windows" {
		// On Windows, check if we're in a console
		return true // Simplified
	}
	
	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	
	return (fileInfo.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

// GetRecommendedTheme returns the recommended theme based on terminal capabilities
func (c *Capabilities) GetRecommendedTheme() string {
	if !c.SupportsColor {
		return "none"
	}
	if c.SupportsTrueColor {
		return "truecolor"
	}
	if c.Supports256Color {
		return "256"
	}
	return "basic"
}

// ShouldUseASCII returns true if the terminal should use ASCII-only output
func (c *Capabilities) ShouldUseASCII() bool {
	return !c.SupportsUnicode || c.Name == "dumb" || !c.IsTTY
}

// ShouldUseEmoji returns true if the terminal supports emoji
func (c *Capabilities) ShouldUseEmoji() bool {
	return c.SupportsEmoji && c.IsTTY
}

// ShouldUseNerdFonts returns true if the terminal likely supports Nerd Fonts
func (c *Capabilities) ShouldUseNerdFonts() bool {
	return c.SupportsNerdFonts && c.IsTTY
}

// GetTerminalInfo returns a summary of terminal information
func (c *Capabilities) GetTerminalInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":               c.Name,
		"program":            c.Program,
		"version":            c.Version,
		"is_tty":             c.IsTTY,
		"supports_color":     c.SupportsColor,
		"supports_256color":  c.Supports256Color,
		"supports_truecolor": c.SupportsTrueColor,
		"supports_unicode":   c.SupportsUnicode,
		"supports_emoji":     c.SupportsEmoji,
		"supports_nerdfonts": c.SupportsNerdFonts,
		"recommended_theme":  c.GetRecommendedTheme(),
	}
}

// Detect is a convenience function to detect terminal capabilities
func Detect() *Capabilities {
	return NewDetector().Detect()
}

// ForceColor forces color support detection
func ForceColor() bool {
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if val, err := strconv.ParseBool(os.Getenv("FORCE_COLOR")); err == nil {
		return val
	}
	return false
}
