package session

import (
	"chronos/log"
	"chronos/session/git"
	"chronos/session/tmux"
	"fmt"
	"os"
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
	// CustomMetadata stores additional instance-specific metadata (lazy loaded)
	CustomMetadata map[string]interface{}
	// tmuxSession is the managed tmux session
	tmuxSession *tmux.TmuxSession
	// Status represents the current status
	Status Status
	// Branch represents the current branch
	Branch string
	// SystemPrompt is the path to the system prompt file for this squad
	SystemPrompt string
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
		CustomMetadata: nil, // Lazy loaded
		Status:         StatusStopped,
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
	content, err := i.tmuxSession.CapturePaneContent()
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
	return i.tmuxSession.TapEnter()
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
	i.DiffStats = diff.Content
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

	if err := i.tmuxSession.Close(); err != nil {
		return err
	}

	i.TmuxSessionName = ""
	i.TmuxWindow = ""
	i.TmuxPane = ""
	i.Content = ""
	i.Status = StatusStopped

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

	if err := i.tmuxSession.Close(); err != nil {
		return err
	}

	i.State = "paused"
	i.Status = StatusPaused
	return nil
}

// Resume resumes a paused instance
func (i *Instance) Resume() error {
	if !i.Paused() {
		return nil
	}

	// Make sure we have a valid git worktree
	if i.GitWorktree == nil && !i.DisableGit {
		// Create git worktree
		repoPath, err := git.FindGitRepo(".")
		if err != nil {
			return err
		}

		i.GitWorktree, _, err = git.NewGitWorktree(repoPath, i.ID)
		if err != nil {
			return err
		}
		i.Branch = i.GitWorktree.GetBranchName()
	}

	i.tmuxSession = tmux.NewTmuxSession(i.TmuxSessionName, i.Program)
	if err := i.tmuxSession.Start(i.Program, i.GitWorktree.GetWorktreePath()); err != nil {
		return err
	}

	i.TmuxSessionName = i.tmuxSession.Name
	i.State = ""
	i.Status = StatusActive

	// Directory change is handled by tmux session start with workdir parameter

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

// SetMetadata sets a custom metadata value (lazy initialization)
func (i *Instance) SetMetadata(key string, value interface{}) {
	i.ensureMetadataInitialized()
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

// ensureMetadataInitialized lazily initializes the metadata map
func (i *Instance) ensureMetadataInitialized() {
	if i.CustomMetadata == nil {
		i.CustomMetadata = make(map[string]interface{})
	}
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
	
	_, err := os.Stat(i.GitWorktree.GetWorktreePath())
	return !os.IsNotExist(err)
}

// ToInstanceData converts an Instance to InstanceData for serialization
func (i *Instance) ToInstanceData() InstanceData {
	data := InstanceData{
		Title:        i.Title,
		CreatedAt:    i.CreatedAt,
		AutoYes:      i.AutoYes,
		Program:      i.Program,
		SystemPrompt: i.SystemPrompt,
	}

	// Convert GitWorktree if available
	if i.GitWorktree != nil {
		data.Worktree = GitWorktreeData{
			RepoPath:      i.GitWorktree.GetRepoPath(),
			WorktreePath:  i.GitWorktree.GetWorktreePath(),
			BranchName:    i.GitWorktree.GetBranchName(),
			BaseCommitSHA: i.GitWorktree.GetBaseCommitSHA(),
		}
		data.Path = i.GitWorktree.GetWorktreePath()
		data.Branch = i.GitWorktree.GetBranchName()
	}

	// Set status based on state
	switch i.State {
	case "paused":
		data.Status = StatusPaused
	case "":
		if i.Started() {
			data.Status = StatusActive
		} else {
			data.Status = StatusStopped
		}
	default:
		data.Status = StatusStopped
	}

	return data
}

// GetDiffStats returns the diff stats for this instance
func (i *Instance) GetDiffStats() *git.DiffStats {
	if i.GitWorktree == nil {
		return nil
	}
	stats, _ := i.GitWorktree.GetDiffStats()
	return stats
}

// SetPreviewSize sets the preview size for the tmux session
func (i *Instance) SetPreviewSize(width, height int) error {
	if i.tmuxSession == nil {
		return nil
	}
	return i.tmuxSession.SetDetachedSize(width, height)
}


// RepoName returns the repository name if GitWorktree is available
func (i *Instance) RepoName() (string, error) {
	if i.GitWorktree != nil {
		return i.GitWorktree.GetRepoName(), nil
	}
	return "", nil
}

// Kill terminates the instance
func (i *Instance) Kill() error {
	return i.Stop()
}

// Attach attaches to the tmux session (returns a channel that closes when detached)
func (i *Instance) Attach() (chan struct{}, error) {
	if i.tmuxSession == nil {
		return nil, fmt.Errorf("no tmux session available")
	}
	
	return i.tmuxSession.Attach()
}

// Preview returns a preview of the instance content
func (i *Instance) Preview() (string, error) {
	return i.Content, nil
}

// SetStatus sets the status of the instance
func (i *Instance) SetStatus(status Status) {
	i.Status = status
}

// Start starts the instance with the given resume flag
func (i *Instance) Start(resume bool) error {
	if resume {
		return i.Resume()
	} else {
		// Start new instance logic - create git worktree and tmux session
		if i.Started() {
			return fmt.Errorf("instance already started")
		}

		// Create git worktree if not disabled
		if !i.DisableGit {
			repoPath, err := git.FindGitRepo(".")
			if err != nil {
				return fmt.Errorf("failed to find git repo: %w", err)
			}

			i.GitWorktree, _, err = git.NewGitWorktree(repoPath, i.ID)
			if err != nil {
				return fmt.Errorf("failed to create git worktree: %w", err)
			}
			i.Branch = i.GitWorktree.GetBranchName()
		}

		// Create and start tmux session
		i.tmuxSession = tmux.NewTmuxSession(i.ID, i.Program)
		workDir := "."
		if i.GitWorktree != nil {
			workDir = i.GitWorktree.GetWorktreePath()
		}
		
		if err := i.tmuxSession.Start(i.Program, workDir); err != nil {
			return fmt.Errorf("failed to start tmux session: %w", err)
		}

		i.TmuxSessionName = i.tmuxSession.Name
		i.Status = StatusActive
		i.State = ""

		return nil
	}
}

// SetTitle sets the title of the instance
func (i *Instance) SetTitle(title string) error {
	i.Title = title
	return nil
}

// SendPrompt sends a prompt to the instance
func (i *Instance) SendPrompt(prompt string) error {
	if i.tmuxSession == nil {
		return fmt.Errorf("no tmux session available")
	}
	return i.tmuxSession.SendKeys(prompt + "\n")
}

// GetGitWorktree returns the git worktree for this instance
func (i *Instance) GetGitWorktree() (*git.GitWorktree, error) {
	return i.GitWorktree, nil
}

// TmuxAlive returns whether the tmux session is alive
func (i *Instance) TmuxAlive() bool {
	if i.tmuxSession == nil {
		return false
	}
	
	// Check if tmux session actually exists
	if !i.tmuxSession.DoesSessionExist() {
		return false
	}
	
	return i.Started()
}

// LoadSystemPrompt loads a system prompt file for this squad
func (i *Instance) LoadSystemPrompt(promptPath string) error {
	// Check if the prompt file exists
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		return fmt.Errorf("system prompt file not found: %s", promptPath)
	}
	
	// Store the prompt path
	i.SystemPrompt = promptPath
	
	// Add system prompt metadata
	i.ensureMetadataInitialized()
	i.CustomMetadata["system_prompt"] = promptPath
	i.CustomMetadata["prompt_loaded_at"] = time.Now()
	
	log.InfoLog.Printf("Loaded system prompt for squad %s: %s", i.Title, promptPath)
	return nil
}