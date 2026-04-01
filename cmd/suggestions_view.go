package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	appctx "wut/internal/context"
	"wut/internal/smart"
)

type smartSection struct {
	Title       string
	Description string
	Accent      lipgloss.Color
	Suggestions []smart.Suggestion
}

type smartBadge struct {
	Label  string
	FG     lipgloss.Color
	BG     lipgloss.Color
	Border lipgloss.Color
}

func renderSmartView(query string, ctx *appctx.Context, suggestions []smart.Suggestion) {
	width := smartTerminalWidth()

	fmt.Println()
	fmt.Println(renderSmartHeader(query, suggestions, width))
	fmt.Println()
	fmt.Println(renderSmartContextCard(ctx, width))

	sections := buildSmartSections(suggestions)
	for _, section := range sections {
		fmt.Println()
		fmt.Println(renderSmartSection(section, width))
	}

	fmt.Println()
	fmt.Println(renderSmartFooter(query, suggestions, width))
}

func smartTerminalWidth() int {
	width := 96
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w - 2
	}
	if width < 64 {
		width = 64
	}
	if width > 120 {
		width = 120
	}
	return width
}

func renderSmartHeader(query string, suggestions []smart.Suggestion, width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC"))

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CBD5E1"))

	topCommand := "No suggestions"
	if len(suggestions) > 0 {
		topCommand = suggestions[0].Command
	}

	badges := []string{
		renderSmartBadge(smartBadge{
			Label:  smartModeLabel(query),
			FG:     lipgloss.Color("#E0F2FE"),
			BG:     lipgloss.Color("#0C4A6E"),
			Border: lipgloss.Color("#0284C7"),
		}),
		renderSmartBadge(smartBadge{
			Label:  fmt.Sprintf("%d result%s", len(suggestions), smartPlural(len(suggestions))),
			FG:     lipgloss.Color("#ECFCCB"),
			BG:     lipgloss.Color("#365314"),
			Border: lipgloss.Color("#65A30D"),
		}),
	}

	for _, badge := range buildSmartHeaderBadges(suggestions) {
		badges = append(badges, renderSmartBadge(badge))
	}

	topLine := titleStyle.Render("Smart Suggestions")
	if query != "" {
		topLine += "\n" + subtitleStyle.Render(fmt.Sprintf("Query: %s", query))
	} else {
		topLine += "\n" + subtitleStyle.Render("Mode: context-aware recommendations")
	}

	leadLabel := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#93C5FD")).
		Render("Top match")

	content := strings.Join([]string{
		topLine,
		renderSmartBadgeRows(badges, width-8),
		leadLabel + "\n" + renderSmartPrefixedBlock("  ", topCommand, width-8, lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))),
	}, "\n\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#2563EB")).
		Background(lipgloss.Color("#0F172A")).
		Padding(1, 2).
		Width(width).
		Render(content)
}

func renderSmartContextCard(ctx *appctx.Context, width int) string {
	if ctx == nil {
		return ""
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E2E8F0")).
		Render("Current Context")

	workspace := filepath.Base(ctx.WorkingDir)
	if workspace == "." || workspace == "/" || workspace == "\\" || workspace == "" {
		workspace = ctx.WorkingDir
	}

	pills := []string{
		renderSmartBadge(smartBadge{
			Label:  "Workspace " + workspace,
			FG:     lipgloss.Color("#EDE9FE"),
			BG:     lipgloss.Color("#4C1D95"),
			Border: lipgloss.Color("#7C3AED"),
		}),
		renderSmartBadge(smartBadge{
			Label:  "Type " + smartContextValue(ctx.ProjectType, "unknown"),
			FG:     lipgloss.Color("#D1FAE5"),
			BG:     lipgloss.Color("#064E3B"),
			Border: lipgloss.Color("#10B981"),
		}),
		renderSmartBadge(smartBadge{
			Label:  "Shell " + smartDisplayContextToken(ctx.Shell, "unknown"),
			FG:     lipgloss.Color("#E0F2FE"),
			BG:     lipgloss.Color("#0C4A6E"),
			Border: lipgloss.Color("#0891B2"),
		}),
		renderSmartBadge(smartBadge{
			Label:  "OS " + smartContextValue(ctx.OS, "unknown"),
			FG:     lipgloss.Color("#FDE68A"),
			BG:     lipgloss.Color("#78350F"),
			Border: lipgloss.Color("#D97706"),
		}),
	}

	if ctx.IsGitRepo {
		pills = append(pills, renderSmartBadge(smartBadge{
			Label:  "Branch " + smartContextValue(ctx.GitBranch, "detached"),
			FG:     lipgloss.Color("#DCFCE7"),
			BG:     lipgloss.Color("#14532D"),
			Border: lipgloss.Color("#16A34A"),
		}))

		if ctx.GitStatus.IsClean {
			pills = append(pills, renderSmartBadge(smartBadge{
				Label:  "Clean",
				FG:     lipgloss.Color("#D1FAE5"),
				BG:     lipgloss.Color("#064E3B"),
				Border: lipgloss.Color("#10B981"),
			}))
		} else {
			pills = append(pills, renderSmartBadge(smartBadge{
				Label:  "Dirty",
				FG:     lipgloss.Color("#FDE68A"),
				BG:     lipgloss.Color("#78350F"),
				Border: lipgloss.Color("#F59E0B"),
			}))
		}

		if ctx.GitStatus.Ahead > 0 {
			pills = append(pills, renderSmartBadge(smartBadge{
				Label:  fmt.Sprintf("Ahead %d", ctx.GitStatus.Ahead),
				FG:     lipgloss.Color("#DBEAFE"),
				BG:     lipgloss.Color("#1E3A8A"),
				Border: lipgloss.Color("#3B82F6"),
			}))
		}
		if ctx.GitStatus.Behind > 0 {
			pills = append(pills, renderSmartBadge(smartBadge{
				Label:  fmt.Sprintf("Behind %d", ctx.GitStatus.Behind),
				FG:     lipgloss.Color("#FECACA"),
				BG:     lipgloss.Color("#7F1D1D"),
				Border: lipgloss.Color("#EF4444"),
			}))
		}
	}

	pathLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Render(ctx.WorkingDir)

	content := strings.Join([]string{
		title,
		renderSmartBadgeRows(pills, width-8),
		pathLine,
	}, "\n\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(1, 2).
		Width(width).
		Render(content)
}

func buildSmartSections(suggestions []smart.Suggestion) []smartSection {
	if len(suggestions) == 0 {
		return nil
	}

	sections := []smartSection{
		{
			Title:       "Best Match",
			Description: "Highest-confidence suggestion after scoring all sources.",
			Accent:      lipgloss.Color("#22C55E"),
			Suggestions: []smart.Suggestion{suggestions[0]},
		},
	}

	grouped := map[string][]smart.Suggestion{
		"History":   {},
		"Context":   {},
		"Reference": {},
		"Explore":   {},
		"Other":     {},
	}

	for _, suggestion := range suggestions[1:] {
		grouped[smartSuggestionBucket(suggestion)] = append(grouped[smartSuggestionBucket(suggestion)], suggestion)
	}

	appendSection := func(key, title, description string, accent lipgloss.Color) {
		if len(grouped[key]) == 0 {
			return
		}
		sections = append(sections, smartSection{
			Title:       title,
			Description: description,
			Accent:      accent,
			Suggestions: grouped[key],
		})
	}

	appendSection("History", "From Your History", "Commands you actually ran before, ranked by fit and freshness.", lipgloss.Color("#8B5CF6"))
	appendSection("Context", "Contextual Picks", "Commands inferred from the current repo, branch, and project type.", lipgloss.Color("#06B6D4"))
	appendSection("Reference", "Command Reference", "Suggestions pulled from the local command catalog / TLDR cache.", lipgloss.Color("#F59E0B"))
	appendSection("Explore", "Explore", "Loose matches and discovery-oriented suggestions.", lipgloss.Color("#64748B"))
	appendSection("Other", "Other Matches", "Suggestions that did not fit the primary buckets.", lipgloss.Color("#94A3B8"))

	return sections
}

func renderSmartSection(section smartSection, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(section.Accent).
		Render(section.Title)

	count := renderSmartBadge(smartBadge{
		Label:  fmt.Sprintf("%d", len(section.Suggestions)),
		FG:     lipgloss.Color("#E2E8F0"),
		BG:     lipgloss.Color("#1E293B"),
		Border: section.Accent,
	})

	header := title + " " + count
	if section.Description != "" {
		header += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(section.Description)
	}

	parts := []string{header}
	for i, suggestion := range section.Suggestions {
		parts = append(parts, renderSmartSuggestionCard(i+1, suggestion, width, i == 0 && section.Title == "Best Match", section.Accent))
	}

	return strings.Join(parts, "\n\n")
}

func renderSmartSuggestionCard(rank int, suggestion smart.Suggestion, width int, featured bool, accent lipgloss.Color) string {
	cardWidth := width
	contentWidth := width - 8
	if contentWidth < 40 {
		contentWidth = 40
	}

	rankBadge := renderSmartBadge(smartBadge{
		Label:  fmt.Sprintf("#%d", rank),
		FG:     lipgloss.Color("#F8FAFC"),
		BG:     accent,
		Border: accent,
	})

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))
	if featured {
		commandStyle = commandStyle.Foreground(lipgloss.Color("#DCFCE7"))
	}

	header := renderSmartPrefixedBlock(rankBadge+"  ", suggestion.Command, contentWidth, commandStyle)

	badges := []string{
		renderSmartBadge(smartConfidenceBadge(suggestion, featured)),
	}
	for _, badge := range smartSourceBadges(suggestion.Source) {
		badges = append(badges, renderSmartBadge(badge))
	}
	if suggestion.IsPerfectMatch {
		badges = append(badges, renderSmartBadge(smartBadge{
			Label:  "Exact",
			FG:     lipgloss.Color("#DCFCE7"),
			BG:     lipgloss.Color("#14532D"),
			Border: lipgloss.Color("#16A34A"),
		}))
	}
	if suggestion.ContextMatch >= 0.3 {
		badges = append(badges, renderSmartBadge(smartBadge{
			Label:  "Local context",
			FG:     lipgloss.Color("#E0F2FE"),
			BG:     lipgloss.Color("#164E63"),
			Border: lipgloss.Color("#06B6D4"),
		}))
	}
	if suggestion.UsageCount > 1 {
		badges = append(badges, renderSmartBadge(smartBadge{
			Label:  fmt.Sprintf("Used %d times", suggestion.UsageCount),
			FG:     lipgloss.Color("#EDE9FE"),
			BG:     lipgloss.Color("#4C1D95"),
			Border: lipgloss.Color("#8B5CF6"),
		}))
	}
	if age := smartRelativeAgeShort(suggestion.LastUsed); age != "" {
		badges = append(badges, renderSmartBadge(smartBadge{
			Label:  age,
			FG:     lipgloss.Color("#CBD5E1"),
			BG:     lipgloss.Color("#1F2937"),
			Border: lipgloss.Color("#475569"),
		}))
	}

	parts := []string{
		header,
		renderSmartBadgeRows(badges, contentWidth),
	}
	if suggestion.Description != "" {
		desc := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Render(wrapText(suggestion.Description, contentWidth))
		parts = append(parts, desc)
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 1).
		Width(cardWidth)

	if featured {
		style = style.Background(lipgloss.Color("#0B1220"))
	}

	return style.Render(strings.Join(parts, "\n\n"))
}

func renderSmartFooter(query string, suggestions []smart.Suggestion, width int) string {
	tips := []string{
		"Use `wut ? \"<query>\"` for a fast shortcut.",
	}
	if query != "" {
		tips = append(tips, fmt.Sprintf("Need raw log matches too? Run `wut history --search %q`.", query))
	} else {
		tips = append(tips, "Add a short query like `git status` or `docker logs` to focus the ranking.")
	}
	if len(suggestions) > 0 {
		tips = append(tips, "The first card is the highest-confidence pick after merging history, context, and catalog signals.")
	}

	content := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94A3B8")).
		Render("Tips\n" + wrapText(strings.Join(tips, "  "), width-8))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#334155")).
		Padding(1, 2).
		Width(width).
		Render(content)
}

func buildSmartHeaderBadges(suggestions []smart.Suggestion) []smartBadge {
	counts := make(map[string]int)
	for _, suggestion := range suggestions {
		counts[smartSuggestionBucket(suggestion)]++
	}

	order := []struct {
		Key   string
		Label string
		Color lipgloss.Color
	}{
		{Key: "History", Label: "History", Color: lipgloss.Color("#8B5CF6")},
		{Key: "Context", Label: "Context", Color: lipgloss.Color("#06B6D4")},
		{Key: "Reference", Label: "Reference", Color: lipgloss.Color("#F59E0B")},
		{Key: "Explore", Label: "Explore", Color: lipgloss.Color("#64748B")},
	}

	badges := make([]smartBadge, 0, len(order))
	for _, item := range order {
		if counts[item.Key] == 0 {
			continue
		}
		badges = append(badges, smartBadge{
			Label:  fmt.Sprintf("%s %d", item.Label, counts[item.Key]),
			FG:     lipgloss.Color("#E2E8F0"),
			BG:     lipgloss.Color("#1E293B"),
			Border: item.Color,
		})
	}
	return badges
}

func smartSourceBadges(source string) []smartBadge {
	parts := strings.Split(source, " + ")
	badges := make([]smartBadge, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		label, badge := smartSourceBadge(part)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		badges = append(badges, badge)
	}
	return badges
}

func smartSourceBadge(source string) (string, smartBadge) {
	switch {
	case strings.Contains(source, "Smart History"):
		return "History", smartBadge{
			Label:  "History",
			FG:     lipgloss.Color("#EDE9FE"),
			BG:     lipgloss.Color("#4C1D95"),
			Border: lipgloss.Color("#8B5CF6"),
		}
	case strings.Contains(source, "Context"):
		return "Context", smartBadge{
			Label:  "Context",
			FG:     lipgloss.Color("#E0F2FE"),
			BG:     lipgloss.Color("#164E63"),
			Border: lipgloss.Color("#06B6D4"),
		}
	case strings.Contains(source, "Quick"):
		return "Quick", smartBadge{
			Label:  "Quick",
			FG:     lipgloss.Color("#DBEAFE"),
			BG:     lipgloss.Color("#1E3A8A"),
			Border: lipgloss.Color("#3B82F6"),
		}
	case strings.Contains(source, "Command DB"):
		return "Reference", smartBadge{
			Label:  "Reference",
			FG:     lipgloss.Color("#FEF3C7"),
			BG:     lipgloss.Color("#78350F"),
			Border: lipgloss.Color("#F59E0B"),
		}
	case strings.Contains(source, "Fuzzy"):
		return "Fuzzy", smartBadge{
			Label:  "Fuzzy",
			FG:     lipgloss.Color("#E2E8F0"),
			BG:     lipgloss.Color("#334155"),
			Border: lipgloss.Color("#64748B"),
		}
	case strings.Contains(source, "Common"):
		return "Common", smartBadge{
			Label:  "Common",
			FG:     lipgloss.Color("#E2E8F0"),
			BG:     lipgloss.Color("#334155"),
			Border: lipgloss.Color("#94A3B8"),
		}
	default:
		cleaned := strings.TrimSpace(source)
		return cleaned, smartBadge{
			Label:  cleaned,
			FG:     lipgloss.Color("#E2E8F0"),
			BG:     lipgloss.Color("#1F2937"),
			Border: lipgloss.Color("#475569"),
		}
	}
}

func smartConfidenceBadge(suggestion smart.Suggestion, featured bool) smartBadge {
	switch {
	case featured || suggestion.IsPerfectMatch || suggestion.Score >= 2.3:
		return smartBadge{
			Label:  "Top",
			FG:     lipgloss.Color("#DCFCE7"),
			BG:     lipgloss.Color("#14532D"),
			Border: lipgloss.Color("#22C55E"),
		}
	case suggestion.Score >= 1.4:
		return smartBadge{
			Label:  "Strong",
			FG:     lipgloss.Color("#DBEAFE"),
			BG:     lipgloss.Color("#1E3A8A"),
			Border: lipgloss.Color("#3B82F6"),
		}
	case suggestion.Score >= 0.85:
		return smartBadge{
			Label:  "Good",
			FG:     lipgloss.Color("#FEF3C7"),
			BG:     lipgloss.Color("#78350F"),
			Border: lipgloss.Color("#F59E0B"),
		}
	default:
		return smartBadge{
			Label:  "Related",
			FG:     lipgloss.Color("#E2E8F0"),
			BG:     lipgloss.Color("#334155"),
			Border: lipgloss.Color("#64748B"),
		}
	}
}

func smartSuggestionBucket(suggestion smart.Suggestion) string {
	source := suggestion.Source
	switch {
	case strings.Contains(source, "Smart History"):
		return "History"
	case strings.Contains(source, "Context"), strings.Contains(source, "Quick"):
		return "Context"
	case strings.Contains(source, "Command DB"):
		return "Reference"
	case strings.Contains(source, "Fuzzy"), strings.Contains(source, "Common"):
		return "Explore"
	default:
		return "Other"
	}
}

func smartModeLabel(query string) string {
	if strings.TrimSpace(query) == "" {
		return "For current context"
	}
	return "Focused search"
}

func smartContextValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func smartDisplayContextToken(value, fallback string) string {
	value = smartContextValue(value, fallback)
	value = strings.TrimSuffix(value, ".exe")
	value = strings.TrimSuffix(value, ".cmd")
	value = strings.TrimSuffix(value, ".bat")
	if value == "" {
		return fallback
	}
	return value
}

func smartRelativeAgeShort(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	delta := time.Since(ts)
	switch {
	case delta < time.Hour:
		return "Seen <1h"
	case delta < 24*time.Hour:
		return fmt.Sprintf("Seen %dh", int(delta.Hours()+0.5))
	case delta < 30*24*time.Hour:
		return fmt.Sprintf("Seen %dd", int(delta.Hours()/24+0.5))
	default:
		return fmt.Sprintf("Seen %dmo", int(delta.Hours()/(24*30)+0.5))
	}
}

func renderSmartPrefixedBlock(prefix, text string, width int, style lipgloss.Style) string {
	textWidth := width - lipgloss.Width(prefix)
	if textWidth < 16 {
		textWidth = 16
	}
	lines := strings.Split(wrapText(text, textWidth), "\n")
	if len(lines) == 0 {
		return prefix
	}

	padding := strings.Repeat(" ", lipgloss.Width(prefix))
	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		if i == 0 {
			rendered = append(rendered, prefix+style.Render(line))
			continue
		}
		rendered = append(rendered, padding+style.Render(line))
	}
	return strings.Join(rendered, "\n")
}

func renderSmartBadgeRows(items []string, width int) string {
	if len(items) == 0 {
		return ""
	}

	lines := []string{}
	current := []string{}
	currentWidth := 0

	for _, item := range items {
		itemWidth := lipgloss.Width(item)
		if len(current) > 0 && currentWidth+1+itemWidth > width {
			lines = append(lines, strings.Join(current, " "))
			current = current[:0]
			currentWidth = 0
		}
		if len(current) > 0 {
			currentWidth++
		}
		current = append(current, item)
		currentWidth += itemWidth
	}

	if len(current) > 0 {
		lines = append(lines, strings.Join(current, " "))
	}

	return strings.Join(lines, "\n")
}

func renderSmartBadge(badge smartBadge) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(badge.FG).
		Background(badge.BG).
		Padding(0, 1).
		MarginRight(0).
		Render(badge.Label)
}

func wrapText(text string, width int) string {
	text = strings.TrimSpace(text)
	if text == "" || width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	lines := make([]string, 0, len(words)/3+1)
	current := words[0]

	for _, word := range words[1:] {
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}

		if lipgloss.Width(word) > width {
			if current != "" {
				lines = append(lines, current)
			}
			lines = append(lines, smartHardWrap(word, width)...)
			current = ""
			continue
		}

		if current != "" {
			lines = append(lines, current)
		}
		current = word
	}

	if current != "" {
		lines = append(lines, current)
	}

	return strings.Join(lines, "\n")
}

func smartHardWrap(text string, width int) []string {
	if width <= 0 || text == "" {
		return []string{text}
	}

	runes := []rune(text)
	if len(runes) <= width {
		return []string{text}
	}

	lines := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func smartPlural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
