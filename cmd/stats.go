package cmd

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/logger"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"stat", "metrics", "analytics"},
	Short:   "View WUT usage statistics and productivity metrics",
	Long: `Display detailed productivity analytics including command usage,
time-of-day heatmaps, top command leaderboard, and a productivity score.`,
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

// statsColors — palette used throughout the stats dashboard
var (
	sColPurple = lipgloss.Color("#7C3AED")
	sColViolet = lipgloss.Color("#8B5CF6")
	sColBlue   = lipgloss.Color("#3B82F6")
	sColCyan   = lipgloss.Color("#06B6D4")
	sColGreen  = lipgloss.Color("#10B981")
	sColAmber  = lipgloss.Color("#F59E0B")
	sColPink   = lipgloss.Color("#EC4899")
	sColYellow = lipgloss.Color("#FCD34D")
	sColGray   = lipgloss.Color("#6B7280")
	sColLtGray = lipgloss.Color("#D1D5DB")
	sColBg     = lipgloss.Color("#1E1B4B") // deep indigo bg hint (border only)
)

func runStats(cmd *cobra.Command, args []string) error {
	logger.Info("generating usage stats")

	cfg := config.Get()
	dbPath := cfg.Database.Path
	if dbPath == "" {
		home, _ := os.UserHomeDir()
		dbPath = home + "/.config/wut/wut.db"
	}

	store, err := db.NewStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer store.Close()

	stats, err := store.GetHistoryStats(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	if stats.TotalExecutions == 0 {
		emptyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(sColGray).
			Padding(1, 3).
			Render(
				lipgloss.JoinVertical(lipgloss.Center,
					lipgloss.NewStyle().Foreground(sColGray).Bold(true).Render("📭  No history yet"),
					"",
					lipgloss.NewStyle().Foreground(sColGray).Render("Start using WUT commands to build your productivity stats."),
				),
			)
		fmt.Println()
		fmt.Println(emptyBox)
		return nil
	}

	// ─── Styles ───────────────────────────────────────────────────────────────
	panelBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(sColViolet).
		Padding(0, 1)

	sectionTitle := func(icon, text string) string {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(sColViolet).
			Render(icon + " " + text)
	}

	muted := func(s string) string {
		return lipgloss.NewStyle().Foreground(sColGray).Render(s)
	}

	// ─── Header Banner ────────────────────────────────────────────────────────
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(sColPurple).
		Padding(0, 3).
		Render("  📊  WUT Productivity Dashboard  ")

	fmt.Println()
	fmt.Println(banner)
	fmt.Println()

	// ─── Summary Cards ────────────────────────────────────────────────────────
	cardStyle := func(bg lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(bg).
			Padding(0, 2).
			Width(22)
	}

	card1 := cardStyle(sColBlue).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(sColBlue).Render("Total Commands"),
			lipgloss.NewStyle().Bold(true).Foreground(sColYellow).Render(fmt.Sprintf("%d", stats.TotalExecutions)),
		),
	)
	card2 := cardStyle(sColCyan).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(sColCyan).Render("Unique Commands"),
			lipgloss.NewStyle().Bold(true).Foreground(sColYellow).Render(fmt.Sprintf("%d", stats.UniqueCommands)),
		),
	)

	// Productivity Score: ratio of unique/total, scaled 0-100
	score := 0
	if stats.TotalExecutions > 0 {
		ratio := float64(stats.UniqueCommands) / float64(stats.TotalExecutions)
		score = int(math.Min(100, ratio*200)) // 50% unique = 100 score
	}
	scoreColor := sColGreen
	scoreLabel := "Excellent"
	if score < 40 {
		scoreColor = sColAmber
		scoreLabel = "Repetitive"
	} else if score < 70 {
		scoreColor = sColCyan
		scoreLabel = "Good"
	}
	card3 := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(scoreColor).
		Padding(0, 2).
		Width(22).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(scoreColor).Render("Variety Score"),
				lipgloss.NewStyle().Bold(true).Foreground(sColYellow).Render(fmt.Sprintf("%d%%", score))+" "+
					lipgloss.NewStyle().Foreground(scoreColor).Render(scoreLabel),
			),
		)

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, card1, "  ", card2, "  ", card3))
	fmt.Println()

	// ─── Top Commands Leaderboard ─────────────────────────────────────────────
	displayCount := 7
	if displayCount > len(stats.TopCommands) {
		displayCount = len(stats.TopCommands)
	}
	maxCount := 0
	for i := 0; i < displayCount; i++ {
		if stats.TopCommands[i].Count > maxCount {
			maxCount = stats.TopCommands[i].Count
		}
	}

	medals := []string{"🥇", "🥈", "🥉", " 4", " 5", " 6", " 7"}
	barColors := []lipgloss.Color{sColPink, sColViolet, sColBlue, sColCyan, sColGreen, sColAmber, sColGray}

	var lbLines []string
	lbLines = append(lbLines, sectionTitle("🏆", "Top Command Leaderboard"))
	lbLines = append(lbLines, "")

	for i := 0; i < displayCount; i++ {
		c := stats.TopCommands[i]
		barWidth := 0
		if maxCount > 0 {
			barWidth = int(math.Round(float64(c.Count) / float64(maxCount) * 36.0))
		}
		if barWidth == 0 {
			barWidth = 1
		}
		if barWidth > 36 {
			barWidth = 36
		}

		barStr := strings.Repeat("█", barWidth) + strings.Repeat(" ", 36-barWidth)
		barCol := lipgloss.NewStyle().Foreground(barColors[i%len(barColors)]).Render(barStr)
		pct := float64(c.Count) / float64(stats.TotalExecutions) * 100

		medal := medals[i]
		if i >= 3 {
			medal = lipgloss.NewStyle().Foreground(sColGray).Render(medal)
		}

		cmdLabel := c.Command
		if len(cmdLabel) > 22 {
			cmdLabel = cmdLabel[:21] + "…"
		}

		cmdCol := lipgloss.NewStyle().Foreground(sColLtGray).Render(fmt.Sprintf("%-22s", cmdLabel))
		valCol := lipgloss.NewStyle().Bold(true).Foreground(sColYellow).Render(fmt.Sprintf("%5d", c.Count))
		pctCol := lipgloss.NewStyle().Foreground(sColGray).Render(fmt.Sprintf("(%5.1f%%)", pct))

		line := fmt.Sprintf("  %s  %s %s  %s  %s", medal, cmdCol, barCol, valCol, pctCol)
		lbLines = append(lbLines, line)
	}

	lbBox := panelBorder.Width(86).Render(strings.Join(lbLines, "\n"))
	fmt.Println(lbBox)
	fmt.Println()

	// ─── Time-of-Day Heatmap ──────────────────────────────────────────────────
	timeKeys := []string{
		"Morning (06:00-12:00)",
		"Afternoon (12:00-18:00)",
		"Evening (18:00-24:00)",
		"Night (00:00-06:00)",
	}
	timeIcons := []string{"🌅", "☀️ ", "🌆", "🌙"}
	timeColors := []lipgloss.Color{sColAmber, sColCyan, sColViolet, sColBlue}

	timeMax := 0
	for _, k := range timeKeys {
		if v := stats.TimeDistribution[k]; v > timeMax {
			timeMax = v
		}
	}

	var hmLines []string
	hmLines = append(hmLines, sectionTitle("🕒", "Usage by Time of Day"))
	hmLines = append(hmLines, "")

	for i, k := range timeKeys {
		v := stats.TimeDistribution[k]
		w := 0
		if timeMax > 0 {
			w = int(math.Round(float64(v) / float64(timeMax) * 32.0))
		}
		if w == 0 && v > 0 {
			w = 1
		}
		if w > 32 {
			w = 32
		}
		filled := strings.Repeat("▇", w)
		empty := strings.Repeat("·", 32-w)
		barCol := lipgloss.NewStyle().Foreground(timeColors[i]).Render(filled) +
			lipgloss.NewStyle().Foreground(sColGray).Render(empty)

		pct := 0.0
		if stats.TotalExecutions > 0 {
			pct = float64(v) / float64(stats.TotalExecutions) * 100
		}

		// Fixed width padding for strings
		timeCol := lipgloss.NewStyle().Foreground(timeColors[i]).Render(fmt.Sprintf("%-24s", k))
		valCol := lipgloss.NewStyle().Bold(true).Foreground(sColYellow).Render(fmt.Sprintf("%5d", v))
		pctCol := lipgloss.NewStyle().Foreground(sColGray).Render(fmt.Sprintf("(%5.1f%%)", pct))

		icon := timeIcons[i]
		line := fmt.Sprintf("  %s  %s %s  %s  %s", icon, timeCol, barCol, valCol, pctCol)
		hmLines = append(hmLines, line)
	}

	hmBox := panelBorder.Width(86).Render(strings.Join(hmLines, "\n"))
	fmt.Println(hmBox)

	// ─── Footer ───────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(muted("  💡 Tip: Use ") +
		lipgloss.NewStyle().Foreground(sColCyan).Render("wut bookmark add \"cmd\" -l label") +
		muted(" to save your favourite commands."))
	fmt.Println()
	return nil
}
