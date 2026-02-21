// Package ui provides UI components for WUT
package ui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"wut/internal/config"
	"wut/internal/search"
	"wut/internal/terminal"
)

// KeyMap defines keybindings for the suggestion UI
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Accept   key.Binding
	Quit     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	NextPage key.Binding
	PrevPage key.Binding
	Home     key.Binding
	End      key.Binding
	Search   key.Binding
	Clear    key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "ctrl+p", "ctrl+k"),
			key.WithHelp("↑/ctrl+p", "previous"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "ctrl+n", "ctrl+j"),
			key.WithHelp("↓/ctrl+n", "next"),
		),
		Accept: key.NewBinding(
			key.WithKeys("enter", "tab"),
			key.WithHelp("enter/tab", "select"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c", "q"),
			key.WithHelp("esc/ctrl+c", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "select"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous"),
		),
		NextPage: key.NewBinding(
			key.WithKeys("right", "pgdown", "ctrl+f"),
			key.WithHelp("→/pgdn", "next page"),
		),
		PrevPage: key.NewBinding(
			key.WithKeys("left", "pgup", "ctrl+b"),
			key.WithHelp("←/pgup", "previous page"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "ctrl+a"),
			key.WithHelp("home", "first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "ctrl+e"),
			key.WithHelp("end", "last"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear input"),
		),
	}
}

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
	Prompt      lipgloss.Style
	Match       lipgloss.Style
	Cursor      lipgloss.Style
	
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
		s.Prompt = lipgloss.NewStyle()
		s.Match = lipgloss.NewStyle().Bold(true)
		s.Cursor = lipgloss.NewStyle().Reverse(true)
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
		s.Prompt = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)
		s.Match = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)
		s.Cursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)
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
		s.Prompt = lipgloss.NewStyle().
			Foreground(lipgloss.Color("93")).
			Bold(true)
		s.Match = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
		s.Cursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("93")).
			Bold(true)
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
		s.Prompt = lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")).
			Bold(true)
		s.Match = lipgloss.NewStyle().
			Bold(true)
		s.Cursor = lipgloss.NewStyle().
			Bold(true)
	}
	
	return s
}

// Suggestion represents a suggestion item
type Suggestion struct {
	Command     string
	Score       float64
	Description string
	Source      search.Source
	IsDangerous bool
	Category    string
	
	// Match info for highlighting
	MatchInfo   *search.Result
}

// SuggestModel represents the suggestion UI model
type SuggestModel struct {
	list        list.Model
	textInput   textinput.Model
	keys        KeyMap
	styles      *Styles
	quitting    bool
	err         error
	suggestions []Suggestion
	allItems    []list.Item
	caps        *terminal.Capabilities
	
	// Search state
	searchQuery string
	searcher    *search.Engine
	
	// Selection
	selected    string
	
	// Dimensions
	width       int
	height      int
}

// SuggestionsMsg is sent when suggestions are updated
type SuggestionsMsg struct {
	Suggestions []Suggestion
}

// NewSuggestModel creates a new suggestion model
func NewSuggestModel(suggestions []Suggestion, cfg config.UIConfig, searcher *search.Engine) *SuggestModel {
	caps := terminal.Detect()
	styles := DefaultStyles()
	keys := DefaultKeyMap()
	
	// Create list items
	items := make([]list.Item, len(suggestions))
	for i, s := range suggestions {
		items[i] = suggestionItem{
			title:       s.Command,
			description: s.Description,
			score:       s.Score,
			source:      s.Source,
			isDangerous: s.IsDangerous,
			category:    s.Category,
			matchInfo:   s.MatchInfo,
		}
	}
	
	// Create list with terminal-adapted delegate
	delegate := newSuggestionDelegate(caps, styles)
	
	// Calculate dimensions based on terminal size
	width, height := 80, 20
	if caps.Width > 0 {
		width = caps.Width - 4
	}
	if caps.Height > 0 {
		height = caps.Height - 8
	}
	
	l := list.New(items, delegate, width, height)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(true)
	l.SetShowHelp(false)
	
	if caps.ShouldUseASCII() {
		l.Title = "Suggestions"
	} else {
		l.Title = "WUT - AI Command Helper"
	}
	
	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Type to search commands..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = width - 4
	
	if caps.SupportsColor {
		ti.Prompt = styles.Prompt.Render("> ")
	} else {
		ti.Prompt = "> "
	}
	
	return &SuggestModel{
		list:        l,
		textInput:   ti,
		keys:        keys,
		styles:      styles,
		suggestions: suggestions,
		allItems:    items,
		caps:        caps,
		searcher:    searcher,
		width:       width,
		height:      height,
	}
}

// Init initializes the model
func (m SuggestModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m SuggestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
			
		case key.Matches(msg, m.keys.Accept):
			if i, ok := m.list.SelectedItem().(suggestionItem); ok {
				m.selected = i.title
				m.quitting = true
				return m, tea.Quit
			}
			
		case key.Matches(msg, m.keys.Clear):
			m.textInput.SetValue("")
			m.searchQuery = ""
			m.updateList("")
			return m, nil
			
		case key.Matches(msg, m.keys.Up):
			m.list.CursorUp()
			return m, nil
			
		case key.Matches(msg, m.keys.Down):
			m.list.CursorDown()
			return m, nil
			
		case key.Matches(msg, m.keys.NextPage):
			m.list.NextPage()
			return m, nil
			
		case key.Matches(msg, m.keys.PrevPage):
			m.list.PrevPage()
			return m, nil
			
		case key.Matches(msg, m.keys.Home):
			m.list.Select(0)
			return m, nil
			
		case key.Matches(msg, m.keys.End):
			m.list.Select(len(m.list.Items()) - 1)
			return m, nil
		}
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width - 4)
		m.textInput.Width = msg.Width - 8
		return m, nil
		
	case SuggestionsMsg:
		m.updateSuggestions(msg.Suggestions)
		return m, nil
	}
	
	// Handle text input
	var cmd tea.Cmd
	oldValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)
	
	// Update search if text changed
	newValue := m.textInput.Value()
	if newValue != oldValue {
		m.searchQuery = newValue
		m.updateList(newValue)
	}
	
	// Update list
	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, listCmd)
	
	return m, tea.Batch(cmds...)
}

// updateList updates the list based on search query
func (m *SuggestModel) updateList(query string) {
	if m.searcher == nil {
		return
	}
	
	// Search for commands
	ctx := context.Background()
	results, err := m.searcher.SearchInteractive(ctx, query, nil)
	if err != nil {
		// Fall back to filtering existing items
		m.filterExistingItems(query)
		return
	}
	
	// Convert results to suggestions
	suggestions := make([]Suggestion, len(results))
	for i, r := range results {
		suggestions[i] = Suggestion{
			Command:     r.Command,
			Description: r.Description,
			Score:       r.Score,
			Source:      r.Source,
			IsDangerous: r.IsDangerous,
			Category:    r.Category,
			MatchInfo:   &r,
		}
	}
	
	m.updateSuggestions(suggestions)
}

// filterExistingItems filters existing items locally
func (m *SuggestModel) filterExistingItems(query string) {
	if query == "" {
		m.list.SetItems(m.allItems)
		return
	}
	
	filtered := make([]list.Item, 0)
	for _, item := range m.allItems {
		if i, ok := item.(suggestionItem); ok {
			if strings.Contains(strings.ToLower(i.title), strings.ToLower(query)) ||
			   strings.Contains(strings.ToLower(i.description), strings.ToLower(query)) {
				filtered = append(filtered, item)
			}
		}
	}
	
	if len(filtered) == 0 {
		filtered = m.allItems
	}
	
	m.list.SetItems(filtered)
}

// updateSuggestions updates the suggestions list
func (m *SuggestModel) updateSuggestions(suggestions []Suggestion) {
	m.suggestions = suggestions
	
	items := make([]list.Item, len(suggestions))
	for i, s := range suggestions {
		items[i] = suggestionItem{
			title:       s.Command,
			description: s.Description,
			score:       s.Score,
			source:      s.Source,
			isDangerous: s.IsDangerous,
			category:    s.Category,
			matchInfo:   s.MatchInfo,
			query:       m.searchQuery,
		}
	}
	
	m.list.SetItems(items)
}

// View renders the model
func (m SuggestModel) View() string {
	if m.quitting {
		return ""
	}
	
	var b strings.Builder
	
	// Title
	if m.caps.ShouldUseASCII() {
		b.WriteString("\n")
		b.WriteString("WUT - AI Command Helper\n")
		b.WriteString(strings.Repeat("=", m.width) + "\n")
	} else {
		b.WriteString("\n")
		b.WriteString(m.styles.Title.Render("WUT - AI Command Helper"))
		b.WriteString("\n")
	}
	
	// Search input
	b.WriteString("\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	
	// Results count
	if m.searchQuery != "" {
		count := len(m.list.Items())
		if m.caps.ShouldUseASCII() {
			b.WriteString(fmt.Sprintf("Found %d result(s)\n\n", count))
		} else {
			b.WriteString(m.styles.Help.Render(fmt.Sprintf("Found %d result(s)", count)))
			b.WriteString("\n\n")
		}
	}
	
	// List
	b.WriteString(m.list.View())
	b.WriteString("\n")
	
	// Help
	if m.caps.ShouldUseASCII() {
		b.WriteString("\n")
		b.WriteString("Navigation: Up/Down | Enter: select | Esc: quit | Ctrl+U: clear\n")
	} else {
		b.WriteString("\n")
		help := m.styles.Help.Render("↑/↓: navigate • enter: select • esc: quit • ctrl+u: clear")
		b.WriteString(help)
		b.WriteString("\n")
	}
	
	return b.String()
}

// Selected returns the selected command
func (m SuggestModel) Selected() string {
	return m.selected
}

// SearchQuery returns the current search query
func (m SuggestModel) SearchQuery() string {
	return m.searchQuery
}

// suggestionItem represents a list item
type suggestionItem struct {
	title       string
	description string
	score       float64
	source      search.Source
	isDangerous bool
	category    string
	matchInfo   *search.Result
	query       string
}

func (i suggestionItem) FilterValue() string { return i.title }
func (i suggestionItem) Title() string       { return i.title }
func (i suggestionItem) Description() string { return i.description }

// suggestionDelegate represents list item styling with terminal adaptation
type suggestionDelegate struct {
	caps   *terminal.Capabilities
	styles *Styles
}

func newSuggestionDelegate(caps *terminal.Capabilities, styles *Styles) list.ItemDelegate {
	return &suggestionDelegate{caps: caps, styles: styles}
}

func (d *suggestionDelegate) Height() int  { return 3 }
func (d *suggestionDelegate) Spacing() int { return 1 }

func (d *suggestionDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d *suggestionDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(suggestionItem)
	if !ok {
		return
	}
	
	selected := index == m.Index()
	
	if d.caps.ShouldUseASCII() {
		d.renderASCII(w, i, selected)
	} else {
		d.renderUnicode(w, i, selected)
	}
}

func (d *suggestionDelegate) renderASCII(w io.Writer, i suggestionItem, selected bool) {
	// Title with source indicator
	title := i.title
	if i.source != "" {
		title = fmt.Sprintf("[%s] %s", i.source, title)
	}
	
	if selected {
		title = "> " + title
	} else {
		title = "  " + title
	}
	
	// Dangerous indicator
	if i.isDangerous {
		title = "[!] " + title
	}
	
	fmt.Fprintf(w, "%s\n", title)
	
	// Description
	if i.description != "" {
		desc := fmt.Sprintf("    %s", i.description)
		if selected {
			desc = "    " + i.description
		}
		fmt.Fprintf(w, "%s\n", desc)
	}
	
	// Score
	if i.score > 0 {
		fmt.Fprintf(w, "    Score: %.0f%%\n", i.score*100)
	}
}

func (d *suggestionDelegate) renderUnicode(w io.Writer, i suggestionItem, selected bool) {
	// Source icon mapping
	sourceIcons := map[search.Source]string{
		search.SourceHistory: "📜",
		search.SourceBuiltin: "📦",
		search.SourceAlias:   "🏷️",
		search.SourcePath:    "🛤️",
	}
	
	icon := sourceIcons[i.source]
	if icon == "" {
		icon = "•"
	}
	
	// Build title
	title := i.title
	
	// Highlight matching text if query is present
	if i.query != "" && i.matchInfo != nil {
		title = d.highlightMatch(i.query, i.title)
	}
	
	if selected {
		title = d.styles.Selected.Render("> " + icon + " " + title)
	} else {
		title = d.styles.Normal.Render("  " + icon + " " + title)
	}
	
	// Add dangerous indicator
	if i.isDangerous {
		title = d.styles.Error.Render("⚠") + " " + title
	}
	
	fmt.Fprintf(w, "%s\n", title)
	
	// Description
	if i.description != "" {
		desc := i.description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\n", d.styles.Description.Render("     "+desc))
	}
}

func (d *suggestionDelegate) highlightMatch(query, text string) string {
	if query == "" {
		return text
	}
	
	// Simple highlighting - find and wrap matching parts
	lowerQuery := strings.ToLower(query)
	lowerText := strings.ToLower(text)
	
	idx := strings.Index(lowerText, lowerQuery)
	if idx >= 0 {
		before := text[:idx]
		match := text[idx : idx+len(query)]
		after := text[idx+len(query):]
		return before + d.styles.Match.Render(match) + after
	}
	
	return text
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
		
		if s.Source != "" {
			b.WriteString(fmt.Sprintf(" [%s]", s.Source))
		}
		
		b.WriteString("\n")
	}
	
	return b.String()
}
