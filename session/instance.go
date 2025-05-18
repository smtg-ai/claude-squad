package session

import (
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Instance represents a session instance
type Instance struct {
	// Title of the session
	Title string
	// ID must be unique
	ID string
	// CreatedAt is when this instance was created
	CreatedAt time.Time
	// Program is the name of the program to run
	Program string
	// AutoYes is a flag for enabling auto-yes mode for this instance
	AutoYes bool
	// DisableGit is a flag to disable git functionality
	DisableGit bool

	// TmuxSessionName is the name of the tmux session
	TmuxSessionName string
	// TmuxWindow is the window number
	TmuxWindow string
	// TmuxPane is the pane id
	TmuxPane string
	// Content is the last output of the session
	Content string
	// GitWorktree is the git worktree info
	GitWorktree *git.GitWorktree
	// State is additional metadata for this instance
	State string
	// DiffStats is the diff stats for this instance
	DiffStats string
	// Tags are used to categorize instances
	Tags []string
	// CustomMetadata stores additional instance-specific metadata
	CustomMetadata map[string]interface{}
}

// NewInstance creates a new instance
func NewInstance(id, title, program string, autoYes bool) *Instance {
	return &Instance{
		ID:             id,
		Title:          title,
		CreatedAt:      time.Now(),
		Program:        program,
		AutoYes:        autoYes,
		DisableGit:     false,
		TmuxSessionName: "",
		TmuxWindow:     "",
		TmuxPane:       "",
		Content:        "",
		GitWorktree:    nil,
		Tags:           []string{},
		CustomMetadata: make(map[string]interface{}),
	}
}

// Started returns whether this instance has been started
func (i *Instance) Started() bool {
	return i.TmuxSessionName != ""
}

// Paused returns whether this instance has been paused
func (i *Instance) Paused() bool {
	return i.State == "paused"
}

// HasUpdated returns whether this instance has updated content
// If hasPrompt is true, there might be a prompt waiting for input
func (i *Instance) HasUpdated() (updated bool, hasPrompt bool) {
	content, err := tmux.CapturePane(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane)
	if err != nil {
		log.WarningLog.Printf("could not capture pane: %v", err)
		return false, false
	}

	// content from tmux has lots of trailing newlines
	content = strings.TrimRight(content, "\n")

	// Also check if the prompt has appeared and needs input.
	// if the last line is just "> ", it's probably a prompt.
	lines := strings.Split(content, "\n")
	lastLine := ""
	for j := len(lines) - 1; j >= 0; j-- {
		line := strings.TrimSpace(lines[j])
		if line != "" {
			lastLine = line
			break
		}
	}
	if strings.HasSuffix(lastLine, ">") || strings.HasSuffix(lastLine, "> ") {
		hasPrompt = true
	}

	if content != i.Content {
		i.Content = content
		return true, hasPrompt
	}
	return false, hasPrompt
}

// TapEnter sends an enter key to the pane
func (i *Instance) TapEnter() error {
	return tmux.SendKey(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane, "Enter")
}

// UpdateDiffStats updates the diff stats for this instance
func (i *Instance) UpdateDiffStats() error {
	if i.GitWorktree == nil {
		return nil
	}

	diff, err := i.GitWorktree.GetDiffStats()
	if err != nil {
		return err
	}
	i.DiffStats = diff
	return nil
}

// DiffFiles returns the list of files that have been changed
func (i *Instance) DiffFiles() ([]string, error) {
	if i.GitWorktree == nil {
		return nil, nil
	}

	return i.GitWorktree.GetChangedFiles()
}

// GetSubmoduleStatus returns the status of all submodules
func (i *Instance) GetSubmoduleStatus() (map[string]string, error) {
	if i.GitWorktree == nil {
		return nil, nil
	}

	return i.GitWorktree.GetSubmoduleStatus()
}

// Stop stops the instance
func (i *Instance) Stop() error {
	if !i.Started() {
		return nil
	}

	if err := tmux.KillSession(i.TmuxSessionName); err != nil {
		return err
	}

	i.TmuxSessionName = ""
	i.TmuxWindow = ""
	i.TmuxPane = ""
	i.Content = ""

	if i.GitWorktree != nil && !i.DisableGit {
		if err := i.GitWorktree.Cleanup(); err != nil {
			return err
		}
		i.GitWorktree = nil
	}

	return nil
}

// Pause pauses the instance
func (i *Instance) Pause() error {
	if !i.Started() || i.Paused() {
		return nil
	}

	if err := tmux.KillSession(i.TmuxSessionName); err != nil {
		return err
	}

	i.State = "paused"
	return nil
}

// Resume resumes a paused instance
func (i *Instance) Resume() error {
	if !i.Paused() {
		return nil
	}

	// Make sure we have a valid git worktree
	var err error
	if i.GitWorktree == nil && !i.DisableGit {
		// Create git worktree
		worktreePath, err := filepath.Abs(filepath.Join("worktrees", i.ID))
		if err != nil {
			return err
		}
		repoPath, err := git.FindGitRepo(".")
		if err != nil {
			return err
		}
		branchName := "session/" + i.ID

		i.GitWorktree = git.NewGitWorktree(repoPath, worktreePath, branchName)
		if err := i.GitWorktree.Setup(); err != nil {
			return err
		}
	}

	tmuxSession, err := tmux.CreateSession(i.TmuxSessionName, i.Program)
	if err != nil {
		return err
	}

	i.TmuxSessionName = tmuxSession.SessionName
	i.TmuxWindow = tmuxSession.Window
	i.TmuxPane = tmuxSession.Pane
	i.State = ""

	// Change to git worktree directory
	if i.GitWorktree != nil && !i.DisableGit {
		if err := tmux.SendCommand(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane, "cd "+i.GitWorktree.WorktreePath()); err != nil {
			return err
		}
	}

	return nil
}

// AddTag adds a tag to the instance
func (i *Instance) AddTag(tag string) {
	// Check if tag already exists
	for _, t := range i.Tags {
		if t == tag {
			return
		}
	}
	i.Tags = append(i.Tags, tag)
}

// RemoveTag removes a tag from the instance
func (i *Instance) RemoveTag(tag string) {
	newTags := []string{}
	for _, t := range i.Tags {
		if t != tag {
			newTags = append(newTags, t)
		}
	}
	i.Tags = newTags
}

// HasTag checks if the instance has a specific tag
func (i *Instance) HasTag(tag string) bool {
	for _, t := range i.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SetMetadata sets a custom metadata value
func (i *Instance) SetMetadata(key string, value interface{}) {
	if i.CustomMetadata == nil {
		i.CustomMetadata = make(map[string]interface{})
	}
	i.CustomMetadata[key] = value
}

// GetMetadata gets a custom metadata value
func (i *Instance) GetMetadata(key string) (interface{}, bool) {
	if i.CustomMetadata == nil {
		return nil, false
	}
	value, ok := i.CustomMetadata[key]
	return value, ok
}

// GetUpdatedFiles returns a list of files that have been modified in this session
func (i *Instance) GetUpdatedFiles() ([]string, error) {
	if i.GitWorktree == nil {
		return nil, nil
	}
	
	files, err := i.GitWorktree.GetChangedFiles()
	if err != nil {
		return nil, err
	}
	
	return files, nil
}

// WorktreeExists checks if the worktree directory exists
func (i *Instance) WorktreeExists() bool {
	if i.GitWorktree == nil {
		return false
	}
	
	_, err := os.Stat(i.GitWorktree.WorktreePath())
	return !os.IsNotExist(err)
}