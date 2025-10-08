package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/caevv/jobster/internal/store"
	"github.com/charmbracelet/lipgloss"
)

// View renders the UI.
func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	// Switch between list and detail view
	if m.viewMode == ViewModeDetail {
		return m.renderDetailView()
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Stats bar
	sections = append(sections, m.renderStats())

	// Job list
	sections = append(sections, m.renderJobList())

	// Recent runs
	sections = append(sections, m.renderRecentRuns())

	// Help/Status bar
	sections = append(sections, m.renderHelpBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderHeader renders the dashboard header.
func (m Model) renderHeader() string {
	title := titleStyle.Render("⚡ Jobster Dashboard")
	subtitle := subtitleStyle.Render(fmt.Sprintf("Last updated: %s", m.lastUpdate.Format("15:04:05")))

	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", subtitle)
	return headerStyle.Render(header)
}

// renderStats renders the statistics bar.
func (m Model) renderStats() string {
	stats := []string{
		fmt.Sprintf("%s %d", keyStyle.Render("Jobs:"), m.totalJobs),
		fmt.Sprintf("%s %d", keyStyle.Render("Running:"), m.runningJobs),
	}

	if m.totalRuns > 0 {
		successRate := float64(m.successRuns) / float64(m.totalRuns) * 100
		stats = append(stats, fmt.Sprintf("%s %d/%d (%.0f%%)",
			keyStyle.Render("Success:"),
			m.successRuns,
			m.totalRuns,
			successRate,
		))
	}

	content := strings.Join(stats, "  │  ")
	return statsStyle.Render(content)
}

// renderJobList renders the list of jobs.
func (m Model) renderJobList() string {
	if len(m.jobs) == 0 {
		return jobListStyle.Render(subtitleStyle.Render("No jobs configured"))
	}

	var rows []string

	// Title
	rows = append(rows, titleStyle.Render("Jobs"))
	rows = append(rows, "")

	// Header row
	header := fmt.Sprintf("   %-22s  %-10s  %-6s  %s",
		"Job ID", "Status", "Last", "Next Run")
	rows = append(rows, keyStyle.Render(header))
	rows = append(rows, keyStyle.Render(strings.Repeat("─", 70)))

	// Job rows
	for i, job := range m.jobs {
		rows = append(rows, m.renderJobRow(job, i == m.selectedJob))
	}

	content := strings.Join(rows, "\n")
	return jobListStyle.Render(content)
}

// renderJobRow renders a single job row.
func (m Model) renderJobRow(job JobState, selected bool) string {
	// Cursor indicator
	cursor := " "
	if selected {
		cursor = iconArrow
	}

	// Job ID (truncate to fit)
	jobID := padRight(truncate(job.ID, 22), 22)

	// Status icon and text
	var statusIcon string
	var statusText string
	var statusStyle lipgloss.Style

	switch job.Status {
	case JobStatusRunning:
		statusIcon = iconRunning
		statusText = "Running"
		statusStyle = statusRunningStyle
	case JobStatusSuccess:
		statusIcon = iconSuccess
		statusText = "Success"
		statusStyle = statusSuccessStyle
	case JobStatusError:
		statusIcon = iconError
		statusText = "Failed "
		statusStyle = statusErrorStyle
	default:
		statusIcon = iconIdle
		statusText = "Idle   "
		statusStyle = statusIdleStyle
	}

	statusDisplay := statusStyle.Render(fmt.Sprintf("%s %s", statusIcon, statusText))

	// Last run time
	lastRunStr := "-     "
	if job.LastRun != nil && !job.LastRun.EndTime.IsZero() {
		duration := job.LastRun.Duration()
		lastRunStr = padRight(formatDuration(duration), 6)
	}
	lastRunDisplay := durationStyle.Render(lastRunStr)

	// Next run time
	nextRunStr := formatTimeFromNow(job.NextRun)
	nextRunDisplay := keyStyle.Render(nextRunStr)

	// Build row with fixed spacing
	row := fmt.Sprintf("%s  %-22s  %s  %s  %s",
		cursor,
		jobID,
		statusDisplay,
		lastRunDisplay,
		nextRunDisplay,
	)

	if selected {
		return jobItemSelectedStyle.Render(row)
	}
	return jobItemStyle.Render(row)
}

// renderRecentRuns renders the recent runs panel.
func (m Model) renderRecentRuns() string {
	var rows []string

	// Title with count
	titleText := fmt.Sprintf("Recent Runs (%d)", len(m.recentRuns))
	rows = append(rows, titleStyle.Render(titleText))
	rows = append(rows, "")

	if len(m.recentRuns) == 0 {
		rows = append(rows, subtitleStyle.Render("No runs yet"))
	} else {
		// Column headers
		header := fmt.Sprintf("   %-10s  %-22s  %-6s  %s", "Time", "Job", "Status", "Duration")
		rows = append(rows, keyStyle.Render(header))
		rows = append(rows, keyStyle.Render("   "+strings.Repeat("─", 60)))

		// Runs
		for _, run := range m.recentRuns {
			rows = append(rows, m.renderRunItem(run))
		}
	}

	content := strings.Join(rows, "\n")
	return recentRunsStyle.Render(content)
}

// renderRunItem renders a single run item.
func (m Model) renderRunItem(run *store.JobRun) string {
	// Status icon
	var statusIcon string
	var statusStyleFunc lipgloss.Style
	if run.Success {
		statusIcon = iconSuccess
		statusStyleFunc = statusSuccessStyle
	} else {
		statusIcon = iconError
		statusStyleFunc = statusErrorStyle
	}

	// Format time (HH:MM:SS = 8 chars)
	timeStr := run.StartTime.Format("15:04:05")

	// Format job ID
	jobIDStr := padRight(truncate(run.JobID, 22), 22)

	// Format duration
	durationStr := ""
	if !run.EndTime.IsZero() {
		durationStr = formatDuration(run.Duration())
	} else {
		durationStr = "running..."
	}

	// Build row aligned with headers: Time (10), Job (22), Status (6), Duration
	row := fmt.Sprintf("%s  %-10s  %-22s  %s      %s",
		iconBullet,
		timeStr,
		jobIDStr,
		statusStyleFunc.Render(statusIcon),
		durationStyle.Render(durationStr),
	)

	return runItemStyle.Render(row)
}

// renderHelpBar renders the help/status bar at the bottom.
func (m Model) renderHelpBar() string {
	if m.errorMessage != "" {
		return statusBarStyle.Render(statusErrorStyle.Render("Error: " + m.errorMessage))
	}

	help := "q: quit  │  ↑/↓: navigate  │  enter: details  │  r: refresh"
	return statusBarStyle.Render(help)
}

// renderDetailView renders the detailed view for a selected job.
func (m Model) renderDetailView() string {
	if m.selectedJob >= len(m.jobs) {
		return "Invalid job selection"
	}

	job := m.jobs[m.selectedJob]
	var sections []string

	// Header with job name - make it prominent
	jobTitle := fmt.Sprintf("⚡ Jobster Dashboard - %s", job.ID)
	lastUpdate := fmt.Sprintf("Last updated: %s", m.lastUpdate.Format("15:04:05"))
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(jobTitle),
		"  ",
		subtitleStyle.Render(lastUpdate),
	)
	sections = append(sections, headerStyle.Render(header))

	// Job info panel
	var jobInfo []string
	jobInfo = append(jobInfo, titleStyle.Render("Configuration"))
	jobInfo = append(jobInfo, "")

	// Get the full job config from the config
	var jobCommand string
	for _, configJob := range m.config.Jobs {
		if configJob.ID == job.ID {
			jobCommand = configJob.Command.String()
			break
		}
	}

	if jobCommand != "" {
		jobInfo = append(jobInfo, fmt.Sprintf("%s %s", keyStyle.Render("Command:"), valueStyle.Render(truncate(jobCommand, 60))))
	}
	jobInfo = append(jobInfo, fmt.Sprintf("%s %s", keyStyle.Render("Schedule:"), valueStyle.Render(job.Schedule)))

	// Status
	var statusDisplay string
	switch job.Status {
	case JobStatusRunning:
		statusDisplay = statusRunningStyle.Render(iconRunning + " Running")
	case JobStatusSuccess:
		statusDisplay = statusSuccessStyle.Render(iconSuccess + " Success")
	case JobStatusError:
		statusDisplay = statusErrorStyle.Render(iconError + " Failed")
	default:
		statusDisplay = statusIdleStyle.Render(iconIdle + " Idle")
	}
	jobInfo = append(jobInfo, fmt.Sprintf("%s %s", keyStyle.Render("Status:"), statusDisplay))

	// Next run
	nextRunStr := formatTimeFromNow(job.NextRun)
	jobInfo = append(jobInfo, fmt.Sprintf("%s %s", keyStyle.Render("Next Run:"), valueStyle.Render(nextRunStr)))

	// Last run
	if job.LastRun != nil {
		lastRunTime := job.LastRun.StartTime.Format("2006-01-02 15:04:05")
		duration := formatDuration(job.LastRun.Duration())
		jobInfo = append(jobInfo, fmt.Sprintf("%s %s (%s)", keyStyle.Render("Last Run:"), valueStyle.Render(lastRunTime), durationStyle.Render(duration)))
	}

	sections = append(sections, jobListStyle.Render(strings.Join(jobInfo, "\n")))

	// Run history
	var historyInfo []string
	historyInfo = append(historyInfo, titleStyle.Render(fmt.Sprintf("Run History (%d runs)", len(m.detailRuns))))
	historyInfo = append(historyInfo, "")

	if len(m.detailRuns) == 0 {
		historyInfo = append(historyInfo, subtitleStyle.Render("No runs yet"))
	} else {
		// Header
		header := fmt.Sprintf("  %-20s  %-8s  %-12s  %s", "Start Time", "Status", "Duration", "Exit Code")
		historyInfo = append(historyInfo, keyStyle.Render(header))
		historyInfo = append(historyInfo, keyStyle.Render("  "+strings.Repeat("─", 65)))

		// Runs
		for _, run := range m.detailRuns {
			// Status icon
			statusIcon := iconSuccess
			statusStyleFunc := statusSuccessStyle
			if !run.Success {
				statusIcon = iconError
				statusStyleFunc = statusErrorStyle
			}

			// Format fields
			timeStr := run.StartTime.Format("2006-01-02 15:04:05")
			statusDisplay := statusStyleFunc.Render(statusIcon)

			durationStr := formatDuration(run.Duration())
			if run.IsRunning() {
				durationStr = "running..."
			}
			durationDisplay := durationStyle.Render(padRight(durationStr, 12))

			// Build row with proper spacing
			row := fmt.Sprintf("  %-20s  %s        %-12s  %d",
				timeStr,
				statusDisplay,
				durationDisplay,
				run.ExitCode,
			)
			historyInfo = append(historyInfo, row)

			// Show stderr if failed
			if !run.Success && run.StderrTail != "" {
				errorPreview := truncate(strings.TrimSpace(run.StderrTail), 75)
				historyInfo = append(historyInfo, "    "+keyStyle.Render("Error: ")+statusErrorStyle.Render(errorPreview))
			}
		}
	}

	sections = append(sections, detailHistoryStyle.Render(strings.Join(historyInfo, "\n")))

	// Help bar
	helpText := "esc: back  │  q: quit  │  r: refresh"
	sections = append(sections, statusBarStyle.Render(helpText))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Helper functions

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// formatTimeFromNow formats a time relative to now.
func formatTimeFromNow(t time.Time) string {
	duration := time.Until(t)

	if duration < 0 {
		return "now"
	}

	if duration < time.Minute {
		return fmt.Sprintf("in %ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("in %dm", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("in %dh %dm",
			int(duration.Hours()),
			int(duration.Minutes())%60,
		)
	}
	return fmt.Sprintf("in %dd", int(duration.Hours()/24))
}

// truncate truncates a string to a maximum length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// padRight pads a string with spaces to reach the desired length.
func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}
