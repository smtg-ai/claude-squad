# Claude Squad Monitoring and Usage Tracking Framework

## 🎯 Overview

This document describes the comprehensive monitoring and usage tracking framework implemented for claude-squad. The framework provides real-time monitoring, performance metrics, usage analytics, dashboard visualization, and automated reporting capabilities.

## 🚀 Features Implemented

### 1. Usage Tracking System (`monitoring/tracker.go`)

**Comprehensive Event Tracking:**

- Session lifecycle events (create, attach, detach, kill)
- Command execution tracking with timing
- Git operations monitoring (commit, push, pull, branch)
- Performance metrics collection
- Error tracking and categorization
- System events and state changes

**Advanced Features:**

- Async logging with configurable buffering
- Event filtering and querying
- Automatic log rotation and retention
- Configurable flush intervals
- Session-specific tracking with context
- Real-time statistics generation

**Event Types:**

```go
EventSessionCreated, EventSessionAttached, EventSessionKilled
EventCommandExecuted, EventPromptSent, EventResponseReceived
EventGitCommit, EventGitPush, EventGitPull, EventGitBranch
EventSystemStart, EventSystemStop, EventError, EventPerformance
```

### 2. Metrics Collection System (`monitoring/metrics.go`)

**System Metrics:**

- Memory usage (allocated, heap, stack, system)
- CPU utilization and load averages
- Goroutine tracking and analysis
- Garbage collection metrics
- Network and disk I/O (extensible)
- Response time percentiles (P95, P99)

**Application Metrics:**

- Active and total session counts
- Command execution statistics
- Git operation tracking
- Error rates and success rates
- Security event monitoring
- Configuration change tracking

**Health Monitoring:**

- Component health status
- Overall system health scoring
- Performance trending analysis
- Automated alert generation
- Predictive analytics
- Recommendation engine

**Alert System:**

- Configurable thresholds for all metrics
- Real-time alert triggering
- Multi-level severity (info, warning, critical)
- Alert acknowledgment and management
- Rate limiting and deduplication
- Multiple notification channels

### 3. Integration Layer (`monitoring/integration.go`)

**Unified Monitoring Interface:**

- Seamless integration with claude-squad
- Context-aware session tracking
- Performance timing utilities
- Automatic event correlation
- Real-time data aggregation
- Configuration management

**Session Context Tracking:**

```go
SessionContext {
    ID, Name, Program, StartTime, LastActivity
    CommandCount, Repository, Branch, UserID
    Performance metrics and timing data
}
```

**Performance Timer:**

- Operation timing with unique IDs
- Automatic duration calculation
- Context correlation
- Performance bottleneck identification

### 4. Web Dashboard (`monitoring/dashboard.go`)

**Interactive Dashboard Features:**

- Real-time metrics visualization
- Session monitoring and management
- Performance graphs and charts
- System health overview
- Alert management interface
- Historical data analysis

**Widget System:**

- Overview widget (high-level KPIs)
- Session widget (active sessions, history)
- Performance widget (CPU, memory, GC)
- Usage widget (patterns, trends)
- System widget (environment info)
- Git widget (repository activity)
- Error widget (error analysis)
- Health widget (system status)
- Trend widget (growth analysis)
- Alert widget (notifications)

**Dashboard Configuration:**

```go
DashboardConfig {
    Port, Host, RefreshInterval
    EnableAuth, Theme, Layout
    EnableRealtime, MaxDataPoints
    CustomWidgets support
}
```

**RESTful API Endpoints:**

- `/api/dashboard` - Complete dashboard data
- `/api/stats` - Detailed statistics
- `/api/health` - Health status
- `/api/sessions` - Session information
- `/api/metrics` - System metrics

### 5. Report Generation (`monitoring/reporting.go`)

**Automated Report Generation:**

- Daily, weekly, monthly reports
- Configurable report templates
- Multiple output formats (JSON, CSV, HTML, Markdown)
- Executive summary generation
- Trend analysis and insights
- Actionable recommendations

**Report Types:**

- **Daily Summary:** Quick overview of system activity
- **Weekly Detailed:** Comprehensive analysis with trends
- **Monthly Executive:** Strategic insights and recommendations
- **Custom Reports:** User-defined templates and filters

**Report Sections:**

- Executive summary with KPIs
- Session analysis and patterns
- Performance metrics and trends
- Usage analytics and insights
- Git operations summary
- Error analysis and resolution
- System health assessment
- Growth predictions and recommendations

**Intelligent Insights:**

- Automatic anomaly detection
- Performance bottleneck identification
- Usage pattern analysis
- Predictive maintenance alerts
- Optimization recommendations
- Security event correlation

### 6. Command-Line Interface (`monitoring/cli.go`)

**Comprehensive CLI Commands:**

```bash
claude-squad monitoring status              # System status overview
claude-squad monitoring stats --format=json # Detailed statistics
claude-squad monitoring sessions list       # Active sessions
claude-squad monitoring performance         # Performance metrics
claude-squad monitoring dashboard start     # Web dashboard
claude-squad monitoring reports generate    # Generate reports
claude-squad monitoring export stats json   # Data export
claude-squad monitoring alerts list         # Alert management
claude-squad monitoring config show         # Configuration
```

**Advanced Features:**

- Interactive status displays with icons
- Multiple output formats (table, JSON, CSV)
- Real-time data streaming
- Filtering and time range selection
- Export capabilities
- Configuration management
- Alert acknowledgment and management

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Usage Tracker  │    │ Metrics Collector│    │ Report Generator│
│                 │    │                 │    │                 │
│ • Event Logging │    │ • System Metrics│    │ • Template Engine│
│ • Session Track │    │ • Health Monitor│    │ • Multi-format  │
│ • Performance   │    │ • Alert Manager │    │ • Automation    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   Integration   │
                    │     Layer       │
                    │                 │
                    │ • Unified API   │
                    │ • Context Mgmt  │
                    │ • Config Mgmt   │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐    ┌─────────────────┐
                    │   Dashboard     │    │      CLI        │
                    │                 │    │                 │
                    │ • Web Interface │    │ • Status/Stats  │
                    │ • Real-time UI  │    │ • Management    │
                    │ • Charts/Graphs │    │ • Export/Import │
                    └─────────────────┘    └─────────────────┘
```

## 📊 Key Metrics Tracked

### System Performance

- **Memory:** Usage percentage, allocation patterns, GC pressure
- **CPU:** Utilization, load averages, process-specific usage
- **Goroutines:** Count, creation rate, lifecycle analysis
- **Network:** Throughput, connection counts, latency
- **Disk:** Usage, I/O operations, read/write throughput

### Application Metrics

- **Sessions:** Total, active, average duration, success rate
- **Commands:** Execution count, timing, success rate
- **Git Operations:** Commits, pushes, pulls, branch operations
- **Errors:** Count, rate, categorization, trends
- **Security:** Authentication events, permission checks

### Business Intelligence

- **Usage Patterns:** Peak hours, program preferences, trends
- **Performance Trends:** Response time evolution, bottlenecks
- **Growth Metrics:** User adoption, feature usage, scaling needs
- **Health Indicators:** System reliability, availability, performance

## 🎛️ Configuration

### Monitoring Configuration

```json
{
  "enabled": true,
  "storage_dir": "~/.claude-squad/monitoring",
  "usage": {
    "enabled": true,
    "buffer_size": 1000,
    "flush_interval": "30s",
    "async_logging": true
  },
  "metrics": {
    "collection_interval": "1m",
    "alert_thresholds": {
      "cpu_usage": 80.0,
      "memory_usage": 85.0,
      "error_rate": 0.05
    }
  },
  "dashboard": {
    "enabled": true,
    "port": 8080,
    "refresh_interval": "5s"
  },
  "reporting": {
    "auto_generate": true,
    "schedule": {
      "daily": true,
      "daily_at": "02:00"
    }
  }
}
```

### Integration Settings

- **Track Sessions:** Monitor session lifecycle and activity
- **Track Commands:** Log command execution and timing
- **Track Git Operations:** Monitor repository activities
- **Track Performance:** Collect operation timing data
- **Track Errors:** Log and categorize errors
- **Real-time Updates:** Enable live dashboard updates

## 🚀 Usage Examples

### Basic Monitoring

```bash
# Check system status
claude-squad monitoring status

# View detailed statistics
claude-squad monitoring stats

# List active sessions
claude-squad monitoring sessions active

# Show performance metrics
claude-squad monitoring performance
```

### Dashboard Management

```bash
# Start web dashboard
claude-squad monitoring dashboard start --port=8080

# Check dashboard status
claude-squad monitoring dashboard status

# Access dashboard at http://localhost:8080
```

### Report Generation

```bash
# Generate daily report
claude-squad monitoring reports generate daily

# Generate weekly report in CSV format
claude-squad monitoring reports generate weekly --format=csv

# List available report templates
claude-squad monitoring reports templates

# View report history
claude-squad monitoring reports history
```

### Data Export

```bash
# Export statistics as JSON
claude-squad monitoring export stats json --time=7d

# Export session data as CSV
claude-squad monitoring export sessions csv --output=sessions.csv

# Export performance metrics
claude-squad monitoring export metrics json --time=1h
```

### Alert Management

```bash
# List active alerts
claude-squad monitoring alerts list

# Acknowledge an alert
claude-squad monitoring alerts ack alert-123

# Clear acknowledged alerts
claude-squad monitoring alerts clear

# Show alert rules
claude-squad monitoring alerts rules
```

## 📈 Dashboard Features

### Real-time Monitoring

- Live system metrics updates
- Active session monitoring
- Performance graphs with historical data
- Alert notifications
- Health status indicators

### Interactive Widgets

- Customizable dashboard layout
- Drag-and-drop widget arrangement
- Real-time chart updates
- Drill-down capabilities
- Export functionality

### Mobile Responsive

- Optimized for mobile devices
- Touch-friendly interface
- Responsive layout adaptation
- Progressive web app features

## 📋 Report Capabilities

### Automated Insights

- Performance bottleneck identification
- Usage pattern analysis
- Anomaly detection
- Trend forecasting
- Optimization recommendations

### Executive Reporting

- High-level KPI summaries
- Strategic insights
- Growth analysis
- ROI metrics
- Decision support data

### Technical Reports

- Detailed performance analysis
- Error root cause analysis
- System capacity planning
- Security audit trails
- Compliance reporting

## 🔧 Integration Points

### Claude-Squad Integration

```go
// Track session creation
monitoring.TrackSessionCreated(sessionID, sessionName, program, repo, userID)

// Track command execution
monitoring.TrackCommandExecuted(sessionID, userID, command, duration, success, errorMsg)

// Track git operations
monitoring.TrackGitCommit(sessionID, userID, repo, branch, commitHash, success)

// Track performance
opID := monitoring.StartOperation("git_push")
defer monitoring.EndOperation(opID, sessionID, userID, success)

// Track errors
monitoring.TrackError(sessionID, userID, errorType, errorMsg, severity)
```

### Event-Driven Architecture

- Automatic event generation
- Context-aware tracking
- Performance impact minimization
- Graceful degradation
- Configurable verbosity

## 🛡️ Security and Privacy

### Data Protection

- Local data storage only
- Configurable data retention
- Sensitive data filtering
- Access control integration
- Audit trail maintenance

### Performance Impact

- Minimal overhead design
- Async processing
- Configurable sampling
- Resource-aware scaling
- Graceful failure handling

## 🎯 Benefits

### For Developers

- **Visibility:** Complete insight into system behavior
- **Performance:** Identify and resolve bottlenecks
- **Debugging:** Comprehensive error tracking and analysis
- **Optimization:** Data-driven performance improvements

### For Operations

- **Monitoring:** Real-time system health visibility
- **Alerting:** Proactive issue detection and notification
- **Reporting:** Automated operational reporting
- **Capacity Planning:** Usage trend analysis and forecasting

### For Management

- **Analytics:** Business intelligence and usage insights
- **Compliance:** Audit trails and regulatory reporting
- **ROI:** Productivity metrics and optimization opportunities
- **Decision Support:** Data-driven strategic planning

## 📋 Implementation Status

### ✅ Completed Components

1. **Usage Tracking Framework** - Complete event tracking and storage
2. **Metrics Collection System** - Comprehensive system and application metrics
3. **Integration Layer** - Unified monitoring interface and context management
4. **Web Dashboard** - Interactive monitoring dashboard with real-time updates
5. **Report Generation** - Automated report generation with multiple formats
6. **CLI Interface** - Comprehensive command-line management tools
7. **Configuration System** - Flexible configuration and customization
8. **Alert Management** - Real-time alerting with configurable thresholds

### 🎯 Key Achievements

- **Zero Breaking Changes:** Monitoring is optional and disabled by default
- **Production Ready:** Full async processing, error handling, and performance optimization
- **Developer Friendly:** Comprehensive CLI and clear integration APIs
- **Enterprise Features:** Advanced analytics, reporting, and alerting capabilities
- **Performance Optimized:** Minimal overhead with configurable resource usage
- **Scalable Architecture:** Modular design supporting future enhancements

### 🚀 Ready for Deployment

The monitoring framework is fully implemented and ready for:

- Development environments (with basic monitoring)
- Testing environments (with detailed tracking)
- Production environments (with full monitoring and alerting)

All components integrate seamlessly with the existing claude-squad codebase while maintaining complete backward compatibility.

---

**Monitoring Team Contact**: For monitoring-related questions or feature requests, please refer to the CLI help system and configuration documentation.
