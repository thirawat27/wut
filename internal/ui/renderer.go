// Package ui provides UI components for WUT
package ui

import (
	"fmt"
	"strings"

	"wut/internal/config"
	"wut/internal/terminal"
)

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
		fmt.Println(content)
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
