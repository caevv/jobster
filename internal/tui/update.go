package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Refresh data from store
		m.refreshData()
		// Schedule next tick
		return m, tickCmd()

	case error:
		m.errorMessage = msg.Error()
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		// Go back to list view if in detail view
		if m.viewMode == ViewModeDetail {
			m.viewMode = ViewModeList
			m.detailRuns = nil
		}
		return m, nil

	case "enter":
		// Show detail view for selected job
		if m.viewMode == ViewModeList && len(m.jobs) > 0 {
			m.viewMode = ViewModeDetail
			// Load runs for the selected job
			if m.selectedJob < len(m.jobs) {
				jobID := m.jobs[m.selectedJob].ID
				runs, err := m.store.GetJobRuns(jobID, 5) // Get last 5 runs (fits on screen with config and errors)
				if err == nil {
					m.detailRuns = runs
				}
			}
		}
		return m, nil

	case "up", "k":
		if m.viewMode == ViewModeList && m.selectedJob > 0 {
			m.selectedJob--
		}
		return m, nil

	case "down", "j":
		if m.viewMode == ViewModeList && m.selectedJob < len(m.jobs)-1 {
			m.selectedJob++
		}
		return m, nil

	case "g":
		// Go to top
		if m.viewMode == ViewModeList {
			m.selectedJob = 0
		}
		return m, nil

	case "G":
		// Go to bottom
		if m.viewMode == ViewModeList && len(m.jobs) > 0 {
			m.selectedJob = len(m.jobs) - 1
		}
		return m, nil

	case "r":
		// Manual refresh
		m.refreshData()
		// Reload detail runs if in detail view
		if m.viewMode == ViewModeDetail && m.selectedJob < len(m.jobs) {
			jobID := m.jobs[m.selectedJob].ID
			runs, err := m.store.GetJobRuns(jobID, 5)
			if err == nil {
				m.detailRuns = runs
			}
		}
		return m, nil

	case "?", "h":
		// Toggle help (TODO: implement help view)
		return m, nil
	}

	return m, nil
}
