package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/caevv/jobster/internal/config"
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage jobs in the configuration",
	Long: `Manage cron jobs in the Jobster configuration file.

Subcommands:
  add     - Add a new job to the configuration
  list    - List all jobs in the configuration
  remove  - Remove a job from the configuration

Examples:
  jobster job add backup --schedule "@daily" --command "/usr/bin/backup.sh"
  jobster job list --config jobster.yaml
  jobster job remove backup --config jobster.yaml`,
}

var addJobCmd = &cobra.Command{
	Use:   "add [job-id]",
	Short: "Add a new job to the configuration",
	Long: `Add a new cron job to the Jobster configuration file.

If --interactive flag is used, the command will prompt for all job details.
Otherwise, job-id and required flags must be provided.

Examples:
  # Add simple job
  jobster job add daily-backup --schedule "@daily" --command "/usr/bin/backup.sh"

  # Add job with options
  jobster job add api-check \
    --schedule "*/5 * * * *" \
    --command "curl http://api/health" \
    --timeout 30 \
    --env "API_KEY=secret" \
    --env "TIMEOUT=10"

  # Interactive mode
  jobster job add --interactive`,
	RunE: runAddJob,
}

var listJobsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all jobs in the configuration",
	Long: `List all configured cron jobs from the Jobster configuration file.

Displays job ID, schedule, and command in a table format.

Example:
  jobster job list --config jobster.yaml`,
	RunE: runListJobs,
}

var removeJobCmd = &cobra.Command{
	Use:   "remove [job-id]",
	Short: "Remove a job from the configuration",
	Long: `Remove a cron job from the Jobster configuration file by its ID.

Example:
  jobster job remove daily-backup --config jobster.yaml`,
	RunE: runRemoveJob,
	Args: cobra.ExactArgs(1),
}

func init() {
	// Add subcommands
	jobCmd.AddCommand(addJobCmd)
	jobCmd.AddCommand(listJobsCmd)
	jobCmd.AddCommand(removeJobCmd)

	// Common flags
	jobCmd.PersistentFlags().StringP("config", "c", "jobster.yaml", "Path to configuration file")

	// Add command flags
	addJobCmd.Flags().String("schedule", "", "Cron expression or @-notation (required unless --interactive)")
	addJobCmd.Flags().String("command", "", "Command to execute (required unless --interactive)")
	addJobCmd.Flags().String("workdir", "", "Working directory")
	addJobCmd.Flags().Int("timeout", 600, "Timeout in seconds")
	addJobCmd.Flags().StringSlice("env", []string{}, "Environment variables (KEY=VALUE, repeatable)")
	addJobCmd.Flags().BoolP("interactive", "i", false, "Interactive mode with prompts")
}

func runAddJob(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	interactive, _ := cmd.Flags().GetBool("interactive")

	var job config.Job
	var err error

	if interactive {
		// Interactive mode
		job, err = promptForJob()
		if err != nil {
			return fmt.Errorf("failed to get job details: %w", err)
		}
	} else {
		// Flag mode
		if len(args) == 0 {
			return fmt.Errorf("job ID is required (or use --interactive flag)")
		}

		jobID := args[0]
		schedule, _ := cmd.Flags().GetString("schedule")
		command, _ := cmd.Flags().GetString("command")
		workdir, _ := cmd.Flags().GetString("workdir")
		timeout, _ := cmd.Flags().GetInt("timeout")
		envVars, _ := cmd.Flags().GetStringSlice("env")

		if schedule == "" {
			return fmt.Errorf("--schedule flag is required")
		}
		if command == "" {
			return fmt.Errorf("--command flag is required")
		}

		// Parse environment variables
		env := make(map[string]string)
		for _, envVar := range envVars {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", envVar)
			}
			env[parts[0]] = parts[1]
		}

		job = config.Job{
			ID:         jobID,
			Schedule:   schedule,
			Command:    command,
			Workdir:    workdir,
			TimeoutSec: timeout,
			Env:        env,
			Hooks:      config.Hooks{},
		}
	}

	// Validate schedule
	if err := config.ValidateSchedule(job.Schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Add job to config
	if err := config.AddJob(configPath, job); err != nil {
		return fmt.Errorf("failed to add job: %w", err)
	}

	fmt.Printf("✓ Job '%s' added successfully to %s\n", job.ID, configPath)
	fmt.Printf("  Schedule: %s\n", job.Schedule)
	fmt.Printf("  Command:  %s\n", job.Command)

	return nil
}

func runListJobs(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	// Load config
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Jobs) == 0 {
		fmt.Println("No jobs configured")
		return nil
	}

	// Print jobs in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSCHEDULE\tCOMMAND\tWORKDIR\tTIMEOUT")
	fmt.Fprintln(w, "──\t────────\t───────\t───────\t───────")

	for _, job := range cfg.Jobs {
		workdir := job.Workdir
		if workdir == "" {
			workdir = "."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%ds\n",
			job.ID,
			job.Schedule,
			truncate(job.Command, 40),
			workdir,
			job.TimeoutSec,
		)
	}

	w.Flush()
	fmt.Printf("\nTotal jobs: %d\n", len(cfg.Jobs))

	return nil
}

func runRemoveJob(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	jobID := args[0]

	// Remove job from config
	if err := config.RemoveJob(configPath, jobID); err != nil {
		return fmt.Errorf("failed to remove job: %w", err)
	}

	fmt.Printf("✓ Job '%s' removed successfully from %s\n", jobID, configPath)

	return nil
}

// Helper functions

func promptForJob() (config.Job, error) {
	reader := bufio.NewReader(os.Stdin)
	job := config.Job{
		Env:   make(map[string]string),
		Hooks: config.Hooks{},
	}

	// Job ID
	fmt.Print("Job ID: ")
	id, err := reader.ReadString('\n')
	if err != nil {
		return job, err
	}
	job.ID = strings.TrimSpace(id)

	// Schedule
	fmt.Print("Schedule (cron expression or @notation): ")
	schedule, err := reader.ReadString('\n')
	if err != nil {
		return job, err
	}
	job.Schedule = strings.TrimSpace(schedule)

	// Command
	fmt.Print("Command: ")
	command, err := reader.ReadString('\n')
	if err != nil {
		return job, err
	}
	job.Command = strings.TrimSpace(command)

	// Working directory (optional)
	fmt.Print("Working directory (optional, press Enter to skip): ")
	workdir, err := reader.ReadString('\n')
	if err != nil {
		return job, err
	}
	job.Workdir = strings.TrimSpace(workdir)

	// Timeout
	fmt.Print("Timeout in seconds (default 600): ")
	timeoutStr, err := reader.ReadString('\n')
	if err != nil {
		return job, err
	}
	timeoutStr = strings.TrimSpace(timeoutStr)
	if timeoutStr == "" {
		job.TimeoutSec = 600
	} else {
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return job, fmt.Errorf("invalid timeout: %w", err)
		}
		job.TimeoutSec = timeout
	}

	// Environment variables (optional)
	fmt.Print("Add environment variables? (y/N): ")
	addEnv, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(addEnv)) == "y" {
		fmt.Println("Enter environment variables (KEY=VALUE, one per line, empty line to finish):")
		for {
			fmt.Print("  ")
			envVar, _ := reader.ReadString('\n')
			envVar = strings.TrimSpace(envVar)
			if envVar == "" {
				break
			}
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) != 2 {
				fmt.Println("  Invalid format, use KEY=VALUE")
				continue
			}
			job.Env[parts[0]] = parts[1]
		}
	}

	// Preview
	fmt.Println("\n=== Job Preview ===")
	fmt.Printf("ID:       %s\n", job.ID)
	fmt.Printf("Schedule: %s\n", job.Schedule)
	fmt.Printf("Command:  %s\n", job.Command)
	if job.Workdir != "" {
		fmt.Printf("Workdir:  %s\n", job.Workdir)
	}
	fmt.Printf("Timeout:  %ds\n", job.TimeoutSec)
	if len(job.Env) > 0 {
		fmt.Println("Environment:")
		for k, v := range job.Env {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	// Confirm
	fmt.Print("\nAdd this job? (Y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.ToLower(strings.TrimSpace(confirm))
	if confirm != "" && confirm != "y" && confirm != "yes" {
		return job, fmt.Errorf("job creation cancelled")
	}

	return job, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
