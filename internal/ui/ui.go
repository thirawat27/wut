// Package ui provides UI components for WUT
package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"wut/internal/config"
	"wut/internal/terminal"
)

// Styles holds UI styles with terminal adaptation
type Styles struct {
	Title       lipgloss.Style
	Selected    lipgloss.Style
	Normal      lipgloss.Style
	Description lipgloss.Style
	Help        lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
	Border      lipgloss.Style
	
	// Terminal capabilities
	caps *terminal.Capabilities
}

// DefaultStyles returns default UI styles adapted to terminal
func DefaultStyles() *Styles {
	caps := terminal.Detect()
	s := &Styles{caps: caps}
	
	// Adapt styles based on terminal capabilities
	if !caps.SupportsColor {
		// ASCII-only styles
		s.Title = lipgloss.NewStyle().Bold(true)
		s.Selected = lipgloss.NewStyle().Bold(true)
		s.Normal = lipgloss.NewStyle()
		s.Description = lipgloss.NewStyle()
		s.Help = lipgloss.NewStyle()
		s.Error = lipgloss.NewStyle()
		s.Success = lipgloss.NewStyle()
		s.Border = lipgloss.NewStyle()
	} else if caps.SupportsTrueColor {
		// Full color support
		s.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)
		s.Selected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)
		s.Normal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
		s.Description = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true)
		s.Help = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
		s.Error = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))
		s.Success = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))
		s.Border = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED"))
	} else if caps.Supports256Color {
		// 256 color support
		s.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("93")).
			MarginBottom(1)
		s.Selected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)
		s.Normal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))
		s.Description = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")).
			Italic(true)
		s.Help = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))
		s.Error = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		s.Success = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))
		s.Border = lipgloss.NewStyle().
			Foreground(lipgloss.Color("93"))
	} else {
		// Basic 16 color support
		s.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("5"))
		s.Selected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)
		s.Normal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))
		s.Description = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
		s.Error = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))
		s.Success = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
		s.Border = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5"))
	}
	
	return s
}

// Renderer provides UI rendering capabilities with terminal adaptation
type Renderer struct {
	config config.UIConfig
	Styles *Styles
	caps   *terminal.Capabilities
}

// NewRenderer creates a new UI renderer
func NewRenderer(cfg config.UIConfig) *Renderer {
	caps := terminal.Detect()
	return &Renderer{
		config: cfg,
		Styles: DefaultStyles(),
		caps:   caps,
	}
}

// PrintHeader prints a header
func (r *Renderer) PrintHeader(title string) {
	if r.caps.ShouldUseASCII() {
		fmt.Println("=== " + title + " ===")
	} else {
		fmt.Println(r.Styles.Title.Render(title))
	}
}

// PrintBox prints a box around content
func (r *Renderer) PrintBox(content string) {
	if r.caps.ShouldUseASCII() {
		lines := strings.Split(content, "\n")
		maxLen := 0
		for _, line := range lines {
			if len(line) > maxLen {
				maxLen = len(line)
			}
		}
		
		fmt.Println("+" + strings.Repeat("-", maxLen+2) + "+")
		for _, line := range lines {
			fmt.Printf("| %s%s |\n", line, strings.Repeat(" ", maxLen-len(line)))
		}
		fmt.Println("+" + strings.Repeat("-", maxLen+2) + "+")
	} else {
		style := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2)
		fmt.Println(style.Render(content))
	}
}

// Icon returns an icon adapted to terminal capabilities
func (r *Renderer) Icon(name string) string {
	if r.caps == nil {
		r.caps = terminal.Detect()
	}
	
	icons := map[string]map[string]string{
		"check": {
			"emoji": "✓",
			"ascii": "[OK]",
			"nerd":  "\uf00c",
		},
		"cross": {
			"emoji": "✗",
			"ascii": "[X]",
			"nerd":  "\uf00d",
		},
		"info": {
			"emoji": "ℹ",
			"ascii": "[i]",
			"nerd":  "\uf129",
		},
		"warning": {
			"emoji": "⚠",
			"ascii": "[!]",
			"nerd":  "\uf071",
		},
		"rocket": {
			"emoji": "🚀",
			"ascii": "=>",
			"nerd":  "\uf135",
		},
		"star": {
			"emoji": "⭐",
			"ascii": "*",
			"nerd":  "\uf005",
		},
		"arrow": {
			"emoji": "→",
			"ascii": "->",
			"nerd":  "\uf061",
		},
		"bullet": {
			"emoji": "•",
			"ascii": "*",
			"nerd":  "\uf111",
		},
		"folder": {
			"emoji": "📁",
			"ascii": "[DIR]",
			"nerd":  "\uf07b",
		},
		"file": {
			"emoji": "📄",
			"ascii": "[FILE]",
			"nerd":  "\uf15b",
		},
	}
	
	iconSet, ok := icons[name]
	if !ok {
		return ""
	}
	
	if r.caps.ShouldUseNerdFonts() {
		return iconSet["nerd"]
	}
	if r.caps.ShouldUseEmoji() {
		return iconSet["emoji"]
	}
	return iconSet["ascii"]
}

// Suggestion represents a suggestion item
type Suggestion struct {
	Command     string
	Score       float64
	Description string
}

// SuggestModel represents the suggestion UI model
type SuggestModel struct {
	list        list.Model
	textInput   textinput.Model
	quitting    bool
	err         error
	suggestions []Suggestion
	caps        *terminal.Capabilities
}

// NewSuggestModel creates a new suggestion model
func NewSuggestModel(suggestions []Suggestion, cfg config.UIConfig) *SuggestModel {
	caps := terminal.Detect()
	
	// Create list items
	items := make([]list.Item, len(suggestions))
	for i, s := range suggestions {
		items[i] = suggestionItem{
			title:       s.Command,
			description: s.Description,
			score:       s.Score,
		}
	}
	
	// Create list with terminal-adapted delegate
	delegate := newSuggestionDelegate(caps)
	l := list.New(items, delegate, 80, 20)
	
	if caps.ShouldUseASCII() {
		l.Title = "Suggestions"
	} else {
		l.Title = "WUT - Command Suggestions"
	}
	
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	
	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()
	
	return &SuggestModel{
		list:        l,
		textInput:   ti,
		suggestions: suggestions,
		caps:        caps,
	}
}

// Init initializes the model
func (m SuggestModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m SuggestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.quitting = true
			return m, tea.Quit
			
		case "enter":
			if i, ok := m.list.SelectedItem().(suggestionItem); ok {
				fmt.Println(i.title)
				m.quitting = true
				return m, tea.Quit
			}
		}
		
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}
	
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the model
func (m SuggestModel) View() string {
	if m.quitting {
		return ""
	}
	
	var b strings.Builder
	
	if m.caps.ShouldUseASCII() {
		b.WriteString("\n")
		b.WriteString("WUT - Command Suggestions\n")
		b.WriteString(strings.Repeat("=", 25) + "\n\n")
		b.WriteString(m.list.View())
		b.WriteString("\n\n")
		b.WriteString("Navigation: Up/Down arrows | Enter: select | q: quit\n")
	} else {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Render("WUT - Command Suggestions"))
		b.WriteString("\n\n")
		b.WriteString(m.list.View())
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("↑/↓: navigate • enter: select • q: quit"))
		b.WriteString("\n")
	}
	
	return b.String()
}

// suggestionItem represents a list item
type suggestionItem struct {
	title       string
	description string
	score       float64
}

func (i suggestionItem) FilterValue() string { return i.title }
func (i suggestionItem) Title() string       { return i.title }
func (i suggestionItem) Description() string { return i.description }

// suggestionDelegate represents list item styling with terminal adaptation
type suggestionDelegate struct {
	caps *terminal.Capabilities
}

func newSuggestionDelegate(caps *terminal.Capabilities) list.ItemDelegate {
	return &suggestionDelegate{caps: caps}
}

func (d *suggestionDelegate) Height() int  { return 2 }
func (d *suggestionDelegate) Spacing() int { return 1 }

func (d *suggestionDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d *suggestionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(suggestionItem)
	if !ok {
		return
	}
	
	if d.caps.ShouldUseASCII() {
		// ASCII-only rendering
		var prefix string
		if index == m.Index() {
			prefix = "> "
		} else {
			prefix = "  "
		}
		
		fmt.Fprintf(w, "%s%s\n", prefix, i.title)
		if i.description != "" {
			fmt.Fprintf(w, "%s  %s\n", prefix, i.description)
		}
	} else {
		// Unicode rendering
		normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true)
		
		var title string
		if index == m.Index() {
			title = selectedStyle.Render("> " + i.title)
		} else {
			title = normalStyle.Render("  " + i.title)
		}
		
		fmt.Fprintf(w, "%s\n", title)
		if i.description != "" {
			fmt.Fprintf(w, "%s\n", descStyle.Render("    "+i.description))
		}
	}
}

// SimpleOutput prints suggestions in simple text format (for non-TTY)
func SimpleOutput(suggestions []Suggestion, showScores bool) string {
	var b strings.Builder
	
	for i, s := range suggestions {
		b.WriteString(fmt.Sprintf("%d. %s", i+1, s.Command))
		
		if showScores {
			b.WriteString(fmt.Sprintf(" (%.0f%%)", s.Score*100))
		}
		
		if s.Description != "" {
			b.WriteString(fmt.Sprintf(" - %s", s.Description))
		}
		
		b.WriteString("\n")
	}
	
	return b.String()
}
