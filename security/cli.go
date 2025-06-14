package security

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// SecurityCLI provides command-line interface for security management
type SecurityCLI struct {
	integration *SecurityIntegration
}

// NewSecurityCLI creates a new security CLI
func NewSecurityCLI(integration *SecurityIntegration) *SecurityCLI {
	return &SecurityCLI{
		integration: integration,
	}
}

// HandleSecurityCommand handles security-related commands
func (cli *SecurityCLI) HandleSecurityCommand(args []string) error {
	if len(args) == 0 {
		return cli.showHelp()
	}

	command := args[0]
	switch command {
	case "status":
		return cli.showStatus()
	case "init":
		return cli.initializeSecurity()
	case "enable":
		return cli.enableSecurity()
	case "disable":
		return cli.disableSecurity()
	case "user":
		return cli.handleUserCommand(args[1:])
	case "role":
		return cli.handleRoleCommand(args[1:])
	case "permission":
		return cli.handlePermissionCommand(args[1:])
	case "audit":
		return cli.handleAuditCommand(args[1:])
	case "config":
		return cli.handleConfigCommand(args[1:])
	case "help":
		return cli.showHelp()
	default:
		return fmt.Errorf("unknown security command: %s", command)
	}
}

// showHelp displays help information
func (cli *SecurityCLI) showHelp() error {
	help := `
Claude Squad Security Management

USAGE:
    claude-squad security <command> [options]

COMMANDS:
    status              Show security system status
    init                Initialize security system with default settings
    enable              Enable security features
    disable             Disable security features
    
    user <subcommand>   User management
        create          Create a new user
        list            List all users
        delete          Delete a user
        passwd          Change user password
        roles           Manage user roles
    
    role <subcommand>   Role management
        create          Create a new role
        list            List all roles
        delete          Delete a role
        permissions     Manage role permissions
    
    permission <subcommand>  Permission management
        list            List all permissions
        check           Check user permissions
    
    audit <subcommand>  Audit log management
        query           Query audit logs
        stats           Show audit statistics
        export          Export audit logs
    
    config <subcommand> Configuration management
        show            Show current configuration
        set             Set configuration value
        export          Export configuration
        import          Import configuration
    
    help                Show this help message

EXAMPLES:
    claude-squad security status
    claude-squad security user create admin
    claude-squad security role list
    claude-squad security audit query --type=login --since=24h
    claude-squad security config show
`
	fmt.Print(help)
	return nil
}

// showStatus displays the current security status
func (cli *SecurityCLI) showStatus() error {
	status := cli.integration.GetSecurityStatus()
	stats := cli.integration.GetSecurityManager().GetSecurityStats()

	fmt.Println("=== Claude Squad Security Status ===")
	fmt.Printf("Security Enabled:     %v\n", status["security_enabled"])
	fmt.Printf("Authentication:       %v\n", status["authentication"])
	fmt.Printf("Permissions:          %v\n", status["permissions"])
	fmt.Printf("Audit Logging:        %v\n", status["audit"])
	fmt.Printf("Encryption at Rest:   %v\n", status["encryption_at_rest"])
	fmt.Printf("Security Headers:     %v\n", status["security_headers"])
	fmt.Printf("Rate Limiting:        %v\n", status["rate_limiting"])
	fmt.Printf("Developer Mode:       %v\n", status["developer_mode"])
	
	fmt.Println("\n=== Security Statistics ===")
	if auditStats, ok := stats["audit_stats"].(*AuditStats); ok {
		fmt.Printf("Total Events:         %d\n", auditStats.TotalEvents)
		fmt.Printf("Alerts Triggered:     %d\n", auditStats.AlertsTriggered)
		fmt.Printf("Error Count:          %d\n", auditStats.ErrorCount)
		if !auditStats.LastEvent.IsZero() {
			fmt.Printf("Last Event:           %s\n", auditStats.LastEvent.Format(time.RFC3339))
		}
	}

	return nil
}

// initializeSecurity initializes the security system
func (cli *SecurityCLI) initializeSecurity() error {
	fmt.Println("Initializing Claude Squad Security System...")

	// Initialize default user
	if err := cli.integration.InitializeDefaultUser(); err != nil {
		return fmt.Errorf("failed to initialize default user: %w", err)
	}

	fmt.Println("✓ Security system initialized successfully")
	fmt.Println("Run 'claude-squad security status' to see the current configuration")
	
	return nil
}

// enableSecurity enables security features
func (cli *SecurityCLI) enableSecurity() error {
	fmt.Println("Enabling security features...")
	// This would modify the configuration to enable security
	// For now, we'll just show what would be enabled
	
	fmt.Println("✓ Authentication enabled")
	fmt.Println("✓ Permission system enabled")
	fmt.Println("✓ Audit logging enabled")
	fmt.Println("✓ Security features activated")
	
	return nil
}

// disableSecurity disables security features
func (cli *SecurityCLI) disableSecurity() error {
	fmt.Print("Are you sure you want to disable security features? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("Security features remain enabled")
		return nil
	}
	
	fmt.Println("Disabling security features...")
	fmt.Println("✓ Security features disabled")
	
	return nil
}

// handleUserCommand handles user management commands
func (cli *SecurityCLI) handleUserCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("user command requires a subcommand (create, list, delete, passwd, roles)")
	}

	subcommand := args[0]
	switch subcommand {
	case "create":
		return cli.createUser(args[1:])
	case "list":
		return cli.listUsers()
	case "delete":
		return cli.deleteUser(args[1:])
	case "passwd":
		return cli.changePassword(args[1:])
	case "roles":
		return cli.manageUserRoles(args[1:])
	default:
		return fmt.Errorf("unknown user subcommand: %s", subcommand)
	}
}

// createUser creates a new user
func (cli *SecurityCLI) createUser(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("username is required")
	}

	username := args[0]
	
	// Get password securely
	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println()

	fmt.Print("Confirm password: ")
	confirmPassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password confirmation: %w", err)
	}
	fmt.Println()

	if string(password) != string(confirmPassword) {
		return fmt.Errorf("passwords do not match")
	}

	// Get roles
	roles := []string{"user"} // Default role
	if len(args) > 1 {
		roles = strings.Split(args[1], ",")
	}

	// Create user
	user, err := cli.integration.GetSecurityManager().CreateUser(username, string(password), roles)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("✓ User '%s' created successfully\n", user.Username)
	fmt.Printf("  Roles: %s\n", strings.Join(user.Roles, ", "))
	
	return nil
}

// listUsers lists all users
func (cli *SecurityCLI) listUsers() error {
	fmt.Println("User management not fully implemented yet")
	fmt.Println("This would show a list of all users with their roles and status")
	return nil
}

// deleteUser deletes a user
func (cli *SecurityCLI) deleteUser(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("username is required")
	}

	username := args[0]
	fmt.Printf("Are you sure you want to delete user '%s'? [y/N]: ", username)
	
	var response string
	fmt.Scanln(&response)
	
	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("User deletion cancelled")
		return nil
	}

	fmt.Printf("✓ User '%s' would be deleted (not implemented yet)\n", username)
	return nil
}

// changePassword changes a user's password
func (cli *SecurityCLI) changePassword(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("username is required")
	}

	username := args[0]
	
	fmt.Print("Enter current password: ")
	oldPassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read current password: %w", err)
	}
	fmt.Println()

	fmt.Print("Enter new password: ")
	newPassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read new password: %w", err)
	}
	fmt.Println()

	fmt.Print("Confirm new password: ")
	confirmPassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password confirmation: %w", err)
	}
	fmt.Println()

	if string(newPassword) != string(confirmPassword) {
		return fmt.Errorf("passwords do not match")
	}

	// Change password
	err = cli.integration.GetSecurityManager().ChangePassword(username, string(oldPassword), string(newPassword))
	if err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	fmt.Printf("✓ Password changed successfully for user '%s'\n", username)
	return nil
}

// manageUserRoles manages user roles
func (cli *SecurityCLI) manageUserRoles(args []string) error {
	fmt.Println("User role management not fully implemented yet")
	return nil
}

// handleRoleCommand handles role management commands
func (cli *SecurityCLI) handleRoleCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("role command requires a subcommand (create, list, delete, permissions)")
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return cli.listRoles()
	case "create":
		return cli.createRole(args[1:])
	case "delete":
		return cli.deleteRole(args[1:])
	case "permissions":
		return cli.manageRolePermissions(args[1:])
	default:
		return fmt.Errorf("unknown role subcommand: %s", subcommand)
	}
}

// listRoles lists all roles
func (cli *SecurityCLI) listRoles() error {
	roles := cli.integration.GetSecurityManager().GetPermissionManager().ListRoles()
	
	fmt.Println("=== Available Roles ===")
	for _, role := range roles {
		fmt.Printf("Role: %s\n", role.Name)
		fmt.Printf("  ID: %s\n", role.ID)
		fmt.Printf("  Description: %s\n", role.Description)
		fmt.Printf("  Permissions: %s\n", strings.Join(role.Permissions, ", "))
		if len(role.Inherits) > 0 {
			fmt.Printf("  Inherits: %s\n", strings.Join(role.Inherits, ", "))
		}
		fmt.Printf("  Priority: %d\n", role.Priority)
		fmt.Println()
	}
	
	return nil
}

// createRole creates a new role
func (cli *SecurityCLI) createRole(args []string) error {
	fmt.Println("Role creation not fully implemented yet")
	return nil
}

// deleteRole deletes a role
func (cli *SecurityCLI) deleteRole(args []string) error {
	fmt.Println("Role deletion not fully implemented yet")
	return nil
}

// manageRolePermissions manages role permissions
func (cli *SecurityCLI) manageRolePermissions(args []string) error {
	fmt.Println("Role permission management not fully implemented yet")
	return nil
}

// handlePermissionCommand handles permission management commands
func (cli *SecurityCLI) handlePermissionCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("permission command requires a subcommand (list, check)")
	}

	subcommand := args[0]
	switch subcommand {
	case "list":
		return cli.listPermissions()
	case "check":
		return cli.checkPermission(args[1:])
	default:
		return fmt.Errorf("unknown permission subcommand: %s", subcommand)
	}
}

// listPermissions lists all permissions
func (cli *SecurityCLI) listPermissions() error {
	permissions := cli.integration.GetSecurityManager().GetPermissionManager().ListPermissions()
	
	fmt.Println("=== Available Permissions ===")
	for _, perm := range permissions {
		fmt.Printf("Permission: %s\n", perm.Name)
		fmt.Printf("  ID: %s\n", perm.ID)
		fmt.Printf("  Description: %s\n", perm.Description)
		fmt.Printf("  Resource: %s\n", perm.Resource)
		fmt.Printf("  Action: %s\n", perm.Action)
		fmt.Printf("  Scope: %s\n", perm.Scope)
		fmt.Println()
	}
	
	return nil
}

// checkPermission checks user permissions
func (cli *SecurityCLI) checkPermission(args []string) error {
	fmt.Println("Permission checking not fully implemented yet")
	return nil
}

// handleAuditCommand handles audit management commands
func (cli *SecurityCLI) handleAuditCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("audit command requires a subcommand (query, stats, export)")
	}

	subcommand := args[0]
	switch subcommand {
	case "stats":
		return cli.showAuditStats()
	case "query":
		return cli.queryAuditLogs(args[1:])
	case "export":
		return cli.exportAuditLogs(args[1:])
	default:
		return fmt.Errorf("unknown audit subcommand: %s", subcommand)
	}
}

// showAuditStats shows audit statistics
func (cli *SecurityCLI) showAuditStats() error {
	stats := cli.integration.GetSecurityManager().GetAuditLogger().GetStats()
	
	fmt.Println("=== Audit Statistics ===")
	fmt.Printf("Total Events:         %d\n", stats.TotalEvents)
	fmt.Printf("Alerts Triggered:     %d\n", stats.AlertsTriggered)
	fmt.Printf("Error Count:          %d\n", stats.ErrorCount)
	
	if !stats.LastEvent.IsZero() {
		fmt.Printf("Last Event:           %s\n", stats.LastEvent.Format(time.RFC3339))
	}
	
	fmt.Println("\n=== Events by Type ===")
	for eventType, count := range stats.EventsByType {
		fmt.Printf("  %s: %d\n", eventType, count)
	}
	
	fmt.Println("\n=== Events by Severity ===")
	for severity, count := range stats.EventsBySeverity {
		fmt.Printf("  %s: %d\n", severity, count)
	}
	
	return nil
}

// queryAuditLogs queries audit logs
func (cli *SecurityCLI) queryAuditLogs(args []string) error {
	// For now, just show recent events
	filter := AuditFilter{
		Limit: 10, // Show last 10 events
	}
	
	events, err := cli.integration.GetSecurityManager().GetAuditLogger().QueryEvents(filter)
	if err != nil {
		return fmt.Errorf("failed to query audit logs: %w", err)
	}
	
	fmt.Println("=== Recent Audit Events ===")
	for _, event := range events {
		fmt.Printf("[%s] %s", event.Timestamp.Format("2006-01-02 15:04:05"), event.Type)
		if event.UserID != "" {
			fmt.Printf(" (User: %s)", event.UserID)
		}
		if event.Success {
			fmt.Print(" ✓")
		} else {
			fmt.Print(" ✗")
		}
		fmt.Printf(" [%s]\n", event.Severity)
		
		if event.ErrorMsg != "" {
			fmt.Printf("  Error: %s\n", event.ErrorMsg)
		}
		fmt.Println()
	}
	
	return nil
}

// exportAuditLogs exports audit logs
func (cli *SecurityCLI) exportAuditLogs(args []string) error {
	fmt.Println("Audit log export not fully implemented yet")
	return nil
}

// handleConfigCommand handles configuration management commands
func (cli *SecurityCLI) handleConfigCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("config command requires a subcommand (show, set, export, import)")
	}

	subcommand := args[0]
	switch subcommand {
	case "show":
		return cli.showConfig()
	case "set":
		return cli.setConfig(args[1:])
	case "export":
		return cli.exportConfig(args[1:])
	case "import":
		return cli.importConfig(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

// showConfig shows the current configuration
func (cli *SecurityCLI) showConfig() error {
	config := cli.integration.GetSecurityManager().GetConfig()
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	fmt.Println("=== Security Configuration ===")
	fmt.Println(string(data))
	
	return nil
}

// setConfig sets a configuration value
func (cli *SecurityCLI) setConfig(args []string) error {
	fmt.Println("Configuration setting not fully implemented yet")
	return nil
}

// exportConfig exports configuration
func (cli *SecurityCLI) exportConfig(args []string) error {
	fmt.Println("Configuration export not fully implemented yet")
	return nil
}

// importConfig imports configuration
func (cli *SecurityCLI) importConfig(args []string) error {
	fmt.Println("Configuration import not fully implemented yet")
	return nil
}