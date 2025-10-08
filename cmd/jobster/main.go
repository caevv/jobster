package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	// Version information (set via ldflags at build time)
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"

	// Global logger
	logger *slog.Logger
)

func main() {
	// Initialize structured logger
	logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger = slog.New(logHandler)
	slog.SetDefault(logger)

	// Execute root command
	if err := rootCmd.Execute(); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "jobster",
	Short: "A lightweight, plugin-based cron job runner",
	Long: `Jobster is a minimal cron-style scheduler with YAML-based job configuration,
pluggable hooks (agents), and optional web dashboard.

Features:
  - YAML-based job definitions
  - Cron expression support
  - Pluggable hooks (pre_run, post_run, on_success, on_error)
  - Execution history tracking
  - Optional web dashboard
  - Graceful shutdown with signal handling`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildTime),
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			// Recreate logger with debug level
			logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			logger = slog.New(logHandler)
			slog.SetDefault(logger)
			logger.Debug("debug logging enabled")
		}
	}

	// Register subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(jobCmd)
}

// setupSignalHandler creates a context that cancels on SIGINT or SIGTERM
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig.String())
		cancel()

		// Force exit if second signal received
		sig = <-sigChan
		logger.Warn("received second signal, forcing exit", "signal", sig.String())
		os.Exit(1)
	}()

	return ctx
}
