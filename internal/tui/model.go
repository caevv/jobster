package tui

import (
	"log/slog"
	"time"

	"github.com/caevv/jobster/internal/config"
	"github.com/caevv/jobster/internal/scheduler"
	"github.com/caevv/jobster/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode represents the current view in the TUI.
type ViewMode int

const (
	ViewModeList ViewMode = iota
	ViewModeDetail
)

// Model holds the state for the TUI.
type Model struct {
	// Configuration and services
	config    *config.Config
	store     store.Store
	scheduler *scheduler.Scheduler
	logger    *slog.Logger

	// UI state
	viewMode     ViewMode
	jobs         []JobState
	recentRuns   []*store.JobRun
	selectedJob  int
	detailRuns   []*store.JobRun // runs for the selected job in detail view
	width        int
	height       int
	lastUpdate   time.Time
	quitting     bool
	errorMessage string

	// Stats
	totalJobs   int
	runningJobs int
	totalRuns   int
	successRuns int
	failedRuns  int
}

// JobState represents the current state of a job in the UI.
type JobState struct {
	ID         string
	Schedule   string
	Status     JobStatus
	NextRun    time.Time
	LastRun    *store.JobRun
	IsSelected bool
}

// JobStatus represents the execution status of a job.
type JobStatus int

const (
	JobStatusIdle JobStatus = iota
	JobStatusRunning
	JobStatusSuccess
	JobStatusError
)

// New creates a new TUI model.
func New(cfg *config.Config, st store.Store, sched *scheduler.Scheduler, logger *slog.Logger) Model {
	return Model{
		config:     cfg,
		store:      st,
		scheduler:  sched,
		logger:     logger,
		jobs:       []JobState{},
		recentRuns: []*store.JobRun{},
		lastUpdate: time.Now(),
	}
}

// Init initializes the model (required by Bubbletea).
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

// tickMsg is sent on a regular interval to refresh the UI.
type tickMsg time.Time

// tickCmd returns a command that sends a tick message every second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshData loads the latest data from the store and scheduler.
func (m *Model) refreshData() {
	// Update job states
	m.totalJobs = len(m.config.Jobs)
	m.runningJobs = 0
	m.jobs = make([]JobState, len(m.config.Jobs))

	for i, job := range m.config.Jobs {
		// Get last run for this job
		lastRuns, err := m.store.GetJobRuns(job.ID, 1)
		var lastRun *store.JobRun
		if err == nil && len(lastRuns) > 0 {
			lastRun = lastRuns[0]
		}

		// Determine job status
		status := JobStatusIdle
		if lastRun != nil {
			if lastRun.IsRunning() {
				status = JobStatusRunning
				m.runningJobs++
			} else if lastRun.Success {
				status = JobStatusSuccess
			} else {
				status = JobStatusError
			}
		}

		// Get next run time from scheduler
		nextRun := time.Now().Add(time.Hour) // default fallback
		if stats, ok := m.scheduler.GetJobStats(job.ID); ok {
			nextRun = stats.NextRun
		}

		m.jobs[i] = JobState{
			ID:         job.ID,
			Schedule:   job.Schedule,
			Status:     status,
			NextRun:    nextRun,
			LastRun:    lastRun,
			IsSelected: i == m.selectedJob,
		}
	}

	// Get recent runs across all jobs
	recentRuns, err := m.store.GetAllRuns(10)
	if err == nil {
		m.recentRuns = recentRuns
		m.totalRuns = len(recentRuns)
		m.successRuns = 0
		m.failedRuns = 0
		for _, run := range recentRuns {
			if run.Success {
				m.successRuns++
			} else {
				m.failedRuns++
			}
		}
	}

	m.lastUpdate = time.Now()
}

// Quitting returns true if the user has requested to quit.
func (m Model) Quitting() bool {
	return m.quitting
}
