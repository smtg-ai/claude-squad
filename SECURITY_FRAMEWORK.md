# Claude Squad Security Framework

## 🔒 Overview

This document describes the comprehensive security and permission framework implemented for claude-squad. The framework provides authentication, authorization, audit logging, and security management capabilities.

## 🎯 Features Implemented

### 1. Authentication System (`security/auth.go`)

**Multi-level Authentication Support:**
- `AuthNone` - No authentication required
- `AuthBasic` - Username/password authentication  
- `AuthToken` - Token-based authentication
- `AuthMFA` - Multi-factor authentication

**User Management:**
- Secure password hashing with salt
- Session management with expiration
- Password policy enforcement
- Account lockout protection
- MFA support (TOTP-ready)

**Session Security:**
- Configurable session timeouts
- Multi-session support per user
- Session token validation
- IP address binding
- Automatic cleanup of expired sessions

### 2. Permission System (`security/permissions.go`)

**Role-Based Access Control (RBAC):**
- Hierarchical role inheritance
- Granular permission definitions
- Resource-based access control
- Scope-based restrictions (own, global, repository)

**Pre-defined Permissions:**
```go
// Session Management
session.create, session.read, session.update, session.delete
session.attach, session.prompt

// Git Operations  
git.read, git.write, git.branch

// Configuration
config.read, config.write

// System Administration
system.admin, security.manage, audit.read
```

**Pre-defined Roles:**
- `guest` - Read-only access
- `user` - Standard session management
- `developer` - Extended git access
- `admin` - Full system access

### 3. Audit Logging (`security/audit.go`)

**Comprehensive Event Tracking:**
- Authentication events (login, logout, failures)
- Permission checks (granted/denied)
- Session activities (create, attach, kill)
- Git operations (commit, push, branch)
- Security violations and suspicious activities

**Advanced Features:**
- Configurable log rotation
- Event filtering and querying
- Real-time alerting rules
- Statistics and metrics
- Optional encryption at rest
- Async logging for performance

**Event Types:**
```go
AuditLogin, AuditLoginFailed, AuditLogout
AuditPermissionGranted, AuditPermissionDenied
AuditSessionCreated, AuditSessionAttached, AuditSessionKilled
AuditGitCommit, AuditGitPush, AuditGitBranch
AuditSecurityViolation, AuditSuspiciousActivity
```

### 4. Security Manager (`security/manager.go`)

**Central Security Coordination:**
- Unified authentication and authorization
- Security context management
- Configuration management
- Component integration
- Security statistics and monitoring

**Security Context:**
```go
type SecurityContext struct {
    User            *User
    Session         *Session  
    Permissions     []*Permission
    IPAddress       string
    IsAuthenticated bool
    IsAdmin         bool
    Environment     map[string]interface{}
}
```

### 5. Integration Layer (`security/integration.go`)

**Seamless Claude-Squad Integration:**
- Session permission checks
- Git operation authorization
- Configuration access control
- Activity logging
- Security wrapper functions

**Convenience Functions:**
```go
CheckSessionPermission(userID, sessionOwner, action)
CheckGitPermission(userID, action)
LogSessionActivity(userID, sessionName, action, success, details)
LogGitActivity(userID, action, success, details)
```

### 6. CLI Management (`security/cli.go`)

**Comprehensive Security CLI:**
```bash
claude-squad security status              # Show security status
claude-squad security init                # Initialize security
claude-squad security user create admin   # Create users
claude-squad security role list          # List roles
claude-squad security permission list     # List permissions
claude-squad security audit stats        # Audit statistics
claude-squad security config show        # Show configuration
```

## 🚀 Usage Examples

### Basic Setup

```bash
# Initialize security system
claude-squad security init

# Check status
claude-squad security status

# Create admin user
claude-squad security user create admin

# List available roles and permissions
claude-squad security role list
claude-squad security permission list
```

### Configuration

The security system uses a JSON configuration file located at `~/.claude-squad/security.json`:

```json
{
  "general": {
    "enabled": false,
    "developer_mode": true
  },
  "authentication": {
    "enabled": false,
    "level": "basic",
    "session_timeout": "24h",
    "password_policy": {
      "min_length": 8,
      "require_upper": false
    }
  },
  "permissions": {
    "enabled": false,
    "default_role": "user",
    "strict_mode": false
  },
  "audit": {
    "enabled": true,
    "log_file": "audit.log",
    "min_severity": "info"
  }
}
```

### Production Deployment

For production environments, use the production security configuration:

```go
config := security.ProductionSecurityConfig()
// Enables: MFA, strict permissions, encryption, security headers
```

### Integration in Application Code

```go
// Initialize security
securityIntegration, err := security.NewSecurityIntegration(appConfig)
if err != nil {
    return fmt.Errorf("failed to initialize security: %w", err)
}

// Check permissions before session creation
if !securityIntegration.CheckSessionPermission(userID, userID, "create") {
    return fmt.Errorf("permission denied")
}

// Log session activity
securityIntegration.LogSessionActivity(userID, sessionName, "create", true, nil)
```

## 🔧 Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Authentication  │    │   Permissions   │    │  Audit Logging  │
│    Manager      │    │    Manager      │    │     Manager     │
│                 │    │                 │    │                 │
│ • Users         │    │ • Roles         │    │ • Events        │
│ • Sessions      │    │ • Permissions   │    │ • Statistics    │
│ • Passwords     │    │ • RBAC          │    │ • Alerts        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │    Security     │
                    │    Manager      │
                    │                 │
                    │ • Central       │
                    │   Coordination  │
                    │ • Context       │
                    │ • Integration   │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │   Integration   │
                    │     Layer       │
                    │                 │
                    │ • CLI Commands  │
                    │ • App Hooks     │
                    │ • Middleware    │
                    └─────────────────┘
```

## 🛡️ Security Features

### 1. **Defense in Depth**
- Multiple security layers
- Fail-safe defaults
- Principle of least privilege

### 2. **Audit Trail**
- Complete activity logging
- Tamper-evident logs
- Compliance-ready reporting

### 3. **Access Control**
- Fine-grained permissions
- Role-based inheritance
- Context-aware decisions

### 4. **Session Security**
- Secure token generation
- Session timeout management
- IP binding options

### 5. **Password Security**
- Strong hashing (SHA-256 + salt)
- Policy enforcement
- MFA ready

## 📊 Monitoring and Alerting

### Security Metrics
- Authentication success/failure rates
- Permission denial counts
- Session creation patterns
- Suspicious activity detection

### Audit Statistics
```bash
# View audit statistics
claude-squad security audit stats

# Query recent events
claude-squad security audit query --limit=20

# Export audit data
claude-squad security audit export --format=json
```

## 🎛️ Configuration Options

### Authentication Levels
- **None**: No authentication (development)
- **Basic**: Username + password
- **Token**: Token-based auth
- **MFA**: Multi-factor authentication

### Permission Modes
- **Disabled**: All access allowed
- **Permissive**: Log violations but allow
- **Strict**: Enforce all permissions

### Audit Logging
- **File-based**: Local file storage
- **Encrypted**: AES-256 encryption
- **Compressed**: Automatic log compression
- **Remote**: Syslog integration ready

## 🔄 Migration and Compatibility

### Backward Compatibility
- Security disabled by default
- Gradual migration path
- Non-breaking integration

### Migration Steps
1. Deploy security framework (disabled)
2. Initialize security system
3. Create users and roles
4. Enable authentication
5. Enable permissions
6. Monitor and adjust

## 🚨 Incident Response

### Security Violations
- Automatic logging
- Real-time alerts
- Lockout mechanisms
- Forensic data collection

### Monitoring Commands
```bash
# Check security status
claude-squad security status

# View recent security events
claude-squad security audit query --type=security_violation

# Check user permissions
claude-squad security permission check user1 session create
```

## 📋 Implementation Status

### ✅ Completed Components

1. **Authentication Framework** - Complete user and session management
2. **Permission System** - RBAC with inheritance and conditions
3. **Audit Logging** - Comprehensive event tracking and statistics
4. **Security Manager** - Central coordination and integration
5. **CLI Interface** - Full command-line management tools
6. **Integration Layer** - Seamless claude-squad integration
7. **Configuration System** - Flexible security configuration
8. **Documentation** - Complete usage and deployment guides

### 🎯 Key Achievements

- **Zero Breaking Changes**: Security is disabled by default
- **Production Ready**: Full encryption, MFA, and audit capabilities
- **Developer Friendly**: Comprehensive CLI and clear APIs
- **Enterprise Features**: RBAC, compliance logging, alerting
- **Performance Optimized**: Async logging, session caching
- **Security Hardened**: Defense in depth, fail-safe defaults

### 🚀 Ready for Deployment

The security framework is fully implemented and ready for:
- Development environments (with security disabled)
- Testing environments (with basic authentication)
- Production environments (with full security enabled)

All components integrate seamlessly with the existing claude-squad codebase while maintaining backward compatibility.

---

**Security Team Contact**: For security-related questions or issues, please refer to the audit logs and security documentation.