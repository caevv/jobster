package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/logging"
	"github.com/caevv/jobster/internal/plugins"
	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/server"
	"github.com/caevv/jobster/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run jobster with web dashboard",
	Long: `Start the job scheduler with an HTTP dashboard.

This command loads the configuration file, initializes the scheduler,
starts all configured jobs, and serves a web dashboard for monitoring
job execution and history.

Example:
  jobster serve --config ./jobster.yaml --addr :8080`,
	RunE: runServer,
}

func init() {
	serveCmd.Flags().StringP("config", "c", "jobster.yaml", "Path to configuration file")
	serveCmd.Flags().StringP("addr", "a", ":8080", "HTTP server address (host:port)")
	serveCmd.MarkFlagRequired("config")
}

func runServer(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	addr, _ := cmd.Flags().GetString("addr")

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply logging config from YAML if provided
	if cfg.Logging.Output != "" || cfg.Logging.Level != "" || cfg.Logging.Format != "" {
		serveLogger, err := logging.NewFromConfig(cfg.Logging.Format, cfg.Logging.Level, cfg.Logging.Output)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		logger = serveLogger
		slog.SetDefault(serveLogger)
	}

	logger.Info("starting jobster in serve mode",
		"config", configPath,
		"addr", addr)
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

	// Create adapters for server
	storeAdapter := server.NewStoreAdapter(st)
	schedAdapter := server.NewSchedulerAdapter(sched)

	// Initialize HTTP server
	srv := server.New(addr, storeAdapter, schedAdapter, logger)

	// Use errgroup to run scheduler and server concurrently
	g, gCtx := errgroup.WithContext(ctx)

	// Start scheduler
	g.Go(func() error {
		logger.Info("starting scheduler...")
		if err := sched.Start(); err != nil {
			return fmt.Errorf("scheduler error: %w", err)
		}
		// Scheduler runs until context is cancelled
		<-gCtx.Done()
		return nil
	})

	// Start HTTP server
	g.Go(func() error {
		logger.Info("starting HTTP server", "addr", addr)
		if err := srv.Start(gCtx); err != nil && err != context.Canceled {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	})

	// Shutdown handler
	g.Go(func() error {
		<-gCtx.Done()
		logger.Info("shutting down gracefully...")

		// Create shutdown context with timeout
		shutdownCtx := context.Background()

		// Stop scheduler first
		if err := sched.Stop(); err != nil {
			logger.Error("error stopping scheduler", "error", err)
		}

		// Stop HTTP server
		if err := srv.Stop(shutdownCtx); err != nil {
			logger.Error("error stopping server", "error", err)
		}

		return nil
	})

	logger.Info("jobster serve mode started successfully",
		"scheduled_jobs", len(cfg.Jobs),
		"dashboard_url", fmt.Sprintf("http://localhost%s", addr))

	// Wait for all goroutines
	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error("error during execution", "error", err)
		return err
	}

	logger.Info("jobster stopped")
	return nil
}
