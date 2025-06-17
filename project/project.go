package project

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Project represents a project workspace containing multiple instances
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	LastAccessed time.Time `json:"last_accessed"`
	CreatedAt    time.Time `json:"created_at"`
	IsActive     bool      `json:"is_active"`
	Instances    []string  `json:"instances"`
}

// NewProject creates a new project with the given path and name
func NewProject(path, name string) (*Project, error) {
	if path == "" {
		return nil, fmt.Errorf("project path cannot be empty")
	}
	
	// Clean and validate the path
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("project path must be absolute: %s", path)
	}
	
	// Generate project name from path if not provided
	if name == "" {
		name = filepath.Base(cleanPath)
		if name == "." || name == "/" {
			return nil, fmt.Errorf("could not determine project name from path: %s", path)
		}
	}
	
	// Generate unique ID from path
	id := generateProjectID(cleanPath)
	
	now := time.Now()
	return &Project{
		ID:           id,
		Name:         name,
		Path:         cleanPath,
		LastAccessed: now,
		CreatedAt:    now,
		IsActive:     false,
		Instances:    make([]string, 0),
	}, nil
}

// generateProjectID creates a unique identifier from the project path
func generateProjectID(path string) string {
	// Use the cleaned path and replace separators with underscores
	id := strings.ReplaceAll(path, string(filepath.Separator), "_")
	// Remove leading underscore if present
	if strings.HasPrefix(id, "_") {
		id = id[1:]
	}
	return id
}

// AddInstance adds an instance ID to the project's instance list
func (p *Project) AddInstance(instanceID string) {
	if instanceID == "" {
		return
	}
	
	// Check if instance already exists
	for _, existing := range p.Instances {
		if existing == instanceID {
			return
		}
	}
	
	p.Instances = append(p.Instances, instanceID)
	p.LastAccessed = time.Now()
}

// RemoveInstance removes an instance ID from the project's instance list
func (p *Project) RemoveInstance(instanceID string) bool {
	for i, existing := range p.Instances {
		if existing == instanceID {
			// Remove from slice
			p.Instances = append(p.Instances[:i], p.Instances[i+1:]...)
			p.LastAccessed = time.Now()
			return true
		}
	}
	return false
}

// HasInstance checks if the project contains the given instance ID
func (p *Project) HasInstance(instanceID string) bool {
	for _, existing := range p.Instances {
		if existing == instanceID {
			return true
		}
	}
	return false
}

// InstanceCount returns the number of instances in this project
func (p *Project) InstanceCount() int {
	return len(p.Instances)
}

// SetActive marks this project as active and updates last accessed time
func (p *Project) SetActive() {
	p.IsActive = true
	p.LastAccessed = time.Now()
}

// SetInactive marks this project as inactive
func (p *Project) SetInactive() {
	p.IsActive = false
}

// Validate ensures the project data is valid
func (p *Project) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}
	if p.Name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if p.Path == "" {
		return fmt.Errorf("project path cannot be empty")
	}
	if !filepath.IsAbs(p.Path) {
		return fmt.Errorf("project path must be absolute: %s", p.Path)
	}
	return nil
}