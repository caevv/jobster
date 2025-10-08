package main

import (
	"fmt"
	"os"

	"github.com/caevv/jobster/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate jobster configuration file",
	Long: `Validate the syntax and semantics of a jobster configuration file.

This command loads and validates the configuration file without starting
the scheduler. It checks for:
  - Valid YAML syntax
  - Required fields
  - Valid cron expressions
  - Valid time zones
  - Valid store driver configuration
  - Valid agent references

Example:
  jobster validate --config ./jobster.yaml`,
	RunE: validateConfig,
}

func init() {
	validateCmd.Flags().StringP("config", "c", "jobster.yaml", "Path to configuration file")
	validateCmd.MarkFlagRequired("config")
}

func validateConfig(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	logger.Info("validating configuration", "path", configPath)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Error("configuration file not found", "path", configPath)
		return fmt.Errorf("configuration file not found: %s", configPath)
	}

	// Load and validate configuration (LoadConfig validates automatically)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("configuration validation failed", "error", err)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Print validation summary
	logger.Info("configuration is valid",
		"path", configPath,
		"jobs", len(cfg.Jobs),
		"timezone", cfg.Defaults.Timezone,
		"store_driver", cfg.Store.Driver,
		"agent_timeout_sec", cfg.Defaults.AgentTimeoutSec)

	// Print job details
	for i, job := range cfg.Jobs {
		logger.Info(fmt.Sprintf("job %d", i+1),
			"id", job.ID,
			"schedule", job.Schedule,
			"command", job.Command,
			"timeout_sec", job.TimeoutSec,
			"workdir", job.Workdir)

		// Print hooks if present
		if len(job.Hooks.PreRun) > 0 {
			logger.Debug("hooks configured",
				"job_id", job.ID,
				"hook_type", "pre_run",
				"count", len(job.Hooks.PreRun))
		}
		if len(job.Hooks.PostRun) > 0 {
			logger.Debug("hooks configured",
				"job_id", job.ID,
				"hook_type", "post_run",
				"count", len(job.Hooks.PostRun))
		}
		if len(job.Hooks.OnSuccess) > 0 {
			logger.Debug("hooks configured",
				"job_id", job.ID,
				"hook_type", "on_success",
				"count", len(job.Hooks.OnSuccess))
		}
		if len(job.Hooks.OnError) > 0 {
			logger.Debug("hooks configured",
				"job_id", job.ID,
				"hook_type", "on_error",
				"count", len(job.Hooks.OnError))
		}
	}

	fmt.Fprintf(os.Stdout, "\n✓ Configuration is valid: %s\n", configPath)
	fmt.Fprintf(os.Stdout, "  Jobs: %d\n", len(cfg.Jobs))
	fmt.Fprintf(os.Stdout, "  Store: %s (%s)\n", cfg.Store.Driver, cfg.Store.Path)
	fmt.Fprintf(os.Stdout, "  Timezone: %s\n", cfg.Defaults.Timezone)

	return nil
}
