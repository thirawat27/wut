package cmd

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/db"
	"wut/internal/logger"
	"wut/internal/metrics"
	"wut/internal/performance"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View command history log",
	Long:  `View, search, and analyze your complete sequential command execution log.`,
	Example: `  wut history
  wut history --limit 50
  wut history --search "docker"
  wut history --stats
  wut history --import-shell`,
	RunE: runHistory,
}

var (
	historyLimit       int
	historySearch      string
	historyStats       bool
	historyClear       bool
	historyExport      string
	historyImport      string
	historyImportShell bool
)

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.Flags().IntVarP(&historyLimit, "limit", "l", 20, "number of entries to show")
	historyCmd.Flags().StringVarP(&historySearch, "search", "s", "", "search term")
	historyCmd.Flags().BoolVar(&historyStats, "stats", false, "show statistics based on complete execution log")
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "clear complete history")
	historyCmd.Flags().StringVarP(&historyExport, "export", "e", "", "export history to JSON file")
	historyCmd.Flags().StringVarP(&historyImport, "import", "i", "", "import history from JSON file")
	historyCmd.Flags().BoolVar(&historyImportShell, "import-shell", false, "import from shell history files")
}

func runHistory(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	log := logger.With("history")

	cfg := config.Get()
	storage, err := db.NewStorage(cfg.Database.Path)
	if err != nil {
		log.Error("failed to initialize storage", "error", err)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer storage.Close()

	if historyClear {
		if err := storage.ClearHistory(ctx); err != nil {
			log.Error("failed to clear history", "error", err)
			return fmt.Errorf("failed to clear history: %w", err)
		}
		fmt.Println("✅ Complete command sequence history cleared successfully")
		return nil
	}

	if historyExport != "" {
		if err := storage.ExportHistory(ctx, historyExport); err != nil {
			log.Error("failed to export history", "error", err, "file", historyExport)
			return fmt.Errorf("failed to export history: %w", err)
		}
		fmt.Printf("✅ Sequential history exported to %s\n", historyExport)
		return nil
	}

	if historyImport != "" {
		if err := storage.ImportHistory(ctx, historyImport); err != nil {
			log.Error("failed to import history", "error", err, "file", historyImport)
			return fmt.Errorf("failed to import history: %w", err)
		}
		fmt.Printf("✅ Sequential history imported from %s\n", historyImport)
		return nil
	}

	if historyImportShell {
		return importShellHistory(ctx, storage)
	}

	if historyStats {
		return showHistoryStats(ctx, storage)
	}

	return showHistory(ctx, storage)
}

// deduplicateHistory filters out duplicate commands from history entries, keeping the most recent.
func deduplicateHistory(entries []db.CommandExecution) []db.CommandExecution {
	seen := make(map[string]bool)
	var result []db.CommandExecution
	for _, e := range entries {
		cmdTrimmed := strings.TrimSpace(e.Command)
		if !seen[cmdTrimmed] && cmdTrimmed != "" {
			seen[cmdTrimmed] = true
			result = append(result, e)
		}
	}
	return result
}

type historyModel struct {
	entries  []db.CommandExecution
	cursor   int
	page     int
	pageSize int
	numPages int
	total    int
	msg      string
	width    int
	height   int
}

func newHistoryModel(entries []db.CommandExecution, total int) historyModel {
	msg := ""

	numPages := int(math.Ceil(float64(len(entries)) / 10.0))
	if numPages == 0 {
		numPages = 1
	}

	return historyModel{
		entries:  entries,
		pageSize: 10,
		numPages: numPages,
		total:    total,
		msg:      msg,
	}
}

func (m historyModel) Init() tea.Cmd {
	return nil
}

type clearMsg struct{}

func tickClearMsg() tea.Cmd {
	return tea.Tick(time.Second*2, func(_ time.Time) tea.Msg {
		return clearMsg{}
	})
}

func (m historyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case clearMsg:
		m.msg = ""
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.page*m.pageSize {
					m.page--
				}
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor >= (m.page+1)*m.pageSize {
					m.page++
				}
			}
		case "left", "h", "pgup":
			if m.page > 0 {
				m.page--
				m.cursor = m.page * m.pageSize
			}
		case "right", "l", "pgdown":
			if m.page < m.numPages-1 {
				m.page++
				m.cursor = m.page * m.pageSize
			}
		case "enter", "c", "y": // c for copy, y for yank, enter for copy
			if m.cursor >= 0 && m.cursor < len(m.entries) {
				targetCmd := m.entries[m.cursor].Command
				if err := clipboard.WriteAll(targetCmd); err == nil {
					m.msg = "📋 Copied to clipboard"
					return m, tickClearMsg()
				} else {
					m.msg = string("❌ Copy failed: " + err.Error())
					return m, tickClearMsg()
				}
			}
		}
	}
	return m, nil
}

func (m historyModel) View() string {
	if len(m.entries) == 0 {
		return "No execution logs found.\n"
	}

	start := m.page * m.pageSize
	end := start + m.pageSize
	if end > len(m.entries) {
		end = len(m.entries)
	}

	// ── Responsive widths ───────────────────────────────────────────────────
	w := m.width
	if w <= 0 {
		w = 80 // ค่าเริ่มต้นก่อนได้ WindowSizeMsg
	}

	// box padding ปรับตามความกว้างจอ
	boxPadX := 2
	if w < 60 {
		boxPadX = 1
	}

	// boxWidth = เต็มจอ ลบ 2 สำหรับขอบ border ทั้งสองข้าง
	boxWidth := w - 2
	if boxWidth < 30 {
		boxWidth = 30
	}

	// innerWidth = พื้นที่ใช้งานจริงภายในกล่อง
	innerWidth := boxWidth - 2 - (boxPadX * 2)
	if innerWidth < 20 {
		innerWidth = 20
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	titleStr := headerStyle.Render("📜 Execution Log (Newest First)")

	var sb strings.Builder
	if m.msg != "" {
		alertIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true).Render("✔️  ")
		alertText := lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB")).Bold(true).Render(m.msg)

		alertStr := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#10B981")).
			Padding(0, 2).
			Render(alertIcon + alertText)

		titleWidth := lipgloss.Width(titleStr)
		alertWidth := lipgloss.Width(alertStr)

		padding := innerWidth - titleWidth - alertWidth
		if padding < 1 {
			padding = 1
		}

		titleBox := lipgloss.NewStyle().Height(lipgloss.Height(alertStr)).AlignVertical(lipgloss.Center).Render(titleStr)
		spaceBox := lipgloss.NewStyle().Width(padding).Render("")

		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, titleBox, spaceBox, alertStr) + "\n\n")
	} else {
		sb.WriteString(titleStr + "\n\n")
	}

	indexStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Width(4).Align(lipgloss.Right)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	// ซ่อน timestamp บนจอแคบ (< 50 col)
	showTime := w >= 50

	// availWidth: พื้นที่สำหรับ command text
	// index(4) + space(1) + time+brackets(13) + spaces(3) + cursor(2) = 23 เมื่อมี time
	// index(4) + space(1) + cursor(2) = 7 เมื่อไม่มี time
	var availWidth int
	if showTime {
		availWidth = innerWidth - 23
	} else {
		availWidth = innerWidth - 7
	}
	if availWidth < 10 {
		availWidth = 10
	}

	for i := start; i < end; i++ {
		entry := m.entries[i]
		cursor := "  "
		cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))

		if m.cursor == i {
			cursor = "👉"
			cmdStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#3B82F6")).Padding(0, 1)
		}

		dispCmd := entry.Command
		if lipgloss.Width(dispCmd) > availWidth {
			dispCmd = truncate.StringWithTail(dispCmd, uint(availWidth), "...")
		}

		if showTime {
			timeStr := entry.Timestamp.Local().Format("01-02 15:04")
			sb.WriteString(fmt.Sprintf("%s %s %s   %s\n\n", cursor, indexStyle.Render(fmt.Sprintf("%d.", i+1)), metaStyle.Render("["+timeStr+"]"), cmdStyle.Render(dispCmd)))
		} else {
			sb.WriteString(fmt.Sprintf("%s %s %s\n\n", cursor, indexStyle.Render(fmt.Sprintf("%d.", i+1)), cmdStyle.Render(dispCmd)))
		}
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(
		fmt.Sprintf("Showing %d unique executions out of %d total recorded.", len(m.entries), m.total)))
	sb.WriteString("\n\n")

	// ── Footer text (responsive) ──────────────────────────────────────────────
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308")).Bold(true)
	sb.WriteString(footerStyle.Render(fmt.Sprintf("Page %d/%d", m.page+1, m.numPages)))

	var footerNav string
	if w >= 90 {
		footerNav = " | [↑/↓] Navigate | [←/→] Prev/Next Page | [c/enter] Copy | [q] Quit"
	} else if w >= 60 {
		footerNav = " | ↑/↓ nav | ←/→ page | c copy | q quit"
	} else {
		footerNav = " | ↑/↓ | ←/→ | c | q"
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(footerNav + "\n"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, boxPadX).
		Width(boxWidth)

	return boxStyle.Render(strings.TrimRight(sb.String(), "\n"))
}

func showHistory(ctx context.Context, storage *db.Storage) error {
	var entries []db.CommandExecution
	var err error

	if historySearch != "" {
		entries, err = searchHistoryOptimized(storage, historySearch, historyLimit)
	} else {
		// Pull more for pagination to be useful after deduplication
		limit := historyLimit
		if limit <= 20 {
			limit = 1000 // default pull plenty to find unique entries
		}
		entries, err = storage.GetHistory(ctx, limit)
	}

	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}

	// Filter out duplicate commands to make UI much cleaner
	entries = deduplicateHistory(entries)

	if len(entries) == 0 {
		fmt.Println("No execution logs found.")
		return nil
	}

	total := getTotalCount(ctx, storage)
	p := tea.NewProgram(newHistoryModel(entries, total))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running history UI: %w", err)
	}

	metrics.RecordHistoryView()
	return nil
}

func searchHistoryOptimized(storage *db.Storage, query string, limit int) ([]db.CommandExecution, error) {
	matcher := performance.NewFastMatcher(false, 0.3, 3)

	entries, err := storage.GetHistory(context.Background(), 10000)
	if err != nil {
		return nil, err
	}

	type scoredEntry struct {
		entry db.CommandExecution
		score float64
	}

	var scored []scoredEntry
	for _, entry := range entries {
		result := matcher.Match(query, entry.Command)
		if result.Matched {
			scored = append(scored, scoredEntry{entry: entry, score: result.Score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}

	results := make([]db.CommandExecution, len(scored))
	for i, s := range scored {
		results[i] = s.entry
	}

	return results, nil
}

func getTotalCount(ctx context.Context, storage *db.Storage) int {
	entries, err := storage.GetHistory(ctx, 0)
	if err != nil {
		return 0
	}
	return len(entries)
}

func showHistoryStats(ctx context.Context, storage *db.Storage) error {
	log := logger.With("history.stats")
	log.Debug("getting sequential history statistics")

	stats, err := storage.GetHistoryStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get history statistics: %w", err)
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fmt.Printf("\n%s\n\n", headerStyle.Render("📊 Execution Log Insights"))

	statStyle := lipgloss.NewStyle().Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	fmt.Printf("  %s %s\n", statStyle.Render("Total Executions :"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalExecutions)))
	fmt.Printf("  %s %s\n", statStyle.Render("Unique Commands  :"), valueStyle.Render(fmt.Sprintf("%d", stats.UniqueCommands)))
	if stats.MostUsedCommand != "" {
		fmt.Printf("  %s %s\n", statStyle.Render("Favorite Command :"), valueStyle.Render(fmt.Sprintf("%s (%d times)", stats.MostUsedCommand, stats.MostUsedCount)))
	}
	fmt.Println()

	if len(stats.TimeDistribution) > 0 {
		catStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3B82F6"))
		fmt.Printf("%s\n", catStyle.Render("🕒 Time Distribution:"))
		for k, v := range stats.TimeDistribution {
			fmt.Printf("  • %-20s: %d\n", k, v)
		}
		fmt.Println()
	}

	if len(stats.TopCommands) > 0 {
		topStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
		fmt.Printf("%s\n", topStyle.Render("🏆 Most Used Combinations/Commands:"))
		for i, cmd := range stats.TopCommands {
			fmt.Printf("  %d. %s (%d times)\n", i+1, cmd.Command, cmd.Count)
		}
		fmt.Println()
	}

	metrics.RecordHistoryView()
	return nil
}

// Below is identical to old shell history import
func importShellHistory(ctx context.Context, storage *db.Storage) error {
	shellHistories := detectShellHistories()
	if len(shellHistories) == 0 {
		return fmt.Errorf("no shell history files detected")
	}

	fmt.Println("🔍 Detected shells:")
	for shellType, path := range shellHistories {
		fmt.Printf("  • %s: %s\n", shellType, path)
	}
	fmt.Println()

	fmt.Println("📖 Importing shell histories sequentially...")
	start := time.Now()

	var allCommands []string
	for shellType, path := range shellHistories {
		commands, err := readShellHistory(shellType, path)
		if err != nil {
			fmt.Printf("Warning: Failed to read %s history: %v\n", shellType, err)
			continue
		}
		allCommands = append(allCommands, commands...)
		fmt.Printf("  ✓ %s: %d linear commands\n", shellType, len(commands))
	}

	if len(allCommands) == 0 {
		fmt.Println("No history entries found in shell files")
		return nil
	}

	importStart := time.Now()
	imported := 0

	for _, cmd := range allCommands {
		if cmd = strings.TrimSpace(cmd); cmd != "" {
			if err := storage.AddHistory(ctx, cmd); err == nil {
				imported++
			}
		}
	}

	fmt.Printf("\n✅ Successfully imported %d execution steps in %v (total time: %v)\n", imported, time.Since(importStart), time.Since(start))
	return nil
}

func detectShellHistories() map[string]string {
	shells := make(map[string]string)
	home, err := os.UserHomeDir()
	if err != nil {
		return shells
	}

	bashHistory := filepath.Join(home, ".bash_history")
	if _, err := os.Stat(bashHistory); err == nil {
		shells["bash"] = bashHistory
	}

	zshHistory := filepath.Join(home, ".zsh_history")
	if _, err := os.Stat(zshHistory); err == nil {
		shells["zsh"] = zshHistory
	}

	fishHistory := filepath.Join(home, ".local", "share", "fish", "fish_history")
	if runtime.GOOS == "darwin" {
		fishHistory = filepath.Join(home, ".config", "fish", "fish_history")
	}
	if _, err := os.Stat(fishHistory); err == nil {
		shells["fish"] = fishHistory
	}

	psHistory := filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "PowerShell", "PSReadLine", "ConsoleHost_history.txt")
	if runtime.GOOS != "windows" {
		psHistory = filepath.Join(home, ".config", "powershell", "PSReadLine", "ConsoleHost_history.txt")
		if _, err := os.Stat(psHistory); err != nil {
			psHistory = filepath.Join(home, ".local", "share", "powershell", "PSReadLine", "ConsoleHost_history.txt")
		}
	}
	if _, err := os.Stat(psHistory); err == nil {
		shells["powershell"] = psHistory
	}

	return shells
}

func readShellHistory(shellType, path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)

	// Increase max line length for large history entries
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	switch shellType {
	case "fish":
		for scanner.Scan() {
			line := scanner.Text()
			if after, ok := strings.CutPrefix(line, "- cmd: "); ok {
				commands = append(commands, after)
			}
		}
	case "zsh":
		for scanner.Scan() {
			line := scanner.Text()
			if _, after, ok := strings.Cut(line, ";"); ok {
				commands = append(commands, after)
			} else if line != "" {
				commands = append(commands, line)
			}
		}
	default:
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				commands = append(commands, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return commands, nil
}
