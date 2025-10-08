package main

import (
	"fmt"
	"log/slog"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/logging"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/store"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run jobster in headless mode (no dashboard)",
	Long: `Start the job scheduler in headless mode.

This command loads the configuration file, initializes the scheduler,
and starts all configured jobs. It runs continuously until interrupted
by SIGINT or SIGTERM.

Example:
  jobster run --config ./jobster.yaml`,
	RunE: runScheduler,
}

func init() {
	runCmd.Flags().StringP("config", "c", "jobster.yaml", "Path to configuration file")
	runCmd.MarkFlagRequired("config")
}

func runScheduler(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply logging config from YAML if provided
	if cfg.Logging.Output != "" || cfg.Logging.Level != "" || cfg.Logging.Format != "" {
		runLogger, err := logging.NewFromConfig(cfg.Logging.Format, cfg.Logging.Level, cfg.Logging.Output)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		logger = runLogger
		slog.SetDefault(runLogger)
	}

	logger.Info("starting jobster in run mode", "config", configPath)
	logger.Info("configuration loaded successfully",
		"jobs", len(cfg.Jobs),
		"timezone", cfg.Defaults.Timezone,
		"store_driver", cfg.Store.Driver)

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

	logger.Info("store initialized", "driver", cfg.Store.Driver, "path", cfg.Store.Path)

	// Initialize plugin manager
	pluginMgr := plugins.New(logger)

	logger.Info("plugin manager initialized",
		"timeout_sec", cfg.Defaults.AgentTimeoutSec,
		"fail_on_error", cfg.Defaults.FailOnAgentError,
		"allowed_agents", cfg.Security.AllowedAgents)

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

	logger.Info("scheduler started successfully",
		"scheduled_jobs", len(cfg.Jobs))

	// Wait for shutdown signal
	<-ctx.Done()

	logger.Info("shutting down gracefully...")

	// Stop scheduler
	if err := sched.Stop(); err != nil {
		logger.Error("error during scheduler shutdown", "error", err)
		return err
	}

	logger.Info("jobster stopped")
	return nil
}
