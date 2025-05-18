package ui

import (
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/git"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"
)

// SyncProgress represents the progress of a sync operation
type SyncProgress struct {
	Status  string
	Message string
	Percent float64
}

// SyncPane is a UI component that displays synchronization status
type SyncPane struct {
	width      int
	height     int
	progress   progress.Model
	spinner    spinner.Model
	inProgress bool
	results    []git.SyncStatus
}

// NewSyncPane creates a new sync pane
func NewSyncPane() *SyncPane {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithDefaultSpinner(),
	)
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(highlightColor)
	
	return &SyncPane{
		progress:   p,
		spinner:    s,
		inProgress: false,
		results:    []git.SyncStatus{},
	}
}

// SetSize sets the dimensions of the sync pane
func (s *SyncPane) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.progress.Width = width / 2
}

// StartSync displays the sync status for an instance
func (s *SyncPane) StartSync(instances []*session.Instance) {
	s.inProgress = true
	s.results = make([]git.SyncStatus, 0, len(instances))
}

// UpdateSyncProgress updates the sync progress UI
func (s *SyncPane) UpdateSyncProgress(instance *session.Instance, status git.SyncStatus) {
	s.results = append(s.results, status)
}

// FinishSync completes the sync operation
func (s *SyncPane) FinishSync() {
	s.inProgress = false
}

// String renders the sync pane
func (s *SyncPane) String() string {
	if s.width == 0 || s.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Make the title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(highlightColor).
		Render("Synchronizing Git Repositories")
	
	sb.WriteString(title)
	sb.WriteString("\n\n")

	if s.inProgress {
		sb.WriteString(s.spinner.View())
		sb.WriteString(" Syncing repositories...\n\n")
		sb.WriteString(s.progress.View())
		sb.WriteString("\n\n")
	} else if len(s.results) > 0 {
		// Display results
		for i, result := range s.results {
			if i >= s.height-6 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(s.results)-i))
				break
			}

			if result.Success {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ "))
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗ "))
			}

			message := wrap.String(result.Message, s.width-4)
			sb.WriteString(message)
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("No synchronization in progress.")
	}

	return sb.String()
}

// SyncInstances performs a synchronization of all instances
func SyncInstances(instances []*session.Instance, options git.SyncOptions) ([]git.SyncStatus, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances to synchronize")
	}

	results := make([]git.SyncStatus, 0, len(instances))

	for _, instance := range instances {
		if instance.GitWorktree == nil {
			results = append(results, git.SyncStatus{
				Success: false,
				Message: fmt.Sprintf("Instance %s has no Git worktree", instance.Title),
			})
			continue
		}

		// Set a dynamic commit message with timestamp
		options.CommitMessage = fmt.Sprintf("Manual sync at %s", time.Now().Format(time.RFC3339))
		
		log.InfoLog.Printf("Syncing instance %s", instance.Title)
		syncStatus := instance.GitWorktree.Sync(options)
		results = append(results, syncStatus)
	}

	return results, nil
}