// Package cmd provides CLI commands for WUT
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"wut/internal/config"
	"wut/internal/health"
	"wut/internal/logger"
	"wut/internal/metrics"
)

var (
	// Version is set during build
	Version = "dev"
	// BuildTime is set during build
	BuildTime = "unknown"
	// Commit is set during build
	Commit = "unknown"

	cfgFile string
	debug   bool

	// rootCmd represents the base command
	rootCmd = &cobra.Command{
		Use:   "wut",
		Short: "AI-Powered Command Helper",
		Long: `WUT is an intelligent command line assistant that helps you 
find the right commands, correct typos, and learn new shell commands 
through natural language queries.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildTime),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initialize(cmd.Context())
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			cleanup()
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("received shutdown signal, shutting down gracefully...")
		cancel()
	}()

	// Set context for root command
	rootCmd.SetContext(ctx)

	if err := rootCmd.Execute(); err != nil {
		logger.Error("command execution failed", "error", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/wut/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Configuration will be loaded in initialize()
}

// initialize performs initialization before command execution
func initialize(ctx context.Context) error {
	// Initialize logger first
	logCfg := logger.DefaultConfig()
	if debug {
		logCfg.Level = "debug"
	}

	if err := logger.Initialize(logCfg); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	log := logger.With("init")
	log.Info("starting WUT", "version", Version, "commit", Commit, "build_time", BuildTime)

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Error("failed to load configuration", "error", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override debug from flag
	if debug {
		cfg.App.Debug = true
	}

	// Ensure directories exist
	if err := config.EnsureDirs(); err != nil {
		log.Error("failed to create directories", "error", err)
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Initialize metrics
	metrics.Initialize(Version, Commit)

	// Initialize health checker
	healthChecker := health.NewChecker(Version)
	healthChecker.RegisterDefaultChecks()

	// Log startup information
	log.Info("initialization complete",
		"config_file", cfgFile,
		"debug", cfg.App.Debug,
	)

	return nil
}

// cleanup performs cleanup after command execution
func cleanup() {
	log := logger.With("cleanup")
	log.Info("performing cleanup")

	// Flush logger
	if err := logger.Get().Sync(); err != nil {
		// Ignore sync errors
		_ = err
	}

	log.Info("cleanup complete")
}
