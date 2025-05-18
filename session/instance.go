package session

import (
	"claude-squad/log"
	"claude-squad/session/git"
	"claude-squad/session/tmux"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status represents the current state of an instance
type Status string

const (
	// Running indicates the instance is actively running and processing
	Running Status = "running"
	// Ready indicates the instance is waiting for input
	Ready Status = "ready"
	// Paused indicates the instance is temporarily stopped
	Paused Status = "paused"
	// Waiting indicates the instance is waiting for something
	Waiting Status = "waiting"
)

// InstanceOptions contains options for creating a new instance
type InstanceOptions struct {
	// Title of the session
	Title string
	// Path to the working directory
	Path string
	// Program is the name of the program to run
	Program string
	// DisableGit disables git functionality
	DisableGit bool
	// AutoYes enables auto-yes mode
	AutoYes bool
}

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
	// Status represents the current state of the instance
	Status Status
	// Branch is the git branch name
	Branch string
}

// NewInstance creates a new instance with the provided options
func NewInstance(opts InstanceOptions) (*Instance, error) {
	// Generate a unique ID based on the title and timestamp
	timestamp := time.Now().UnixNano()
	id := fmt.Sprintf("%s_%d", strings.ReplaceAll(opts.Title, " ", "_"), timestamp)
	
	instance := &Instance{
		ID:             id,
		Title:          opts.Title,
		CreatedAt:      time.Now(),
		Program:        opts.Program,
		AutoYes:        opts.AutoYes,
		DisableGit:     opts.DisableGit,
		TmuxSessionName: "",
		TmuxWindow:     "",
		TmuxPane:       "",
		Content:        "",
		GitWorktree:    nil,
		Tags:           []string{},
		CustomMetadata: make(map[string]interface{}),
		Status:         Waiting,
		Branch:         "session/" + id,
	}
	
	return instance, nil
}

// Start starts the instance
func (i *Instance) Start(createWorktree bool) error {
	if i.Started() {
		return nil
	}
	
	// If git is enabled, create a worktree
	if !i.DisableGit && createWorktree {
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
		i.Branch = branchName

		i.GitWorktree = git.NewGitWorktree(repoPath, worktreePath, branchName)
		if err := i.GitWorktree.Setup(); err != nil {
			return err
		}
	}
	
	// Create tmux session
	tmuxSession, err := tmux.CreateSession("", i.Program)
	if err != nil {
		return err
	}
	
	i.TmuxSessionName = tmuxSession.SessionName
	i.TmuxWindow = tmuxSession.Window
	i.TmuxPane = tmuxSession.Pane
	i.Status = Running
	
	// Change to git worktree directory if available
	if i.GitWorktree != nil && !i.DisableGit {
		if err := tmux.SendCommand(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane, "cd "+i.GitWorktree.WorktreePath()); err != nil {
			return err
		}
	}
	
	return nil
}

// SetTitle sets the title of the instance
func (i *Instance) SetTitle(title string) error {
	i.Title = title
	return nil
}

// SetPreviewSize sets the preview size for the tmux session
func (i *Instance) SetPreviewSize(width, height int) error {
	return tmux.SetPreviewSize(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane, width, height)
}

// Preview returns the current content of the tmux pane
func (i *Instance) Preview() (string, error) {
	return i.Content, nil
}

// RepoName returns the name of the repository
func (i *Instance) RepoName() (string, error) {
	if i.GitWorktree == nil {
		return "", fmt.Errorf("git worktree not initialized")
	}
	return i.GitWorktree.RepoName()
}

// GetGitWorktree returns the git worktree
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	if i.GitWorktree == nil {
		return nil, fmt.Errorf("git worktree not initialized")
	}
	return i.GitWorktree, nil
}

// Kill stops the instance and cleans up resources
func (i *Instance) Kill() error {
	return i.Stop()
}

// SendPrompt sends a prompt to the tmux pane
func (i *Instance) SendPrompt(prompt string) error {
	if !i.Started() {
		return fmt.Errorf("instance not started")
	}
	return tmux.SendCommand(i.TmuxSessionName, i.TmuxWindow, i.TmuxPane, prompt)
}

// TmuxAlive checks if the tmux session is still alive
func (i *Instance) TmuxAlive() bool {
	return tmux.IsSessionAlive(i.TmuxSessionName)
}

// GetDiffStats returns the diff stats for this instance
func (i *Instance) GetDiffStats() *git.DiffStats {
	if i.GitWorktree == nil || i.DiffStats == "" {
		return nil
	}
	
	return i.GitWorktree.ParseDiffStats(i.DiffStats)
}

// Started returns whether this instance has been started
func (i *Instance) Started() bool {
	return i.TmuxSessionName != ""
}

// Paused returns whether this instance has been paused
func (i *Instance) Paused() bool {
	return i.State == "paused" || i.Status == Paused
}

// SetStatus sets the instance status
func (i *Instance) SetStatus(status Status) {
	i.Status = status
	if status == Paused {
		i.State = "paused"
	}
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
	i.Status = Waiting

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
	i.Status = Paused
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
	i.Status = Running

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

// ToInstanceData converts an Instance to InstanceData for serialization
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:     i.Title,
		Path:      ".",
		Branch:    i.Branch,
		Status:    i.Status,
		CreatedAt: i.CreatedAt,
		UpdatedAt: time.Now(),
		AutoYes:   i.AutoYes,
		Program:   i.Program,
	}

	// Include GitWorktree data if available
	if i.GitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      i.GitWorktree.RepoPath(),
			WorktreePath:  i.GitWorktree.WorktreePath(),
			BranchName:    i.GitWorktree.BranchName(),
			BaseCommitSHA: i.GitWorktree.BaseCommitSHA(),
		}
	}

	// Include DiffStats data if available
	if stats := i.GetDiffStats(); stats != nil {
		data.DiffStats = DiffStatsData{
			Added:   stats.Added,
			Removed: stats.Removed,
			Content: i.DiffStats,
		}
	}

	return data
}

// FromInstanceData creates an Instance from InstanceData
func FromInstanceData(data InstanceData) (*Instance, error) {
	instance := &Instance{
		ID:        data.Title, // Use title as ID for restored instances
		Title:     data.Title,
		Program:   data.Program,
		CreatedAt: data.CreatedAt,
		AutoYes:   data.AutoYes,
		Status:    data.Status,
		Branch:    data.Branch,
	}

	// Create GitWorktree if worktree data is available
	if data.Worktree.BranchName != "" {
		instance.GitWorktree = git.NewGitWorktree(
			data.Worktree.RepoPath,
			data.Worktree.WorktreePath,
			data.Worktree.BranchName,
		)
		instance.GitWorktree.SetBaseCommitSHA(data.Worktree.BaseCommitSHA)
	}

	// Set DiffStats if available
	if data.DiffStats.Content != "" {
		instance.DiffStats = data.DiffStats.Content
	}

	// Mark as paused since we're restoring
	instance.Status = Paused
	instance.State = "paused"

	return instance, nil
}