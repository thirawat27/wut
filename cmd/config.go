// Package cmd provides CLI commands for WUT
package cmd

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/logger"
	"wut/internal/ui"

	"github.com/charmbracelet/huh"
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

	// Default: show configuration wizard
	return runConfigUI()
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

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("WUT Configuration Wizards").
				Description("Welcome to the WUT configuration wizard.\nPress Tab to navigate, Space to toggle, Enter to save settings."),
		).Title("Welcome"),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("UI Theme").
				Options(
					huh.NewOption("Auto", "auto"),
					huh.NewOption("Light", "light"),
					huh.NewOption("Dark", "dark"),
				).
				Value(&cfg.UI.Theme),
			huh.NewConfirm().
				Title("Show Confidence").
				Description("Display the AI's confidence level for responses").
				Value(&cfg.UI.ShowConfidence),
			huh.NewConfirm().
				Title("Show Explanations").
				Description("Display detailed explanations for commands").
				Value(&cfg.UI.ShowExplanations),
			huh.NewConfirm().
				Title("Syntax Highlighting").
				Value(&cfg.UI.SyntaxHighlighting),
			huh.NewInput().
				Title("Pagination Size").
				Value(&uiPagination),
		).Title("User Interface"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Fuzzy Matching").
				Description("Enable typo correction and fuzzy searching").
				Value(&cfg.Fuzzy.Enabled),
			huh.NewConfirm().
				Title("Case Sensitive").
				Value(&cfg.Fuzzy.CaseSensitive),
			huh.NewInput().
				Title("Max Distance").
				Description("Maximum Levenshtein distance for fuzzy matching").
				Value(&fuzzyDistance),
			huh.NewInput().
				Title("Threshold (0.0 to 1.0)").
				Description("Confidence threshold for matching").
				Value(&fuzzyThreshold),
		).Title("Fuzzy Matching"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("TLDR Pages Enabled").
				Value(&cfg.TLDR.Enabled),
			huh.NewConfirm().
				Title("Offline Mode").
				Description("Never attempt to fetch pages online").
				Value(&cfg.TLDR.OfflineMode),
			huh.NewConfirm().
				Title("Auto Sync TLDR Pages").
				Value(&cfg.TLDR.AutoSync),
			huh.NewInput().
				Title("Sync Interval (days)").
				Value(&tldrSyncInterval),
			huh.NewSelect[string]().
				Title("Language").
				Options(
					huh.NewOption("English", "en"),
					huh.NewOption("Thai", "th"),
				).
				Value(&cfg.TLDR.Language),
		).Title("TLDR Pages"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Context Analysis").
				Description("Analyze working directory for better suggestions").
				Value(&cfg.Context.Enabled),
			huh.NewConfirm().
				Title("Git Integration").
				Description("Use Git status and history for context").
				Value(&cfg.Context.GitIntegration),
			huh.NewConfirm().
				Title("Project Detection").
				Description("Detect project type (Node, Go, Python, etc.)").
				Value(&cfg.Context.ProjectDetection),
			huh.NewConfirm().
				Title("Environment Vars").
				Value(&cfg.Context.EnvironmentVars),
		).Title("Context Analysis"),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Database Type").
				Options(
					huh.NewOption("bbolt", "bbolt"),
					huh.NewOption("sqlite", "sqlite"),
				).
				Value(&cfg.Database.Type),
			huh.NewInput().
				Title("Max Size (MB)").
				Value(&dbSize),
			huh.NewConfirm().
				Title("Backup Enabled").
				Value(&cfg.Database.BackupEnabled),
		).Title("Database Settings"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Track History").
				Value(&cfg.History.Enabled),
			huh.NewInput().
				Title("Max Entries").
				Value(&historyMaxEntries),
			huh.NewConfirm().
				Title("Track Frequency").
				Value(&cfg.History.TrackFrequency),
		).Title("History Settings"),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Local Only").
				Description("Never send data to external APIs").
				Value(&cfg.Privacy.LocalOnly),
			huh.NewConfirm().
				Title("Encrypt Local Data").
				Value(&cfg.Privacy.EncryptData),
			huh.NewConfirm().
				Title("Anonymize Commands").
				Description("Remove sensitive data from command history").
				Value(&cfg.Privacy.AnonymizeCommands),
		).Title("Privacy Settings"),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Log Level").
				Options(
					huh.NewOption("debug", "debug"),
					huh.NewOption("info", "info"),
					huh.NewOption("warn", "warn"),
					huh.NewOption("error", "error"),
				).
				Value(&cfg.Logging.Level),
			huh.NewInput().
				Title("Max Size (MB)").
				Value(&logMaxSize),
			huh.NewInput().
				Title("Max Age (days)").
				Value(&logMaxAge),
		).Title("Logging Settings"),
	).
		WithTheme(huh.ThemeDracula())

	err := form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("Configuration cancelled.")
			return nil
		}
		return err
	}

	// Parsing strings back to numerical values
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
