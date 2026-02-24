// Package cmd provides CLI commands for WUT
package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"wut/internal/config"
	"wut/internal/logger"
	"wut/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `View, get, set, and reset configuration values.

Supports dot notation for nested keys (e.g., 'ui.theme', 'fuzzy.enabled').
Boolean values can be: true, false, 1, 0, yes, no, on, off`,
	Example: `  wut config                          # Show all config
  wut config --list                   # List all keys
  wut config --get ui.theme           # Get specific value
  wut config --set ui.theme dark      # Set value
  wut config --set fuzzy.enabled true # Enable fuzzy matching
  wut config --edit                   # Open in default editor
  wut config --reset                  # Reset to defaults
  wut config --import config.yaml     # Import from file
  wut config --export backup.yaml     # Export to file`,
	RunE: runConfig,
}

var (
	configList   bool
	configGet    string
	configSet    string
	configValue  string
	configReset  bool
	configEdit   bool
	configImport string
	configExport string
	configPath   bool
)

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().BoolVarP(&configList, "list", "l", false, "list all configuration keys")
	configCmd.Flags().StringVarP(&configGet, "get", "g", "", "get configuration value by key (supports dot notation)")
	configCmd.Flags().StringVarP(&configSet, "set", "s", "", "set configuration key (use with --value)")
	configCmd.Flags().StringVarP(&configValue, "value", "v", "", "value to set")
	configCmd.Flags().BoolVarP(&configReset, "reset", "r", false, "reset to default configuration")
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "open config file in default editor")
	configCmd.Flags().StringVar(&configImport, "import", "", "import configuration from file")
	configCmd.Flags().StringVar(&configExport, "export", "", "export configuration to file")
	configCmd.Flags().BoolVar(&configPath, "path", false, "show config file path")
}

func runConfig(cmd *cobra.Command, args []string) error {
	log := logger.With("config")

	// Handle path
	if configPath {
		fmt.Println(getConfigFile())
		return nil
	}

	// Handle edit
	if configEdit {
		return editConfig()
	}

	// Handle import
	if configImport != "" {
		if err := importConfig(configImport); err != nil {
			log.Error("failed to import config", "error", err)
			return fmt.Errorf("failed to import config: %w", err)
		}
		fmt.Printf("Configuration imported from %s\n", configImport)
		return nil
	}

	// Handle export
	if configExport != "" {
		if err := exportConfig(configExport); err != nil {
			log.Error("failed to export config", "error", err)
			return fmt.Errorf("failed to export config: %w", err)
		}
		fmt.Printf("Configuration exported to %s\n", configExport)
		return nil
	}

	// Handle reset
	if configReset {
		if err := resetConfig(); err != nil {
			log.Error("failed to reset config", "error", err)
			return fmt.Errorf("failed to reset config: %w", err)
		}
		fmt.Println("✅ Configuration reset to defaults")
		return nil
	}

	// Handle list
	if configList {
		return listConfigKeys()
	}

	// Handle get
	if configGet != "" {
		value, err := getConfigValue(configGet)
		if err != nil {
			log.Error("failed to get config value", "key", configGet, "error", err)
			return err
		}
		fmt.Printf("%s: %v\n", configGet, value)
		return nil
	}

	// Handle set
	if configSet != "" {
		if err := setConfigValue(configSet, configValue); err != nil {
			log.Error("failed to set config value", "key", configSet, "error", err)
			return err
		}
		fmt.Printf("✅ Set %s = %v\n", configSet, configValue)
		return nil
	}

	// Default: show configuration wizard (TUI), fall back to plain text on error
	if err := runConfigUI(); err != nil {
		return showConfig()
	}
	return nil
}

func runConfigUI() error {
	cfg := config.Get()

	// Convert numerical settings to strings for inputs
	fuzzyDistance := strconv.Itoa(cfg.Fuzzy.MaxDistance)
	fuzzyThreshold := strconv.FormatFloat(cfg.Fuzzy.Threshold, 'f', 2, 64)
	uiPagination := strconv.Itoa(cfg.UI.Pagination)
	dbSize := strconv.Itoa(cfg.Database.MaxSize)
	tldrSyncInterval := strconv.Itoa(cfg.TLDR.AutoSyncInterval)
	historyMaxEntries := strconv.Itoa(cfg.History.MaxEntries)
	logMaxSize := strconv.Itoa(cfg.Logging.MaxSize)
	logMaxAge := strconv.Itoa(cfg.Logging.MaxAge)
	confirmSave := false

	// Custom keymap: Add Space to Toggle on Confirm, matching other fields
	km := huh.NewDefaultKeyMap()
	km.Confirm.Toggle = key.NewBinding(
		key.WithKeys("h", "l", "right", "left", " "),
		key.WithHelp("←/→/space", "toggle"),
	)

	form := huh.NewForm(
		// ── 1. Appearance ─────────────────────────────────────────
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Theme").
				Description("Color scheme for the interface").
				Options(
					huh.NewOption("Auto (follow system)", "auto"),
					huh.NewOption("Light", "light"),
					huh.NewOption("Dark", "dark"),
				).
				Value(&cfg.UI.Theme),
			huh.NewConfirm().
				Title("Syntax Highlighting").
				Description("Colorize code snippets and commands").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.UI.SyntaxHighlighting),
			huh.NewInput().
				Title("Pagination").
				Description("Number of results per page").
				Value(&uiPagination),
		).Title("  Appearance"),

		// ── 2. Display ────────────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Show Confidence Score").
				Description("Display the AI confidence level alongside results").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.UI.ShowConfidence),
			huh.NewConfirm().
				Title("Show Explanations").
				Description("Include detailed breakdowns for each command").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.UI.ShowExplanations),
		).Title("  Display"),

		// ── 3. Fuzzy Matching ─────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Fuzzy Search").
				Description("Correct typos and find approximate matches").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Fuzzy.Enabled),
			huh.NewConfirm().
				Title("Case Sensitive").
				Description("Distinguish between upper and lower case").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Fuzzy.CaseSensitive),
			huh.NewInput().
				Title("Max Edit Distance").
				Description("Maximum Levenshtein distance (1–5 recommended)").
				Value(&fuzzyDistance),
			huh.NewInput().
				Title("Match Threshold").
				Description("Minimum similarity score, 0.0 to 1.0").
				Value(&fuzzyThreshold),
		).Title("  Fuzzy Matching"),

		// ── 4. TLDR Pages ─────────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable TLDR Pages").
				Description("Show community-maintained command cheatsheets").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.TLDR.Enabled),
			huh.NewConfirm().
				Title("Offline Mode").
				Description("Only use locally cached pages, never fetch online").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.TLDR.OfflineMode),
			huh.NewConfirm().
				Title("Auto Sync").
				Description("Periodically download new and updated pages").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.TLDR.AutoSync),
			huh.NewInput().
				Title("Sync Interval").
				Description("Days between automatic syncs").
				Value(&tldrSyncInterval),
		).Title("  TLDR Pages"),

		// ── 5. Context Analysis ───────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Context").
				Description("Analyze your working directory for smarter suggestions").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Context.Enabled),
			huh.NewConfirm().
				Title("Git Integration").
				Description("Use repository status and history as context").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Context.GitIntegration),
			huh.NewConfirm().
				Title("Project Detection").
				Description("Auto-detect project type (Node.js, Go, Python, …)").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Context.ProjectDetection),
			huh.NewConfirm().
				Title("Environment Variables").
				Description("Include relevant env vars in analysis").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Context.EnvironmentVars),
		).Title("  Context Analysis"),

		// ── 6. Database ───────────────────────────────────────────
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Engine").
				Description("Storage backend for local data").
				Options(
					huh.NewOption("BBolt (default)", "bbolt"),
					huh.NewOption("SQLite", "sqlite"),
				).
				Value(&cfg.Database.Type),
			huh.NewInput().
				Title("Max Size (MB)").
				Description("Maximum database file size").
				Value(&dbSize),
			huh.NewConfirm().
				Title("Automatic Backups").
				Description("Periodically back up the database").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Database.BackupEnabled),
		).Title("  Database"),

		// ── 7. History ────────────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Track History").
				Description("Remember previously looked-up commands").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.History.Enabled),
			huh.NewInput().
				Title("Max Entries").
				Description("Maximum number of history records to keep").
				Value(&historyMaxEntries),
			huh.NewConfirm().
				Title("Track Frequency").
				Description("Record how often each command is used").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.History.TrackFrequency),
		).Title("  History"),

		// ── 8. Privacy ────────────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Local Only").
				Description("Never send any data to external services").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Privacy.LocalOnly),
			huh.NewConfirm().
				Title("Encrypt Data").
				Description("Encrypt locally stored data at rest").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Privacy.EncryptData),
			huh.NewConfirm().
				Title("Anonymize Commands").
				Description("Strip sensitive arguments from history").
				Affirmative("  Yes  ").Negative("  No  ").
				WithButtonAlignment(lipgloss.Left).
				Value(&cfg.Privacy.AnonymizeCommands),
		).Title("  Privacy"),

		// ── 9. Logging ────────────────────────────────────────────
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Log Level").
				Description("Minimum severity of messages to record").
				Options(
					huh.NewOption("Debug", "debug"),
					huh.NewOption("Info", "info"),
					huh.NewOption("Warn", "warn"),
					huh.NewOption("Error", "error"),
				).
				Value(&cfg.Logging.Level),
			huh.NewInput().
				Title("Max Log Size (MB)").
				Description("Rotate log file after this size").
				Value(&logMaxSize),
			huh.NewInput().
				Title("Max Log Age (days)").
				Description("Delete old log files after this many days").
				Value(&logMaxAge),
		).Title("  Logging"),

		// ── 10. Confirm ───────────────────────────────────────────
		huh.NewGroup(
			huh.NewConfirm().
				Title("Save all changes?").
				Affirmative("   Save   ").
				Negative("   Discard   ").
				WithButtonAlignment(lipgloss.Left).
				Value(&confirmSave),
		).Title("  Confirm"),
	).
		WithTheme(getConfigTheme()).
		WithKeyMap(km).
		WithShowHelp(false) // ปิด Help ตัวเก่า เพื่อให้ขนาด UI ชัวร์และไม่บัคซ้อนกัน

	// Wrap in a custom Bubble Tea model for a polished full-screen layout
	p := tea.NewProgram(newConfigUI(form), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	if form.State == huh.StateAborted {
		fmt.Println("\n❌ Configuration cancelled")
		return nil
	}

	if !confirmSave {
		fmt.Println("\n❌ No changes saved")
		return nil
	}

	// Parse strings back to numerical values
	if v, err := strconv.Atoi(fuzzyDistance); err == nil {
		cfg.Fuzzy.MaxDistance = v
	}
	if v, err := strconv.ParseFloat(fuzzyThreshold, 64); err == nil {
		cfg.Fuzzy.Threshold = v
	}
	if v, err := strconv.Atoi(uiPagination); err == nil {
		cfg.UI.Pagination = v
	}
	if v, err := strconv.Atoi(dbSize); err == nil {
		cfg.Database.MaxSize = v
	}
	if v, err := strconv.Atoi(tldrSyncInterval); err == nil {
		cfg.TLDR.AutoSyncInterval = v
	}
	if v, err := strconv.Atoi(historyMaxEntries); err == nil {
		cfg.History.MaxEntries = v
	}
	if v, err := strconv.Atoi(logMaxSize); err == nil {
		cfg.Logging.MaxSize = v
	}
	if v, err := strconv.Atoi(logMaxAge); err == nil {
		cfg.Logging.MaxAge = v
	}

	// Save the config
	config.Set(cfg)
	if err := config.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ Configuration saved successfully!")
	return nil
}

func showConfig() error {
	cfg := config.Get()

	// Styles
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Bold(true)

	fmt.Println()
	fmt.Println(headerStyle.Render("⚙️  Configuration"))
	fmt.Println()

	// App config
	fmt.Println(headerStyle.Render("Application"))
	printConfigItem("  Name", cfg.App.Name, keyStyle, valueStyle)
	printConfigItem("  Debug", fmt.Sprintf("%v", cfg.App.Debug), keyStyle, valueStyle)
	fmt.Println()

	// Fuzzy config
	fmt.Println(headerStyle.Render("Fuzzy Matching"))
	printConfigItem("  Enabled", fmt.Sprintf("%v", cfg.Fuzzy.Enabled), keyStyle, valueStyle)
	printConfigItem("  Case Sensitive", fmt.Sprintf("%v", cfg.Fuzzy.CaseSensitive), keyStyle, valueStyle)
	printConfigItem("  Max Distance", fmt.Sprintf("%d", cfg.Fuzzy.MaxDistance), keyStyle, valueStyle)
	printConfigItem("  Threshold", fmt.Sprintf("%.2f", cfg.Fuzzy.Threshold), keyStyle, valueStyle)
	fmt.Println()

	// UI config
	fmt.Println(headerStyle.Render("User Interface"))
	printConfigItem("  Theme", cfg.UI.Theme, keyStyle, valueStyle)
	printConfigItem("  Show Confidence", fmt.Sprintf("%v", cfg.UI.ShowConfidence), keyStyle, valueStyle)
	printConfigItem("  Show Explanations", fmt.Sprintf("%v", cfg.UI.ShowExplanations), keyStyle, valueStyle)
	printConfigItem("  Syntax Highlighting", fmt.Sprintf("%v", cfg.UI.SyntaxHighlighting), keyStyle, valueStyle)
	printConfigItem("  Pagination", fmt.Sprintf("%d", cfg.UI.Pagination), keyStyle, valueStyle)
	fmt.Println()

	// Database config
	fmt.Println(headerStyle.Render("Database"))
	printConfigItem("  Type", cfg.Database.Type, keyStyle, valueStyle)
	printConfigItem("  Path", cfg.Database.Path, keyStyle, valueStyle)
	printConfigItem("  Max Size", fmt.Sprintf("%d MB", cfg.Database.MaxSize), keyStyle, valueStyle)
	printConfigItem("  Backup Enabled", fmt.Sprintf("%v", cfg.Database.BackupEnabled), keyStyle, valueStyle)
	fmt.Println()

	// History config
	fmt.Println(headerStyle.Render("History"))
	printConfigItem("  Enabled", fmt.Sprintf("%v", cfg.History.Enabled), keyStyle, valueStyle)
	printConfigItem("  Max Entries", fmt.Sprintf("%d", cfg.History.MaxEntries), keyStyle, valueStyle)
	printConfigItem("  Track Frequency", fmt.Sprintf("%v", cfg.History.TrackFrequency), keyStyle, valueStyle)
	printConfigItem("  Track Context", fmt.Sprintf("%v", cfg.History.TrackContext), keyStyle, valueStyle)
	printConfigItem("  Track Timing", fmt.Sprintf("%v", cfg.History.TrackTiming), keyStyle, valueStyle)
	fmt.Println()

	// Context config
	fmt.Println(headerStyle.Render("Context Analysis"))
	printConfigItem("  Enabled", fmt.Sprintf("%v", cfg.Context.Enabled), keyStyle, valueStyle)
	printConfigItem("  Git Integration", fmt.Sprintf("%v", cfg.Context.GitIntegration), keyStyle, valueStyle)
	printConfigItem("  Project Detection", fmt.Sprintf("%v", cfg.Context.ProjectDetection), keyStyle, valueStyle)
	printConfigItem("  Environment Vars", fmt.Sprintf("%v", cfg.Context.EnvironmentVars), keyStyle, valueStyle)
	printConfigItem("  Directory Analysis", fmt.Sprintf("%v", cfg.Context.DirectoryAnalysis), keyStyle, valueStyle)
	fmt.Println()

	// Privacy config
	fmt.Println(headerStyle.Render("Privacy"))
	printConfigItem("  Local Only", fmt.Sprintf("%v", cfg.Privacy.LocalOnly), keyStyle, valueStyle)
	printConfigItem("  Encrypt Data", fmt.Sprintf("%v", cfg.Privacy.EncryptData), keyStyle, valueStyle)
	printConfigItem("  Anonymize Commands", fmt.Sprintf("%v", cfg.Privacy.AnonymizeCommands), keyStyle, valueStyle)
	printConfigItem("  Share Analytics", fmt.Sprintf("%v", cfg.Privacy.ShareAnalytics), keyStyle, valueStyle)
	fmt.Println()

	// Logging config
	fmt.Println(headerStyle.Render("Logging"))
	printConfigItem("  Level", cfg.Logging.Level, keyStyle, valueStyle)
	printConfigItem("  File", cfg.Logging.File, keyStyle, valueStyle)
	printConfigItem("  Max Size", fmt.Sprintf("%d MB", cfg.Logging.MaxSize), keyStyle, valueStyle)
	printConfigItem("  Max Backups", fmt.Sprintf("%d", cfg.Logging.MaxBackups), keyStyle, valueStyle)
	printConfigItem("  Max Age", fmt.Sprintf("%d days", cfg.Logging.MaxAge), keyStyle, valueStyle)
	fmt.Println()

	// TLDR config
	fmt.Println(headerStyle.Render("TLDR Pages"))
	printConfigItem("  Enabled", fmt.Sprintf("%v", cfg.TLDR.Enabled), keyStyle, valueStyle)
	printConfigItem("  Auto Sync", fmt.Sprintf("%v", cfg.TLDR.AutoSync), keyStyle, valueStyle)
	printConfigItem("  Auto Sync Interval", fmt.Sprintf("%d days", cfg.TLDR.AutoSyncInterval), keyStyle, valueStyle)
	printConfigItem("  Offline Mode", fmt.Sprintf("%v", cfg.TLDR.OfflineMode), keyStyle, valueStyle)
	printConfigItem("  Default Platform", cfg.TLDR.DefaultPlatform, keyStyle, valueStyle)
	fmt.Println()

	// Show config file path
	fmt.Println(ui.HiBlackf("Configuration file: %s", getConfigFile()))
	fmt.Println()
	fmt.Println("Use 'wut config --edit' to edit in your default editor")
	fmt.Println("Use 'wut config --set <key> --value <value>' to change settings")

	return nil
}

func printConfigItem(key, value string, keyStyle, valueStyle lipgloss.Style) {
	fmt.Printf("%s %s\n", keyStyle.Render(key+":"), valueStyle.Render(value))
}

// configPathMap maps dot-notation keys to their path in the config struct
type configField struct {
	path     []int
	typeName string
	setter   func(reflect.Value, string) error
}

var configFieldMap = map[string]configField{
	// App
	"app.name":    {[]int{0, 0}, "string", setString},
	"app.version": {[]int{0, 1}, "string", setString},
	"app.debug":   {[]int{0, 2}, "bool", setBool},
	// Fuzzy
	"fuzzy.enabled":        {[]int{1, 0}, "bool", setBool},
	"fuzzy.case_sensitive": {[]int{1, 1}, "bool", setBool},
	"fuzzy.caseSensitive":  {[]int{1, 1}, "bool", setBool},
	"fuzzy.max_distance":   {[]int{1, 2}, "int", setInt},
	"fuzzy.maxDistance":    {[]int{1, 2}, "int", setInt},
	"fuzzy.threshold":      {[]int{1, 3}, "float64", setFloat64},
	// UI
	"ui.theme":               {[]int{2, 0}, "string", setString},
	"ui.show_confidence":     {[]int{2, 1}, "bool", setBool},
	"ui.showConfidence":      {[]int{2, 1}, "bool", setBool},
	"ui.show_explanations":   {[]int{2, 2}, "bool", setBool},
	"ui.showExplanations":    {[]int{2, 2}, "bool", setBool},
	"ui.syntax_highlighting": {[]int{2, 3}, "bool", setBool},
	"ui.syntaxHighlighting":  {[]int{2, 3}, "bool", setBool},
	"ui.pagination":          {[]int{2, 4}, "int", setInt},
	// Database
	"database.type":            {[]int{3, 0}, "string", setString},
	"database.path":            {[]int{3, 1}, "string", setString},
	"database.max_size":        {[]int{3, 2}, "int", setInt},
	"database.maxSize":         {[]int{3, 2}, "int", setInt},
	"database.backup_enabled":  {[]int{3, 3}, "bool", setBool},
	"database.backupEnabled":   {[]int{3, 3}, "bool", setBool},
	"database.backup_interval": {[]int{3, 4}, "int", setInt},
	"database.backupInterval":  {[]int{3, 4}, "int", setInt},
	// History
	"history.enabled":         {[]int{4, 0}, "bool", setBool},
	"history.max_entries":     {[]int{4, 1}, "int", setInt},
	"history.maxEntries":      {[]int{4, 1}, "int", setInt},
	"history.track_frequency": {[]int{4, 2}, "bool", setBool},
	"history.trackFrequency":  {[]int{4, 2}, "bool", setBool},
	"history.track_context":   {[]int{4, 3}, "bool", setBool},
	"history.trackContext":    {[]int{4, 3}, "bool", setBool},
	"history.track_timing":    {[]int{4, 4}, "bool", setBool},
	"history.trackTiming":     {[]int{4, 4}, "bool", setBool},
	// Context
	"context.enabled":            {[]int{5, 0}, "bool", setBool},
	"context.git_integration":    {[]int{5, 1}, "bool", setBool},
	"context.gitIntegration":     {[]int{5, 1}, "bool", setBool},
	"context.project_detection":  {[]int{5, 2}, "bool", setBool},
	"context.projectDetection":   {[]int{5, 2}, "bool", setBool},
	"context.environment_vars":   {[]int{5, 3}, "bool", setBool},
	"context.environmentVars":    {[]int{5, 3}, "bool", setBool},
	"context.directory_analysis": {[]int{5, 4}, "bool", setBool},
	"context.directoryAnalysis":  {[]int{5, 4}, "bool", setBool},
	// Shell
	"shell.enabled": {[]int{6, 0}, "bool", setBool},
	// Privacy
	"privacy.local_only":         {[]int{7, 0}, "bool", setBool},
	"privacy.localOnly":          {[]int{7, 0}, "bool", setBool},
	"privacy.encrypt_data":       {[]int{7, 1}, "bool", setBool},
	"privacy.encryptData":        {[]int{7, 1}, "bool", setBool},
	"privacy.anonymize_commands": {[]int{7, 2}, "bool", setBool},
	"privacy.anonymizeCommands":  {[]int{7, 2}, "bool", setBool},
	"privacy.share_analytics":    {[]int{7, 3}, "bool", setBool},
	"privacy.shareAnalytics":     {[]int{7, 3}, "bool", setBool},
	// Logging
	"logging.level":       {[]int{8, 0}, "string", setString},
	"logging.file":        {[]int{8, 1}, "string", setString},
	"logging.max_size":    {[]int{8, 2}, "int", setInt},
	"logging.maxSize":     {[]int{8, 2}, "int", setInt},
	"logging.max_backups": {[]int{8, 3}, "int", setInt},
	"logging.maxBackups":  {[]int{8, 3}, "int", setInt},
	"logging.max_age":     {[]int{8, 4}, "int", setInt},
	"logging.maxAge":      {[]int{8, 4}, "int", setInt},
	// TLDR
	"tldr.enabled":            {[]int{9, 0}, "bool", setBool},
	"tldr.auto_sync":          {[]int{9, 1}, "bool", setBool},
	"tldr.autoSync":           {[]int{9, 1}, "bool", setBool},
	"tldr.auto_sync_interval": {[]int{9, 2}, "int", setInt},
	"tldr.autoSyncInterval":   {[]int{9, 2}, "int", setInt},
	"tldr.offline_mode":       {[]int{9, 3}, "bool", setBool},
	"tldr.offlineMode":        {[]int{9, 3}, "bool", setBool},
	"tldr.auto_detect_online": {[]int{9, 4}, "bool", setBool},
	"tldr.autoDetectOnline":   {[]int{9, 4}, "bool", setBool},
	"tldr.max_cache_age":      {[]int{9, 5}, "int", setInt},
	"tldr.maxCacheAge":        {[]int{9, 5}, "int", setInt},
	"tldr.default_platform":   {[]int{9, 6}, "string", setString},
	"tldr.defaultPlatform":    {[]int{9, 6}, "string", setString},
}

// Setter functions
func setString(v reflect.Value, s string) error {
	if v.Kind() != reflect.String {
		return fmt.Errorf("expected string, got %s", v.Kind())
	}
	v.SetString(s)
	return nil
}

func setBool(v reflect.Value, s string) error {
	if v.Kind() != reflect.Bool {
		return fmt.Errorf("expected bool, got %s", v.Kind())
	}
	s = strings.ToLower(strings.TrimSpace(s))
	val := s == "true" || s == "1" || s == "yes" || s == "on" || s == "enabled"
	v.SetBool(val)
	return nil
}

func setInt(v reflect.Value, s string) error {
	if v.Kind() != reflect.Int && v.Kind() != reflect.Int64 {
		return fmt.Errorf("expected int, got %s", v.Kind())
	}
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer: %s", s)
	}
	v.SetInt(val)
	return nil
}

func setFloat64(v reflect.Value, s string) error {
	if v.Kind() != reflect.Float64 && v.Kind() != reflect.Float32 {
		return fmt.Errorf("expected float, got %s", v.Kind())
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid float: %s", s)
	}
	v.SetFloat(val)
	return nil
}

func getConfigValue(key string) (any, error) {
	// Normalize key (lowercase, replace spaces with dots)
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, " ", ".")

	field, ok := configFieldMap[key]
	if !ok {
		// Try to find with alternative key formats
		return nil, fmt.Errorf("unknown config key: %s\nUse 'wut config --list' to see available keys", key)
	}

	cfg := config.Get()
	v := reflect.ValueOf(cfg).Elem()

	// Navigate to the field
	for _, idx := range field.path {
		v = v.Field(idx)
	}

	return v.Interface(), nil
}

func setConfigValue(key, value string) error {
	if value == "" {
		return fmt.Errorf("--value is required when using --set")
	}

	// Normalize key
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, " ", ".")

	field, ok := configFieldMap[key]
	if !ok {
		return fmt.Errorf("unknown config key: %s\nUse 'wut config --list' to see available keys", key)
	}

	cfg := config.Get()
	v := reflect.ValueOf(cfg).Elem()

	// Navigate to the field
	for _, idx := range field.path {
		v = v.Field(idx)
	}

	// Set the value using the appropriate setter
	if err := field.setter(v, value); err != nil {
		return fmt.Errorf("failed to set %s: %w", key, err)
	}

	// Save the config
	return config.Save()
}

func listConfigKeys() error {
	fmt.Println()
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fmt.Println(headerStyle.Render("Available Configuration Keys"))
	fmt.Println()

	groups := map[string][]string{
		"app":      {},
		"fuzzy":    {},
		"ui":       {},
		"database": {},
		"history":  {},
		"context":  {},
		"privacy":  {},
		"logging":  {},
		"tldr":     {},
	}

	for key := range configFieldMap {
		parts := strings.Split(key, ".")
		if len(parts) == 2 {
			group := parts[0]
			if _, ok := groups[group]; ok {
				// Only add snake_case keys
				if !strings.Contains(parts[1], "C") && !strings.Contains(parts[1], "D") {
					groups[group] = append(groups[group], key)
				}
			}
		}
	}

	for group, keys := range groups {
		if len(keys) == 0 {
			continue
		}
		fmt.Printf("  %s:\n", headerStyle.Render(group))
		for _, key := range keys {
			fmt.Printf("    - %s\n", key)
		}
		fmt.Println()
	}

	fmt.Println("Examples:")
	fmt.Println("  wut config --get ui.theme")
	fmt.Println("  wut config --set fuzzy.enabled --value true")
	fmt.Println("  wut config --set logging.level --value debug")

	return nil
}

func resetConfig() error {
	return config.Reset()
}

func editConfig() error {
	return config.Edit()
}

func importConfig(path string) error {
	return config.Import(path)
}

func exportConfig(path string) error {
	return config.Export(path)
}

func getConfigFile() string {
	return config.GetConfigPath()
}

// ─── Bubble Tea wrapper for polished full-screen config UI ──────────────────

type configUI struct {
	form   *huh.Form
	width  int
	height int
}

func newConfigUI(form *huh.Form) configUI {
	return configUI{form: form}
}

func (m configUI) Init() tea.Cmd {
	return m.form.Init()
}

func (m configUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// ── Responsive: ปรับตามขนาดหน้าจอ ───────────────────────────────────────
		showLogo := m.height > 24

		// 1. คำนวณความสูงพื้นที่ตกแต่ง (Border/Padding/Header/Footer)
		decorHeight := 9 // ขั้นต่ำ: header(1) + border(2) + padding(2) + footer(2) + margin(2)
		if showLogo {
			decorHeight = 16 // โลโก้ 5 บรรทัด + subtitle 1 + เว้น 2 + ส่วนที่เหลือ
		}

		// 2. ความสูงให้ Form (ป้องกันค่าติดลบ)
		formHeight := m.height - decorHeight
		if formHeight < 5 {
			formHeight = 5
		}

		// 3. คำนวณความกว้าง UI แบบ responsive
		//    - จอกว้าง ≥ 84: ใช้ 75 col (centered look)
		//    - จอกว้าง 40-83: ยืดเต็มเกือบหมด
		//    - จอแคบ < 40: ปรับให้ fit
		uiWidth := 75
		if m.width < 84 {
			uiWidth = m.width - 4
		}
		if uiWidth < 30 {
			uiWidth = 30
		}

		// 4. formWidth = uiWidth หัก border(2) + padding(6)
		formWidth := uiWidth - 8
		if formWidth < 20 {
			formWidth = 20
		}

		// 5. แจ้งขนาดจริงกับ form
		m.form = m.form.WithHeight(formHeight).WithWidth(formWidth)

		// 6. ส่ง WindowSizeMsg ที่ปรับแล้วให้ form เพื่อให้ scroll ทำงานถูกต้อง
		adjustedMsg := tea.WindowSizeMsg{
			Width:  formWidth,
			Height: formHeight,
		}

		form, cmd := m.form.Update(adjustedMsg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.form.State = huh.StateAborted
			return m, tea.Quit
		}
	}

	// สำหรับ Message อื่นๆ ส่งให้ Form จัดการตามปกติ
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		return m, tea.Quit
	}
	return m, cmd
}

func (m configUI) View() string {
	if m.form.State == huh.StateCompleted || m.form.State == huh.StateAborted {
		return ""
	}

	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	// Colors
	accentDark := lipgloss.Color("#7C3AED")
	dimText := lipgloss.Color("#6B7280")

	// ── Responsive width ─────────────────────────────────────────────────────
	uiWidth := 75
	if w < 84 {
		uiWidth = w - 4
	}
	if uiWidth < 30 {
		uiWidth = 30
	}

	// ── Responsive left margin ────────────────────────────────────────────────
	// จอกว้าง ≥ 84 → margin 4, จอแคบลงให้ลด margin ลงตามสัดส่วน
	marginLeft := 4
	if w < 84 {
		marginLeft = 1
	}

	var headerElements []string

	// ─── ASCII Logo (แสดงเมื่อจอสูง > 24 และกว้าง ≥ 36) ────────────────────
	if h > 24 && w >= 36 {
		wutLogo := `
 ██╗    ██╗██╗   ██╗████████╗
 ██║    ██║██║   ██║╚══██╔══╝
 ██║ █╗ ██║██║   ██║   ██║   
 ╚███╔███╔╝╚██████╔╝   ██║   
  ╚══╝╚══╝  ╚═════╝    ╚═╝   `

		logoStyle := lipgloss.NewStyle().
			Foreground(accentDark).
			Bold(true)

		// Subtitle ซ่อนถ้าจอแคบเกิน
		var logoBlock string
		if w >= 70 {
			subtitleStyle := lipgloss.NewStyle().
				Foreground(dimText).
				MarginBottom(1).
				MarginLeft(1)
			logoBlock = lipgloss.JoinVertical(lipgloss.Left,
				logoStyle.Render(strings.TrimPrefix(wutLogo, "\n")),
				subtitleStyle.Render("The Smart Command Line Assistant That Actually Understands You"),
			)
		} else {
			logoBlock = logoStyle.Render(strings.TrimPrefix(wutLogo, "\n"))
		}
		headerElements = append(headerElements, logoBlock)
	}

	// ─── Header Tab ───────────────────────────────────────────────────────────
	titleText := " ⚙  WUT Configuration "
	if w < 40 {
		titleText = " ⚙ Config "
	}
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(accentDark).
		Padding(0, 1)
	headerElements = append(headerElements, headerStyle.Render(titleText))
	headerBlock := lipgloss.JoinVertical(lipgloss.Left, headerElements...)

	// ─── Form box ─────────────────────────────────────────────────────────────
	boxWidth := uiWidth
	if boxWidth < 30 {
		boxWidth = 30
	}

	boxPadX := 3
	if w < 60 {
		boxPadX = 1
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentDark).
		Padding(1, boxPadX).
		Width(boxWidth)

	body := boxStyle.Render(m.form.View())

	// ─── Footer ───────────────────────────────────────────────────────────────
	footerText := "↑/↓ navigate • enter/tab next • ←/→/space toggle • ctrl+c quit"
	if w < 70 {
		footerText = "↑/↓ nav • enter next • ←/→ toggle • ctrl+c quit"
	}
	if w < 50 {
		footerText = "↑/↓ • enter • ←/→ • ^c"
	}
	footerStyle := lipgloss.NewStyle().Foreground(dimText).MarginTop(1)
	footer := footerStyle.Render(footerText)

	// ─── Container ────────────────────────────────────────────────────────────
	containerStyle := lipgloss.NewStyle().
		MarginLeft(marginLeft).
		MarginTop(1)
	container := containerStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, headerBlock, body, footer),
	)

	return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, container)
}

// ─── Standard configuration theme ──────────────────────────────────────────

func getConfigTheme() *huh.Theme {
	t := huh.ThemeDracula()

	accent := lipgloss.Color("#A78BFA")
	dimText := lipgloss.Color("#6B7280")
	lightText := lipgloss.Color("#E5E7EB")
	bgActive := lipgloss.Color("#A78BFA")
	bgInactive := lipgloss.Color("#374151")

	// Focused state
	t.Focused.Base = t.Focused.Base.Border(lipgloss.HiddenBorder())
	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(lightText)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)

	// Yes/No Buttons Styled as solid blocks
	t.Focused.FocusedButton = lipgloss.NewStyle().
		Background(bgActive).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 2)
	t.Focused.BlurredButton = lipgloss.NewStyle().
		Background(bgInactive).
		Foreground(lightText).
		Padding(0, 2)

	// Blurred state
	t.Blurred.Base = t.Blurred.Base.Border(lipgloss.HiddenBorder())
	t.Blurred.Title = t.Blurred.Title.Foreground(dimText)
	t.Blurred.Description = t.Blurred.Description.Foreground(dimText)
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(dimText)
	t.Blurred.SelectSelector = t.Blurred.SelectSelector.Foreground(dimText)

	// Unfocused confirm
	t.Blurred.FocusedButton = lipgloss.NewStyle().
		Background(lipgloss.Color("#4B5563")).
		Foreground(lipgloss.Color("#9CA3AF")).
		Padding(0, 2)
	t.Blurred.BlurredButton = lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#4B5563")).
		Padding(0, 2)

	return t
}
