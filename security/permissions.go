package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Permission represents a specific permission in the system
type Permission struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Scope       string                 `json:"scope"`
	Conditions  map[string]interface{} `json:"conditions,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Role represents a role that groups multiple permissions
type Role struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Permissions []string               `json:"permissions"` // Permission IDs
	Inherits    []string               `json:"inherits"`    // Role IDs to inherit from
	Priority    int                    `json:"priority"`    // Higher priority roles override lower ones
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PermissionContext represents the context for permission evaluation
type PermissionContext struct {
	UserID      string                 `json:"user_id"`
	UserRoles   []string               `json:"user_roles"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Scope       string                 `json:"scope"`
	Environment map[string]interface{} `json:"environment"`
	Timestamp   time.Time              `json:"timestamp"`
}

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Granted     bool                   `json:"granted"`
	Reason      string                 `json:"reason"`
	Permission  *Permission            `json:"permission,omitempty"`
	Role        *Role                  `json:"role,omitempty"`
	Context     *PermissionContext     `json:"context"`
	Metadata    map[string]interface{} `json:"metadata"`
	EvaluatedAt time.Time              `json:"evaluated_at"`
}

// PermissionManager handles role-based access control (RBAC)
type PermissionManager struct {
	permissions map[string]*Permission
	roles       map[string]*Role
	auditLogger *AuditLogger
	storageDir  string
	config      *PermissionConfig
}

// PermissionConfig represents permission system configuration
type PermissionConfig struct {
	Enabled           bool          `json:"enabled"`
	DefaultRole       string        `json:"default_role"`
	AdminRole         string        `json:"admin_role"`
	GuestRole         string        `json:"guest_role"`
	CacheTimeout      time.Duration `json:"cache_timeout"`
	StrictMode        bool          `json:"strict_mode"`
	AuditAllChecks    bool          `json:"audit_all_checks"`
	InheritanceDepth  int           `json:"inheritance_depth"`
	PermissionTimeout time.Duration `json:"permission_timeout"`
}

// Predefined permissions for claude-squad
var DefaultPermissions = map[string]*Permission{
	"session.create": {
		ID:          "session.create",
		Name:        "Create Session",
		Description: "Create new claude sessions",
		Resource:    "session",
		Action:      "create",
		Scope:       "own",
	},
	"session.read": {
		ID:          "session.read",
		Name:        "Read Session",
		Description: "View session details and output",
		Resource:    "session",
		Action:      "read",
		Scope:       "own",
	},
	"session.update": {
		ID:          "session.update",
		Name:        "Update Session",
		Description: "Modify session configuration",
		Resource:    "session",
		Action:      "update",
		Scope:       "own",
	},
	"session.delete": {
		ID:          "session.delete",
		Name:        "Delete Session",
		Description: "Delete sessions",
		Resource:    "session",
		Action:      "delete",
		Scope:       "own",
	},
	"session.attach": {
		ID:          "session.attach",
		Name:        "Attach to Session",
		Description: "Attach to running sessions",
		Resource:    "session",
		Action:      "attach",
		Scope:       "own",
	},
	"session.prompt": {
		ID:          "session.prompt",
		Name:        "Send Prompts",
		Description: "Send prompts to sessions",
		Resource:    "session",
		Action:      "prompt",
		Scope:       "own",
	},
	"git.read": {
		ID:          "git.read",
		Name:        "Read Git Repository",
		Description: "View git repository status and history",
		Resource:    "git",
		Action:      "read",
		Scope:       "repository",
	},
	"git.write": {
		ID:          "git.write",
		Name:        "Write Git Repository",
		Description: "Commit, push, and modify git repository",
		Resource:    "git",
		Action:      "write",
		Scope:       "repository",
	},
	"git.branch": {
		ID:          "git.branch",
		Name:        "Manage Branches",
		Description: "Create, delete, and switch branches",
		Resource:    "git",
		Action:      "branch",
		Scope:       "repository",
	},
	"config.read": {
		ID:          "config.read",
		Name:        "Read Configuration",
		Description: "View application configuration",
		Resource:    "config",
		Action:      "read",
		Scope:       "application",
	},
	"config.write": {
		ID:          "config.write",
		Name:        "Write Configuration",
		Description: "Modify application configuration",
		Resource:    "config",
		Action:      "write",
		Scope:       "application",
	},
	"system.admin": {
		ID:          "system.admin",
		Name:        "System Administration",
		Description: "Full system administrative access",
		Resource:    "system",
		Action:      "*",
		Scope:       "global",
	},
	"security.manage": {
		ID:          "security.manage",
		Name:        "Manage Security",
		Description: "Manage users, roles, and permissions",
		Resource:    "security",
		Action:      "manage",
		Scope:       "global",
	},
	"audit.read": {
		ID:          "audit.read",
		Name:        "Read Audit Logs",
		Description: "View audit and security logs",
		Resource:    "audit",
		Action:      "read",
		Scope:       "global",
	},
}

// Predefined roles
var DefaultRoles = map[string]*Role{
	"guest": {
		ID:          "guest",
		Name:        "Guest",
		Description: "Limited read-only access",
		Permissions: []string{"session.read", "git.read", "config.read"},
		Priority:    1,
	},
	"user": {
		ID:          "user",
		Name:        "User",
		Description: "Standard user with session management",
		Permissions: []string{
			"session.create", "session.read", "session.update", "session.delete",
			"session.attach", "session.prompt", "git.read", "git.write",
			"git.branch", "config.read",
		},
		Inherits: []string{"guest"},
		Priority: 5,
	},
	"developer": {
		ID:          "developer",
		Name:        "Developer",
		Description: "Advanced user with extended git access",
		Permissions: []string{"git.branch", "config.write"},
		Inherits:    []string{"user"},
		Priority:    7,
	},
	"admin": {
		ID:          "admin",
		Name:        "Administrator",
		Description: "Full system access",
		Permissions: []string{"system.admin", "security.manage", "audit.read"},
		Inherits:    []string{"developer"},
		Priority:    10,
	},
}

// NewPermissionManager creates a new permission manager
func NewPermissionManager(config *PermissionConfig, auditLogger *AuditLogger, storageDir string) (*PermissionManager, error) {
	pm := &PermissionManager{
		permissions: make(map[string]*Permission),
		roles:       make(map[string]*Role),
		auditLogger: auditLogger,
		storageDir:  storageDir,
		config:      config,
	}

	// Load default permissions and roles
	pm.loadDefaults()

	// Load from storage
	if err := pm.loadStorage(); err != nil {
		return nil, fmt.Errorf("failed to load permission storage: %w", err)
	}

	return pm, nil
}

// CheckPermission checks if a user has a specific permission
func (pm *PermissionManager) CheckPermission(context *PermissionContext) *PermissionResult {
	result := &PermissionResult{
		Granted:     false,
		Context:     context,
		EvaluatedAt: time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	if !pm.config.Enabled {
		result.Granted = true
		result.Reason = "Permission system disabled"
		return result
	}

	// Check each role the user has
	for _, roleID := range context.UserRoles {
		if pm.checkRolePermission(roleID, context, result) {
			result.Granted = true
			return result
		}
	}

	result.Reason = "Permission denied: no matching role found"

	// Audit failed permission check if configured
	if pm.config.AuditAllChecks {
		pm.auditLogger.LogEvent(AuditEvent{
			Type:      "permission_denied",
			UserID:    context.UserID,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"resource": context.Resource,
				"action":   context.Action,
				"scope":    context.Scope,
				"reason":   result.Reason,
			},
		})
	}

	return result
}

// checkRolePermission checks if a role grants the requested permission
func (pm *PermissionManager) checkRolePermission(roleID string, context *PermissionContext, result *PermissionResult) bool {
	role, exists := pm.roles[roleID]
	if !exists {
		return false
	}

	// Check direct permissions
	for _, permissionID := range role.Permissions {
		permission, exists := pm.permissions[permissionID]
		if !exists {
			continue
		}

		if pm.matchesPermission(permission, context) {
			result.Permission = permission
			result.Role = role
			result.Reason = fmt.Sprintf("Granted by role '%s' permission '%s'", role.Name, permission.Name)
			return true
		}
	}

	// Check inherited roles (with depth limit)
	return pm.checkInheritedPermissions(role, context, result, 0)
}

// checkInheritedPermissions recursively checks inherited role permissions
func (pm *PermissionManager) checkInheritedPermissions(role *Role, context *PermissionContext, result *PermissionResult, depth int) bool {
	if depth >= pm.config.InheritanceDepth {
		return false
	}

	for _, inheritedRoleID := range role.Inherits {
		if pm.checkRolePermission(inheritedRoleID, context, result) {
			return true
		}
	}

	return false
}

// matchesPermission checks if a permission matches the requested context
func (pm *PermissionManager) matchesPermission(permission *Permission, context *PermissionContext) bool {
	// Check resource match
	if permission.Resource != "*" && permission.Resource != context.Resource {
		return false
	}

	// Check action match
	if permission.Action != "*" && permission.Action != context.Action {
		return false
	}

	// Check scope match
	if permission.Scope != "*" && permission.Scope != context.Scope {
		// Special case for "own" scope - check if user owns the resource
		if permission.Scope == "own" {
			if userID, ok := context.Environment["resource_owner"].(string); ok {
				return userID == context.UserID
			}
			return false
		}
		return false
	}

	// Check additional conditions
	if permission.Conditions != nil {
		return pm.evaluateConditions(permission.Conditions, context)
	}

	return true
}

// evaluateConditions evaluates permission conditions
func (pm *PermissionManager) evaluateConditions(conditions map[string]interface{}, context *PermissionContext) bool {
	for key, expectedValue := range conditions {
		actualValue, exists := context.Environment[key]
		if !exists {
			return false
		}

		// Simple equality check for now
		if actualValue != expectedValue {
			return false
		}
	}
	return true
}

// CreateRole creates a new role
func (pm *PermissionManager) CreateRole(role *Role) error {
	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}

	if _, exists := pm.roles[role.ID]; exists {
		return fmt.Errorf("role already exists: %s", role.ID)
	}

	// Validate permissions exist
	for _, permissionID := range role.Permissions {
		if _, exists := pm.permissions[permissionID]; !exists {
			return fmt.Errorf("permission does not exist: %s", permissionID)
		}
	}

	// Validate inherited roles exist
	for _, roleID := range role.Inherits {
		if _, exists := pm.roles[roleID]; !exists {
			return fmt.Errorf("inherited role does not exist: %s", roleID)
		}
	}

	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	pm.roles[role.ID] = role

	// Save to storage
	if err := pm.saveStorage(); err != nil {
		return fmt.Errorf("failed to save role: %w", err)
	}

	// Log role creation
	pm.auditLogger.LogEvent(AuditEvent{
		Type:      "role_created",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"role_id":     role.ID,
			"role_name":   role.Name,
			"permissions": role.Permissions,
		},
	})

	return nil
}

// CreatePermission creates a new permission
func (pm *PermissionManager) CreatePermission(permission *Permission) error {
	if permission.ID == "" {
		return fmt.Errorf("permission ID is required")
	}

	if _, exists := pm.permissions[permission.ID]; exists {
		return fmt.Errorf("permission already exists: %s", permission.ID)
	}

	permission.CreatedAt = time.Now()
	pm.permissions[permission.ID] = permission

	// Save to storage
	if err := pm.saveStorage(); err != nil {
		return fmt.Errorf("failed to save permission: %w", err)
	}

	// Log permission creation
	pm.auditLogger.LogEvent(AuditEvent{
		Type:      "permission_created",
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"permission_id": permission.ID,
			"resource":      permission.Resource,
			"action":        permission.Action,
			"scope":         permission.Scope,
		},
	})

	return nil
}

// AssignRoleToUser assigns a role to a user (this would integrate with AuthenticationManager)
func (pm *PermissionManager) AssignRoleToUser(userID, roleID string) error {
	if _, exists := pm.roles[roleID]; !exists {
		return fmt.Errorf("role does not exist: %s", roleID)
	}

	// Log role assignment
	pm.auditLogger.LogEvent(AuditEvent{
		Type:      "role_assigned",
		UserID:    userID,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"role_id": roleID,
		},
	})

	return nil
}

// GetUserPermissions returns all effective permissions for a user
func (pm *PermissionManager) GetUserPermissions(userRoles []string) ([]*Permission, error) {
	permissionMap := make(map[string]*Permission)

	for _, roleID := range userRoles {
		role, exists := pm.roles[roleID]
		if !exists {
			continue
		}

		// Get direct permissions
		for _, permissionID := range role.Permissions {
			if permission, exists := pm.permissions[permissionID]; exists {
				permissionMap[permissionID] = permission
			}
		}

		// Get inherited permissions
		pm.collectInheritedPermissions(role, permissionMap, 0)
	}

	permissions := make([]*Permission, 0, len(permissionMap))
	for _, permission := range permissionMap {
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// collectInheritedPermissions recursively collects permissions from inherited roles
func (pm *PermissionManager) collectInheritedPermissions(role *Role, permissionMap map[string]*Permission, depth int) {
	if depth >= pm.config.InheritanceDepth {
		return
	}

	for _, inheritedRoleID := range role.Inherits {
		inheritedRole, exists := pm.roles[inheritedRoleID]
		if !exists {
			continue
		}

		// Add inherited role's permissions
		for _, permissionID := range inheritedRole.Permissions {
			if permission, exists := pm.permissions[permissionID]; exists {
				permissionMap[permissionID] = permission
			}
		}

		// Recursively check inherited roles
		pm.collectInheritedPermissions(inheritedRole, permissionMap, depth+1)
	}
}

// GetRole returns a role by ID
func (pm *PermissionManager) GetRole(roleID string) (*Role, error) {
	role, exists := pm.roles[roleID]
	if !exists {
		return nil, fmt.Errorf("role not found: %s", roleID)
	}
	return role, nil
}

// GetPermission returns a permission by ID
func (pm *PermissionManager) GetPermission(permissionID string) (*Permission, error) {
	permission, exists := pm.permissions[permissionID]
	if !exists {
		return nil, fmt.Errorf("permission not found: %s", permissionID)
	}
	return permission, nil
}

// ListRoles returns all roles
func (pm *PermissionManager) ListRoles() []*Role {
	roles := make([]*Role, 0, len(pm.roles))
	for _, role := range pm.roles {
		roles = append(roles, role)
	}
	return roles
}

// ListPermissions returns all permissions
func (pm *PermissionManager) ListPermissions() []*Permission {
	permissions := make([]*Permission, 0, len(pm.permissions))
	for _, permission := range pm.permissions {
		permissions = append(permissions, permission)
	}
	return permissions
}

// Helper functions

func (pm *PermissionManager) loadDefaults() {
	// Load default permissions
	for id, permission := range DefaultPermissions {
		permission.CreatedAt = time.Now()
		pm.permissions[id] = permission
	}

	// Load default roles
	for id, role := range DefaultRoles {
		role.CreatedAt = time.Now()
		role.UpdatedAt = time.Now()
		pm.roles[id] = role
	}
}

func (pm *PermissionManager) loadStorage() error {
	permissionsPath := filepath.Join(pm.storageDir, "permissions.json")
	rolesPath := filepath.Join(pm.storageDir, "roles.json")

	// Load permissions
	if data, err := os.ReadFile(permissionsPath); err == nil {
		var permissions map[string]*Permission
		if err := json.Unmarshal(data, &permissions); err != nil {
			return fmt.Errorf("failed to parse permissions data: %w", err)
		}
		for id, permission := range permissions {
			pm.permissions[id] = permission
		}
	}

	// Load roles
	if data, err := os.ReadFile(rolesPath); err == nil {
		var roles map[string]*Role
		if err := json.Unmarshal(data, &roles); err != nil {
			return fmt.Errorf("failed to parse roles data: %w", err)
		}
		for id, role := range roles {
			pm.roles[id] = role
		}
	}

	return nil
}

func (pm *PermissionManager) saveStorage() error {
	if err := os.MkdirAll(pm.storageDir, 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Save permissions
	permissionsData, err := json.MarshalIndent(pm.permissions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	permissionsPath := filepath.Join(pm.storageDir, "permissions.json")
	if err := os.WriteFile(permissionsPath, permissionsData, 0600); err != nil {
		return fmt.Errorf("failed to save permissions: %w", err)
	}

	// Save roles
	rolesData, err := json.MarshalIndent(pm.roles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal roles: %w", err)
	}

	rolesPath := filepath.Join(pm.storageDir, "roles.json")
	if err := os.WriteFile(rolesPath, rolesData, 0600); err != nil {
		return fmt.Errorf("failed to save roles: %w", err)
	}

	return nil
}

// Utility functions for permission checking

// HasPermission is a convenience function to check if user has specific permission
func (pm *PermissionManager) HasPermission(userRoles []string, resource, action, scope string, environment map[string]interface{}) bool {
	context := &PermissionContext{
		UserRoles:   userRoles,
		Resource:    resource,
		Action:      action,
		Scope:       scope,
		Environment: environment,
		Timestamp:   time.Now(),
	}

	result := pm.CheckPermission(context)
	return result.Granted
}

// RequirePermission panics if user doesn't have the required permission
func (pm *PermissionManager) RequirePermission(userRoles []string, resource, action, scope string, environment map[string]interface{}) {
	if !pm.HasPermission(userRoles, resource, action, scope, environment) {
		panic(fmt.Sprintf("Permission denied: %s.%s on %s", resource, action, scope))
	}
}

// IsAdmin checks if user has admin role
func (pm *PermissionManager) IsAdmin(userRoles []string) bool {
	for _, role := range userRoles {
		if role == pm.config.AdminRole {
			return true
		}
	}
	return false
}

// CanAccessResource checks if user can access a specific resource
func (pm *PermissionManager) CanAccessResource(userRoles []string, resource string, resourceOwner string, userID string) bool {
	environment := map[string]interface{}{
		"resource_owner": resourceOwner,
	}

	context := &PermissionContext{
		UserID:      userID,
		UserRoles:   userRoles,
		Resource:    resource,
		Action:      "read",
		Scope:       "own",
		Environment: environment,
		Timestamp:   time.Now(),
	}

	result := pm.CheckPermission(context)
	return result.Granted
}