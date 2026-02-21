// Package cmd provides CLI commands for WUT
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"wut/internal/config"
	"wut/internal/logger"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View, get, set, and reset configuration values.`,
	Example: `  wut config
  wut config --list
  wut config --get ai.enabled
  wut config --set theme dark
  wut config --reset`,
	RunE: runConfig,
}

var (
	configList   bool
	configGet    string
	configSet    string
	configValue  string
	configReset  bool
)

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().BoolVarP(&configList, "list", "l", false, "list all configuration")
	configCmd.Flags().StringVarP(&configGet, "get", "g", "", "get configuration value")
	configCmd.Flags().StringVarP(&configSet, "set", "s", "", "set configuration key")
	configCmd.Flags().StringVarP(&configValue, "value", "v", "", "value to set")
	configCmd.Flags().BoolVarP(&configReset, "reset", "r", false, "reset to default configuration")
}

func runConfig(cmd *cobra.Command, args []string) error {
	log := logger.With("config")

	// Handle reset
	if configReset {
		if err := resetConfig(); err != nil {
			log.Error("failed to reset config", "error", err)
			return fmt.Errorf("failed to reset config: %w", err)
		}
		fmt.Println("Configuration reset to defaults")
		return nil
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
		fmt.Printf("Set %s = %v\n", configSet, configValue)
		return nil
	}

	// Default: show configuration
	return showConfig()
}

func showConfig() error {
	cfg := config.Get()

	fmt.Println()
	fmt.Println("Current Configuration")
	fmt.Println("=====================")
	fmt.Println()

	// App config
	fmt.Println("App:")
	fmt.Printf("  Name: %s\n", cfg.App.Name)
	fmt.Printf("  Version: %s\n", cfg.App.Version)
	fmt.Printf("  Debug: %v\n", cfg.App.Debug)
	fmt.Println()

	// AI config
	fmt.Println("AI:")
	fmt.Printf("  Enabled: %v\n", cfg.AI.Enabled)
	fmt.Printf("  Model Type: %s\n", cfg.AI.Model.Type)
	fmt.Printf("  Embedding Dimensions: %d\n", cfg.AI.Model.EmbeddingDimensions)
	fmt.Printf("  Hidden Layers: %d\n", cfg.AI.Model.HiddenLayers)
	fmt.Printf("  Hidden Units: %d\n", cfg.AI.Model.HiddenUnits)
	fmt.Printf("  Quantized: %v\n", cfg.AI.Model.Quantized)
	fmt.Println()

	// Training config
	fmt.Println("Training:")
	fmt.Printf("  Epochs: %d\n", cfg.AI.Training.Epochs)
	fmt.Printf("  Learning Rate: %f\n", cfg.AI.Training.LearningRate)
	fmt.Printf("  Batch Size: %d\n", cfg.AI.Training.BatchSize)
	fmt.Printf("  Auto Train: %v\n", cfg.AI.Training.AutoTrain)
	fmt.Printf("  Min History Entries: %d\n", cfg.AI.Training.MinHistoryEntries)
	fmt.Println()

	// Inference config
	fmt.Println("Inference:")
	fmt.Printf("  Max Suggestions: %d\n", cfg.AI.Inference.MaxSuggestions)
	fmt.Printf("  Confidence Threshold: %.2f\n", cfg.AI.Inference.ConfidenceThreshold)
	fmt.Printf("  Cache Enabled: %v\n", cfg.AI.Inference.CacheEnabled)
	fmt.Println()

	// UI config
	fmt.Println("UI:")
	fmt.Printf("  Theme: %s\n", cfg.UI.Theme)
	fmt.Printf("  Show Confidence: %v\n", cfg.UI.ShowConfidence)
	fmt.Printf("  Show Explanations: %v\n", cfg.UI.ShowExplanations)
	fmt.Printf("  Syntax Highlighting: %v\n", cfg.UI.SyntaxHighlighting)
	fmt.Printf("  Pagination: %d\n", cfg.UI.Pagination)
	fmt.Println()

	// Database config
	fmt.Println("Database:")
	fmt.Printf("  Type: %s\n", cfg.Database.Type)
	fmt.Printf("  Path: %s\n", cfg.Database.Path)
	fmt.Printf("  Max Size: %d MB\n", cfg.Database.MaxSize)
	fmt.Printf("  Backup Enabled: %v\n", cfg.Database.BackupEnabled)
	fmt.Println()

	// History config
	fmt.Println("History:")
	fmt.Printf("  Enabled: %v\n", cfg.History.Enabled)
	fmt.Printf("  Max Entries: %d\n", cfg.History.MaxEntries)
	fmt.Printf("  Track Frequency: %v\n", cfg.History.TrackFrequency)
	fmt.Printf("  Track Context: %v\n", cfg.History.TrackContext)
	fmt.Println()

	// Context config
	fmt.Println("Context:")
	fmt.Printf("  Enabled: %v\n", cfg.Context.Enabled)
	fmt.Printf("  Git Integration: %v\n", cfg.Context.GitIntegration)
	fmt.Printf("  Project Detection: %v\n", cfg.Context.ProjectDetection)
	fmt.Println()

	// Privacy config
	fmt.Println("Privacy:")
	fmt.Printf("  Local Only: %v\n", cfg.Privacy.LocalOnly)
	fmt.Printf("  Encrypt Data: %v\n", cfg.Privacy.EncryptData)
	fmt.Printf("  Anonymize Commands: %v\n", cfg.Privacy.AnonymizeCommands)
	fmt.Println()

	// Logging config
	fmt.Println("Logging:")
	fmt.Printf("  Level: %s\n", cfg.Logging.Level)
	fmt.Printf("  File: %s\n", cfg.Logging.File)
	fmt.Printf("  Max Size: %d MB\n", cfg.Logging.MaxSize)
	fmt.Printf("  Max Backups: %d\n", cfg.Logging.MaxBackups)
	fmt.Printf("  Max Age: %d days\n", cfg.Logging.MaxAge)
	fmt.Println()

	fmt.Printf("Configuration file: %s\n", getConfigFile())

	return nil
}

func getConfigValue(key string) (interface{}, error) {
	cfg := config.Get()
	
	// Simple key lookup for common config values
	switch key {
	case "ai.enabled":
		return cfg.AI.Enabled, nil
	case "ai.model.type":
		return cfg.AI.Model.Type, nil
	case "ai.model.quantized":
		return cfg.AI.Model.Quantized, nil
	case "ai.training.epochs":
		return cfg.AI.Training.Epochs, nil
	case "ai.training.learning_rate":
		return cfg.AI.Training.LearningRate, nil
	case "ai.training.batch_size":
		return cfg.AI.Training.BatchSize, nil
	case "ai.inference.max_suggestions":
		return cfg.AI.Inference.MaxSuggestions, nil
	case "ai.inference.confidence_threshold":
		return cfg.AI.Inference.ConfidenceThreshold, nil
	case "ui.theme":
		return cfg.UI.Theme, nil
	case "ui.show_confidence":
		return cfg.UI.ShowConfidence, nil
	case "ui.show_explanations":
		return cfg.UI.ShowExplanations, nil
	case "database.path":
		return cfg.Database.Path, nil
	case "history.enabled":
		return cfg.History.Enabled, nil
	case "history.max_entries":
		return cfg.History.MaxEntries, nil
	case "fuzzy.enabled":
		return cfg.Fuzzy.Enabled, nil
	case "fuzzy.threshold":
		return cfg.Fuzzy.Threshold, nil
	case "context.enabled":
		return cfg.Context.Enabled, nil
	case "logging.level":
		return cfg.Logging.Level, nil
	case "app.debug":
		return cfg.App.Debug, nil
	default:
		return nil, fmt.Errorf("unknown config key: %s", key)
	}
}

func setConfigValue(key, value string) error {
	cfg := config.Get()
	
	// Simple key update for common config values
	switch key {
	case "ai.enabled":
		cfg.AI.Enabled = value == "true" || value == "1"
	case "ai.model.type":
		cfg.AI.Model.Type = value
	case "ai.model.quantized":
		cfg.AI.Model.Quantized = value == "true" || value == "1"
	case "ai.training.epochs":
		// Parse int value
		var epochs int
		if _, err := fmt.Sscanf(value, "%d", &epochs); err == nil {
			cfg.AI.Training.Epochs = epochs
		}
	case "ai.training.learning_rate":
		// Parse float value
		var lr float64
		if _, err := fmt.Sscanf(value, "%f", &lr); err == nil {
			cfg.AI.Training.LearningRate = lr
		}
	case "ai.training.batch_size":
		// Parse int value
		var bs int
		if _, err := fmt.Sscanf(value, "%d", &bs); err == nil {
			cfg.AI.Training.BatchSize = bs
		}
	case "ai.inference.max_suggestions":
		// Parse int value
		var ms int
		if _, err := fmt.Sscanf(value, "%d", &ms); err == nil {
			cfg.AI.Inference.MaxSuggestions = ms
		}
	case "ai.inference.confidence_threshold":
		// Parse float value
		var ct float64
		if _, err := fmt.Sscanf(value, "%f", &ct); err == nil {
			cfg.AI.Inference.ConfidenceThreshold = ct
		}
	case "ui.theme":
		cfg.UI.Theme = value
	case "ui.show_confidence":
		cfg.UI.ShowConfidence = value == "true" || value == "1"
	case "ui.show_explanations":
		cfg.UI.ShowExplanations = value == "true" || value == "1"
	case "database.path":
		cfg.Database.Path = value
	case "history.enabled":
		cfg.History.Enabled = value == "true" || value == "1"
	case "history.max_entries":
		// Parse int value
		var me int
		if _, err := fmt.Sscanf(value, "%d", &me); err == nil {
			cfg.History.MaxEntries = me
		}
	case "fuzzy.enabled":
		cfg.Fuzzy.Enabled = value == "true" || value == "1"
	case "fuzzy.threshold":
		// Parse float value
		var th float64
		if _, err := fmt.Sscanf(value, "%f", &th); err == nil {
			cfg.Fuzzy.Threshold = th
		}
	case "context.enabled":
		cfg.Context.Enabled = value == "true" || value == "1"
	case "logging.level":
		cfg.Logging.Level = value
	case "app.debug":
		cfg.App.Debug = value == "true" || value == "1"
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	
	// Save the updated config
	return config.Save()
}

func resetConfig() error {
	// Create default config
	if err := config.Save(); err != nil {
		return err
	}
	return nil
}

func getConfigFile() string {
	return config.GetDataDir() + "/config.yaml"
}
