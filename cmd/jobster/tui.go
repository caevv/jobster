package main

import (
	"fmt"
	"log/slog"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/logging"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/store"
	"github.com/caevv/jobster/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Run jobster with a terminal UI dashboard",
	Long: `Start the job scheduler with an interactive terminal UI.

This command loads the configuration file, initializes the scheduler,
and displays a beautiful terminal dashboard showing real-time job status,
recent runs, and statistics.

Navigation:
  ↑/↓ or k/j  - Navigate job list
  enter       - View job details (history, logs, error messages)
  esc         - Go back to job list
  g/G         - Jump to top/bottom
  r           - Refresh data
  q           - Quit

Example:
  jobster tui --config ./jobster.yaml`,
	RunE: runTUI,
}

func init() {
	tuiCmd.Flags().StringP("config", "c", "jobster.yaml", "Path to configuration file")
	tuiCmd.MarkFlagRequired("config")
}

func runTUI(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// In TUI mode, suppress logs by default unless configured otherwise
	logOutput := cfg.Logging.Output
	if logOutput == "" {
		// Default to discard in TUI mode to avoid polluting the interface
		logOutput = "discard"
	}

	// Create logger from config (or use discard for TUI)
	tuiLogger, err := logging.NewFromConfig(cfg.Logging.Format, cfg.Logging.Level, logOutput)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	logger = tuiLogger
	slog.SetDefault(tuiLogger)

	// Initialize store for run history
	st, err := store.NewStore(cfg.Store.Driver, cfg.Store.Path)
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("failed to close store", "error", err)
		}
	}()

	// Initialize plugin manager
	pluginMgr := plugins.New(logger)

	// Create job runner
	runner := NewRunner(st, pluginMgr, cfg.Defaults, logger)

	// Setup signal handling for graceful shutdown
	ctx := setupSignalHandler()

	// Initialize scheduler
	sched := scheduler.New(ctx, logger)

	// Add jobs to scheduler
	for i := range cfg.Jobs {
		if err := sched.AddJob(&cfg.Jobs[i], runner); err != nil {
			return fmt.Errorf("failed to add job %s: %w", cfg.Jobs[i].ID, err)
		}
	}

	// Start scheduler
	if err := sched.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// Initialize TUI model
	model := tui.New(cfg, st, sched, logger)

	// Create Bubbletea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		logger.Error("TUI error", "error", err)
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if we should quit
	if m, ok := finalModel.(tui.Model); ok && m.Quitting() {
		logger.Info("shutting down gracefully...")

		// Stop scheduler
		if err := sched.Stop(); err != nil {
			logger.Error("error during scheduler shutdown", "error", err)
			return err
		}

		logger.Info("jobster stopped")
	}

	return nil
}
