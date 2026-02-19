// Package config provides configuration management for WUT
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	AI        AIConfig        `mapstructure:"ai"`
	NLP       NLPConfig       `mapstructure:"nlp"`
	Fuzzy     FuzzyConfig     `mapstructure:"fuzzy"`
	UI        UIConfig        `mapstructure:"ui"`
	Database  DatabaseConfig  `mapstructure:"database"`
	History   HistoryConfig   `mapstructure:"history"`
	Context   ContextConfig   `mapstructure:"context"`
	Shell     ShellConfig     `mapstructure:"shell"`
	Privacy   PrivacyConfig   `mapstructure:"privacy"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}

// AppConfig holds application settings
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Debug   bool   `mapstructure:"debug"`
}

// AIConfig holds AI/ML settings
type AIConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Model     ModelConfig   `mapstructure:"model"`
	Training  TrainingConfig `mapstructure:"training"`
	Inference InferenceConfig `mapstructure:"inference"`
}

// ModelConfig holds model settings
type ModelConfig struct {
	Type                 string `mapstructure:"type"`
	Version              string `mapstructure:"version"`
	Path                 string `mapstructure:"path"`
	EmbeddingDimensions  int    `mapstructure:"embedding_dimensions"`
	HiddenLayers         int    `mapstructure:"hidden_layers"`
	HiddenUnits          int    `mapstructure:"hidden_units"`
	Quantized            bool   `mapstructure:"quantized"`
}

// TrainingConfig holds training settings
type TrainingConfig struct {
	Epochs           int     `mapstructure:"epochs"`
	LearningRate     float64 `mapstructure:"learning_rate"`
	BatchSize        int     `mapstructure:"batch_size"`
	AutoTrain        bool    `mapstructure:"auto_train"`
	MinHistoryEntries int    `mapstructure:"min_history_entries"`
}

// InferenceConfig holds inference settings
type InferenceConfig struct {
	MaxSuggestions      int     `mapstructure:"max_suggestions"`
	ConfidenceThreshold float64 `mapstructure:"confidence_threshold"`
	CacheEnabled        bool    `mapstructure:"cache_enabled"`
	CacheSize           int     `mapstructure:"cache_size"`
}

// NLPConfig holds NLP settings
type NLPConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	Tokenizer          string `mapstructure:"tokenizer"`
	MaxTokens          int    `mapstructure:"max_tokens"`
	IntentRecognition  bool   `mapstructure:"intent_recognition"`
	SemanticSearch     bool   `mapstructure:"semantic_search"`
}

// FuzzyConfig holds fuzzy matching settings
type FuzzyConfig struct {
	Enabled       bool    `mapstructure:"enabled"`
	CaseSensitive bool    `mapstructure:"case_sensitive"`
	MaxDistance   int     `mapstructure:"max_distance"`
	Threshold     float64 `mapstructure:"threshold"`
}

// UIConfig holds UI settings
type UIConfig struct {
	Theme             string            `mapstructure:"theme"`
	ShowConfidence    bool              `mapstructure:"show_confidence"`
	ShowExplanations  bool              `mapstructure:"show_explanations"`
	SyntaxHighlighting bool             `mapstructure:"syntax_highlighting"`
	Pagination        int               `mapstructure:"pagination"`
	Colors            map[string]string `mapstructure:"colors"`
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Type           string `mapstructure:"type"`
	Path           string `mapstructure:"path"`
	MaxSize        int    `mapstructure:"max_size"`
	BackupEnabled  bool   `mapstructure:"backup_enabled"`
	BackupInterval int    `mapstructure:"backup_interval"`
}

// HistoryConfig holds history settings
type HistoryConfig struct {
	Enabled        bool `mapstructure:"enabled"`
	MaxEntries     int  `mapstructure:"max_entries"`
	TrackFrequency bool `mapstructure:"track_frequency"`
	TrackContext   bool `mapstructure:"track_context"`
	TrackTiming    bool `mapstructure:"track_timing"`
}

// ContextConfig holds context analysis settings
type ContextConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	GitIntegration     bool `mapstructure:"git_integration"`
	ProjectDetection   bool `mapstructure:"project_detection"`
	EnvironmentVars    bool `mapstructure:"environment_vars"`
	DirectoryAnalysis  bool `mapstructure:"directory_analysis"`
}

// ShellConfig holds shell integration settings
type ShellConfig struct {
	Enabled bool            `mapstructure:"enabled"`
	Hooks   map[string]bool `mapstructure:"hooks"`
}

// PrivacyConfig holds privacy settings
type PrivacyConfig struct {
	LocalOnly         bool `mapstructure:"local_only"`
	EncryptData       bool `mapstructure:"encrypt_data"`
	AnonymizeCommands bool `mapstructure:"anonymize_commands"`
	ShareAnalytics    bool `mapstructure:"share_analytics"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

var (
	// globalConfig holds the global configuration instance
	globalConfig *Config
	// configPath is the path to the config file
	configPath string
)

// Load loads the configuration from file and environment variables
func Load(path string) (*Config, error) {
	if path == "" {
		path = getDefaultConfigPath()
	}
	configPath = path

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Set default values
	setDefaults()

	// Read environment variables
	viper.SetEnvPrefix("WUT")
	viper.AutomaticEnv()

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Try to read config file, if it doesn't exist, create default
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || os.IsNotExist(err) {
			// Config file not found, create default
			if err := createDefaultConfig(path); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			// Read the newly created config
			if err := viper.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("failed to read created config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Unmarshal config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand paths
	expandPaths(&cfg)

	globalConfig = &cfg
	return &cfg, nil
}

// Get returns the global configuration instance
func Get() *Config {
	if globalConfig == nil {
		// Load default config if not already loaded
		cfg, err := Load("")
		if err != nil {
			// Return default config on error
			return &Config{}
		}
		return cfg
	}
	return globalConfig
}

// Set updates the global configuration
func Set(cfg *Config) {
	globalConfig = cfg
}

// Save saves the current configuration to file
func Save() error {
	if globalConfig == nil {
		return fmt.Errorf("no configuration to save")
	}

	// Update viper with current config
	viper.Set("app", globalConfig.App)
	viper.Set("ai", globalConfig.AI)
	viper.Set("nlp", globalConfig.NLP)
	viper.Set("fuzzy", globalConfig.Fuzzy)
	viper.Set("ui", globalConfig.UI)
	viper.Set("database", globalConfig.Database)
	viper.Set("history", globalConfig.History)
	viper.Set("context", globalConfig.Context)
	viper.Set("shell", globalConfig.Shell)
	viper.Set("privacy", globalConfig.Privacy)
	viper.Set("logging", globalConfig.Logging)

	// Write to file
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("app.name", "wut")
	viper.SetDefault("app.version", "1.0.0")
	viper.SetDefault("app.debug", false)

	viper.SetDefault("ai.enabled", true)
	viper.SetDefault("ai.model.type", "tiny_neural_network")
	viper.SetDefault("ai.model.embedding_dimensions", 64)
	viper.SetDefault("ai.model.hidden_layers", 2)
	viper.SetDefault("ai.model.hidden_units", 64)
	viper.SetDefault("ai.model.quantized", true)
	viper.SetDefault("ai.training.epochs", 100)
	viper.SetDefault("ai.training.learning_rate", 0.01)
	viper.SetDefault("ai.training.batch_size", 32)
	viper.SetDefault("ai.inference.max_suggestions", 5)
	viper.SetDefault("ai.inference.confidence_threshold", 0.7)

	viper.SetDefault("fuzzy.enabled", true)
	viper.SetDefault("fuzzy.case_sensitive", false)
	viper.SetDefault("fuzzy.max_distance", 3)
	viper.SetDefault("fuzzy.threshold", 0.6)

	viper.SetDefault("ui.theme", "auto")
	viper.SetDefault("ui.show_confidence", true)
	viper.SetDefault("ui.show_explanations", true)
	viper.SetDefault("ui.pagination", 10)

	viper.SetDefault("database.type", "bbolt")
	viper.SetDefault("database.max_size", 100)

	viper.SetDefault("history.enabled", true)
	viper.SetDefault("history.max_entries", 10000)
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(path string) error {
	defaultConfig := `# WUT - AI-Powered Command Helper
# Default Configuration File

app:
  name: "wut"
  version: "1.0.0"
  debug: false

ai:
  enabled: true
  model:
    type: "tiny_neural_network"
    version: "1.0.0"
    path: "~/.wut/models"
    embedding_dimensions: 64
    hidden_layers: 2
    hidden_units: 64
    quantized: true
  training:
    epochs: 100
    learning_rate: 0.01
    batch_size: 32
    auto_train: true
    min_history_entries: 50
  inference:
    max_suggestions: 5
    confidence_threshold: 0.7
    cache_enabled: true
    cache_size: 1000

fuzzy:
  enabled: true
  case_sensitive: false
  max_distance: 3
  threshold: 0.6

ui:
  theme: "auto"
  show_confidence: true
  show_explanations: true
  syntax_highlighting: true
  pagination: 10
  colors:
    primary: "#7C3AED"
    secondary: "#10B981"
    warning: "#F59E0B"
    error: "#EF4444"
    info: "#3B82F6"

database:
  type: "bbolt"
  path: "~/.wut/data"
  max_size: 100
  backup_enabled: true
  backup_interval: 24

history:
  enabled: true
  max_entries: 10000
  track_frequency: true
  track_context: true
  track_timing: true

context:
  enabled: true
  git_integration: true
  project_detection: true
  environment_vars: true
  directory_analysis: true

shell:
  enabled: true
  hooks:
    bash: true
    zsh: true
    fish: true
    powershell: true

privacy:
  local_only: true
  encrypt_data: true
  anonymize_commands: false
  share_analytics: false

logging:
  level: "info"
  file: "~/.wut/logs/wut.log"
  max_size: 10
  max_backups: 5
  max_age: 30
`

	return os.WriteFile(path, []byte(defaultConfig), 0644)
}

// expandPaths expands environment variables and home directory in paths
func expandPaths(cfg *Config) {
	homeDir, _ := os.UserHomeDir()

	// Expand database path
	if cfg.Database.Path != "" {
		cfg.Database.Path = expandPath(cfg.Database.Path, homeDir)
	}

	// Expand model path
	if cfg.AI.Model.Path != "" {
		cfg.AI.Model.Path = expandPath(cfg.AI.Model.Path, homeDir)
	}

	// Expand log path
	if cfg.Logging.File != "" {
		cfg.Logging.File = expandPath(cfg.Logging.File, homeDir)
	}
}

// expandPath expands ~ and environment variables in a path
func expandPath(path, homeDir string) string {
	if len(path) > 0 && path[0] == '~' {
		path = filepath.Join(homeDir, path[1:])
	}
	return os.ExpandEnv(path)
}

// getDefaultConfigPath returns the default configuration file path
func getDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".wut.yaml"
	}

	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "wut", "config.yaml")
	}

	// Use default location
	return filepath.Join(homeDir, ".config", "wut", "config.yaml")
}

// GetDataDir returns the data directory path
func GetDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".wut"
	}
	return filepath.Join(homeDir, ".wut")
}

// EnsureDirs ensures all necessary directories exist
func EnsureDirs() error {
	dataDir := GetDataDir()
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "data"),
		filepath.Join(dataDir, "models"),
		filepath.Join(dataDir, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
