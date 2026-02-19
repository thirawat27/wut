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
	// Simplified implementation
	// In production, this would use reflection to get nested values
	return nil, fmt.Errorf("not implemented: get %s", key)
}

func setConfigValue(key, value string) error {
	// Simplified implementation
	// In production, this would parse the key and update the config
	return fmt.Errorf("not implemented: set %s = %s", key, value)
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
