// Package tui provides a terminal user interface for jobster.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSuccess   = lipgloss.Color("#10B981") // Green
	colorError     = lipgloss.Color("#EF4444") // Red
	colorWarning   = lipgloss.Color("#F59E0B") // Orange
	colorInfo      = lipgloss.Color("#3B82F6") // Blue
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBorder    = lipgloss.Color("#374151") // Dark gray
	colorHighlight = lipgloss.Color("#8B5CF6") // Light purple

	// Base styles
	baseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorBorder).
			Padding(0, 1).
			MarginBottom(1)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1).
			MarginTop(1)

	// Job list styles
	jobListStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			MarginBottom(1)

	jobItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	jobItemSelectedStyle = lipgloss.NewStyle().
				Foreground(colorHighlight).
				Bold(true).
				Padding(0, 1)

	// Status indicator styles
	statusRunningStyle = lipgloss.NewStyle().
				Foreground(colorInfo).
				Bold(true)

	statusSuccessStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	statusIdleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Stats panel style
	statsStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 2).
			MarginBottom(1)

	statsItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Recent runs panel style
	recentRunsStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Height(10)

	// Detail view run history style (no fixed height)
	detailHistoryStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(1, 2)

	runItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Help text style
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Value styles for key-value pairs
	keyStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Bold(true)

	// Duration styles
	durationStyle = lipgloss.NewStyle().
			Foreground(colorInfo)
)

// Status icons
const (
	iconRunning = "⟳"
	iconSuccess = "✓"
	iconError   = "✗"
	iconIdle    = "⏸"
	iconPending = "◌"
	iconArrow   = ">"
	iconBullet  = "•"
)
