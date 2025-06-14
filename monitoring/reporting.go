package monitoring

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportGenerator handles report generation for monitoring data
type ReportGenerator struct {
	config      *ReportConfig
	integration *MonitoringIntegration
	templates   map[string]*ReportTemplate
}

// ReportConfig represents report generation configuration
type ReportConfig struct {
	Enabled         bool                    `json:"enabled"`
	StorageDir      string                  `json:"storage_dir"`
	DefaultFormat   string                  `json:"default_format"` // json, csv, html, pdf
	AutoGenerate    bool                    `json:"auto_generate"`
	Schedule        ReportSchedule          `json:"schedule"`
	Recipients      []string                `json:"recipients"`
	Templates       map[string]ReportTemplate `json:"templates"`
	RetentionDays   int                     `json:"retention_days"`
	CompressOld     bool                    `json:"compress_old"`
}

// ReportSchedule represents automatic report generation schedule
type ReportSchedule struct {
	Daily   bool   `json:"daily"`
	Weekly  bool   `json:"weekly"`
	Monthly bool   `json:"monthly"`
	DailyAt string `json:"daily_at"`   // "02:00"
	WeeklyOn string `json:"weekly_on"` // "Sunday"
	MonthlyOn int   `json:"monthly_on"` // 1-31
}

// ReportTemplate defines report structure and content
type ReportTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"` // summary, detailed, custom
	Sections    []ReportSection        `json:"sections"`
	Format      string                 `json:"format"`
	Filters     ReportFilters          `json:"filters"`
	Options     map[string]interface{} `json:"options"`
}

// ReportSection represents a section in a report
type ReportSection struct {
	ID       string                 `json:"id"`
	Title    string                 `json:"title"`
	Type     string                 `json:"type"` // overview, chart, table, text
	Content  string                 `json:"content"`
	Data     map[string]interface{} `json:"data"`
	Options  map[string]interface{} `json:"options"`
}

// ReportFilters represents filters for report data
type ReportFilters struct {
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	UserID      string    `json:"user_id"`
	SessionID   string    `json:"session_id"`
	Program     string    `json:"program"`
	Repository  string    `json:"repository"`
	EventTypes  []string  `json:"event_types"`
	IncludeErrors bool    `json:"include_errors"`
	MinSeverity string    `json:"min_severity"`
}

// Report represents a generated report
type Report struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Type         string              `json:"type"`
	GeneratedAt  time.Time           `json:"generated_at"`
	TimeRange    TimeRange           `json:"time_range"`
	Format       string              `json:"format"`
	FilePath     string              `json:"file_path"`
	Size         int64               `json:"size"`
	Sections     []ReportSection     `json:"sections"`
	Summary      ReportSummary       `json:"summary"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ReportSummary provides key insights from the report
type ReportSummary struct {
	TotalEvents       int64             `json:"total_events"`
	TotalSessions     int64             `json:"total_sessions"`
	TotalCommands     int64             `json:"total_commands"`
	AverageSessionTime time.Duration    `json:"average_session_time"`
	ErrorRate         float64           `json:"error_rate"`
	TopPrograms       []ProgramUsage    `json:"top_programs"`
	TopRepositories   []RepoActivity    `json:"top_repositories"`
	KeyInsights       []string          `json:"key_insights"`
	Recommendations   []string          `json:"recommendations"`
	Trends            map[string]string `json:"trends"`
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(config *ReportConfig, integration *MonitoringIntegration) (*ReportGenerator, error) {
	generator := &ReportGenerator{
		config:      config,
		integration: integration,
		templates:   make(map[string]*ReportTemplate),
	}

	if !config.Enabled {
		return generator, nil
	}

	// Create storage directory
	if err := os.MkdirAll(config.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Load report templates
	generator.loadDefaultTemplates()
	
	// Load custom templates from config
	for id, template := range config.Templates {
		generator.templates[id] = &template
	}

	// Start automatic report generation if enabled
	if config.AutoGenerate {
		go generator.scheduleReports()
	}

	return generator, nil
}

// GenerateReport generates a report based on the specified type and time range
func (rg *ReportGenerator) GenerateReport(reportType string, timeRange TimeRange) (*Report, error) {
	if !rg.config.Enabled {
		return nil, fmt.Errorf("reporting is disabled")
	}

	template, exists := rg.templates[reportType]
	if !exists {
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}

	// Create report
	report := &Report{
		ID:          fmt.Sprintf("report_%d", time.Now().Unix()),
		Title:       template.Name,
		Type:        reportType,
		GeneratedAt: time.Now(),
		TimeRange:   timeRange,
		Format:      template.Format,
		Sections:    make([]ReportSection, 0),
		Metadata:    make(map[string]interface{}),
	}

	// Collect data
	stats := rg.integration.GetStats()
	sessions := rg.integration.GetSessions()

	// Apply filters
	filteredStats := rg.applyFilters(stats, template.Filters, timeRange)

	// Generate sections
	for _, sectionTemplate := range template.Sections {
		section, err := rg.generateSection(sectionTemplate, filteredStats, sessions)
		if err != nil {
			return nil, fmt.Errorf("failed to generate section %s: %w", sectionTemplate.ID, err)
		}
		report.Sections = append(report.Sections, *section)
	}

	// Generate summary
	report.Summary = rg.generateSummary(filteredStats, sessions)

	// Export report
	filePath, err := rg.exportReport(report)
	if err != nil {
		return nil, fmt.Errorf("failed to export report: %w", err)
	}

	report.FilePath = filePath

	// Get file size
	if fileInfo, err := os.Stat(filePath); err == nil {
		report.Size = fileInfo.Size()
	}

	return report, nil
}

// generateSection generates a specific report section
func (rg *ReportGenerator) generateSection(template ReportSection, stats *MonitoringStats, sessions map[string]*SessionContext) (*ReportSection, error) {
	section := ReportSection{
		ID:      template.ID,
		Title:   template.Title,
		Type:    template.Type,
		Options: template.Options,
		Data:    make(map[string]interface{}),
	}

	switch template.Type {
	case "overview":
		section.Data = rg.generateOverviewData(stats)
		section.Content = rg.formatOverviewContent(section.Data)

	case "sessions":
		section.Data = rg.generateSessionData(stats, sessions)
		section.Content = rg.formatSessionContent(section.Data)

	case "performance":
		section.Data = rg.generatePerformanceData(stats)
		section.Content = rg.formatPerformanceContent(section.Data)

	case "usage":
		section.Data = rg.generateUsageData(stats)
		section.Content = rg.formatUsageContent(section.Data)

	case "git":
		section.Data = rg.generateGitData(stats)
		section.Content = rg.formatGitContent(section.Data)

	case "errors":
		section.Data = rg.generateErrorData(stats)
		section.Content = rg.formatErrorContent(section.Data)

	case "trends":
		section.Data = rg.generateTrendData(stats)
		section.Content = rg.formatTrendContent(section.Data)

	case "chart":
		section.Data = rg.generateChartData(stats, template.Options)
		section.Content = rg.formatChartContent(section.Data)

	case "table":
		section.Data = rg.generateTableData(stats, template.Options)
		section.Content = rg.formatTableContent(section.Data)

	default:
		section.Content = template.Content
	}

	return &section, nil
}

// Data generation methods

func (rg *ReportGenerator) generateOverviewData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"total_sessions":       stats.Overview.TotalSessions,
		"active_sessions":      stats.Overview.ActiveSessions,
		"total_commands":       stats.Overview.TotalCommands,
		"total_git_operations": stats.Overview.TotalGitOperations,
		"average_session_time": stats.Overview.AverageSessionTime.String(),
		"commands_per_session": stats.Overview.CommandsPerSession,
		"error_rate":           fmt.Sprintf("%.2f%%", stats.Overview.ErrorRate*100),
		"top_programs":         stats.Overview.TopPrograms,
		"top_repositories":     stats.Overview.TopRepositories,
		"system_uptime":        stats.Overview.SystemUptime.String(),
	}
}

func (rg *ReportGenerator) generateSessionData(stats *MonitoringStats, sessions map[string]*SessionContext) map[string]interface{} {
	activeSessions := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		activeSessions = append(activeSessions, map[string]interface{}{
			"id":            session.ID,
			"name":          session.Name,
			"program":       session.Program,
			"start_time":    session.StartTime.Format(time.RFC3339),
			"duration":      time.Since(session.StartTime).String(),
			"command_count": session.CommandCount,
			"repository":    session.Repository,
			"branch":        session.Branch,
		})
	}

	return map[string]interface{}{
		"active_sessions":     activeSessions,
		"sessions_by_program": stats.SessionStats.SessionsByProgram,
		"total_sessions":      stats.SessionStats.TotalSessions,
		"average_session_time": stats.SessionStats.AverageSessionTime.String(),
	}
}

func (rg *ReportGenerator) generatePerformanceData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"memory_usage":        fmt.Sprintf("%.1f%%", stats.SystemMetrics.Performance.Memory.MemoryPercent),
		"cpu_usage":           fmt.Sprintf("%.1f%%", stats.SystemMetrics.Performance.CPU.UsagePercent),
		"goroutine_count":     stats.SystemMetrics.Performance.Goroutines.Count,
		"gc_collections":      stats.SystemMetrics.Performance.GC.NumGC,
		"last_gc_pause":       stats.SystemMetrics.Performance.GC.LastPause.String(),
		"performance_score":   fmt.Sprintf("%.1f", stats.SystemMetrics.Health.PerformanceScore),
		"response_times": map[string]string{
			"average": stats.SystemMetrics.Performance.ResponseTimes.Average.String(),
			"p95":     stats.SystemMetrics.Performance.ResponseTimes.P95.String(),
			"p99":     stats.SystemMetrics.Performance.ResponseTimes.P99.String(),
		},
	}
}

func (rg *ReportGenerator) generateUsageData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"commands_per_session":  stats.CommandStats.CommandsPerSession,
		"total_commands":        stats.CommandStats.TotalCommands,
		"commands_by_program":   stats.CommandStats.CommandsByProgram,
		"average_command_time":  stats.CommandStats.AverageCommandTime.String(),
		"events_by_type":        stats.UsageStats.EventsByType,
		"time_range": map[string]string{
			"start": stats.UsageStats.TimeRange.Start.Format(time.RFC3339),
			"end":   stats.UsageStats.TimeRange.End.Format(time.RFC3339),
		},
	}
}

func (rg *ReportGenerator) generateGitData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"total_commits":        stats.UsageStats.GitStats.TotalCommits,
		"total_pushes":         stats.UsageStats.GitStats.TotalPushes,
		"total_pulls":          stats.UsageStats.GitStats.TotalPulls,
		"branches_created":     stats.UsageStats.GitStats.BranchesCreated,
		"repository_activity":  stats.UsageStats.GitStats.RepositoryActivity,
		"most_active_repos":    stats.UsageStats.GitStats.MostActiveRepos,
	}
}

func (rg *ReportGenerator) generateErrorData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"total_errors":        stats.UsageStats.ErrorStats.TotalErrors,
		"error_rate":          fmt.Sprintf("%.3f%%", stats.UsageStats.ErrorStats.ErrorRate*100),
		"errors_by_type":      stats.UsageStats.ErrorStats.ErrorsByType,
		"most_common_errors":  stats.UsageStats.ErrorStats.MostCommonErrors,
		"error_trends":        stats.UsageStats.ErrorStats.ErrorTrends,
	}
}

func (rg *ReportGenerator) generateTrendData(stats *MonitoringStats) map[string]interface{} {
	return map[string]interface{}{
		"usage_trend":       string(stats.Trends.UsageGrowth.Direction),
		"performance_trend": string(stats.Trends.PerformanceTrend.Direction),
		"error_trend":       string(stats.Trends.ErrorTrend.Direction),
		"growth_rate":       fmt.Sprintf("%.2f%%", stats.Trends.PredictedGrowth),
		"predictions":       stats.Trends.Predictions,
		"recommended_actions": stats.Trends.RecommendedActions,
	}
}

func (rg *ReportGenerator) generateChartData(stats *MonitoringStats, options map[string]interface{}) map[string]interface{} {
	chartType, _ := options["chart_type"].(string)
	
	switch chartType {
	case "usage_over_time":
		return map[string]interface{}{
			"type":   "line",
			"labels": []string{"Week 1", "Week 2", "Week 3", "Week 4"},
			"data":   []int64{10, 15, 12, 18},
		}
	case "program_distribution":
		return map[string]interface{}{
			"type":   "pie",
			"labels": []string{},
			"data":   []int64{},
		}
	default:
		return map[string]interface{}{}
	}
}

func (rg *ReportGenerator) generateTableData(stats *MonitoringStats, options map[string]interface{}) map[string]interface{} {
	tableType, _ := options["table_type"].(string)
	
	switch tableType {
	case "session_summary":
		return map[string]interface{}{
			"headers": []string{"Session", "Program", "Commands", "Duration", "Status"},
			"rows":    [][]string{},
		}
	case "error_summary":
		return map[string]interface{}{
			"headers": []string{"Error Type", "Count", "Percentage", "Last Seen"},
			"rows":    [][]string{},
		}
	default:
		return map[string]interface{}{}
	}
}

// Content formatting methods

func (rg *ReportGenerator) formatOverviewContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# System Overview\n\n")
	content.WriteString(fmt.Sprintf("**Total Sessions:** %v\n", data["total_sessions"]))
	content.WriteString(fmt.Sprintf("**Active Sessions:** %v\n", data["active_sessions"]))
	content.WriteString(fmt.Sprintf("**Total Commands:** %v\n", data["total_commands"]))
	content.WriteString(fmt.Sprintf("**Commands per Session:** %.2f\n", data["commands_per_session"]))
	content.WriteString(fmt.Sprintf("**Error Rate:** %v\n", data["error_rate"]))
	content.WriteString(fmt.Sprintf("**System Uptime:** %v\n", data["system_uptime"]))
	
	return content.String()
}

func (rg *ReportGenerator) formatSessionContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Session Analysis\n\n")
	content.WriteString(fmt.Sprintf("**Total Sessions:** %v\n", data["total_sessions"]))
	content.WriteString(fmt.Sprintf("**Average Session Time:** %v\n", data["average_session_time"]))
	
	if sessionsByProgram, ok := data["sessions_by_program"].(map[string]int64); ok {
		content.WriteString("\n## Sessions by Program\n\n")
		for program, count := range sessionsByProgram {
			content.WriteString(fmt.Sprintf("- **%s:** %d sessions\n", program, count))
		}
	}
	
	return content.String()
}

func (rg *ReportGenerator) formatPerformanceContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Performance Metrics\n\n")
	content.WriteString(fmt.Sprintf("**Memory Usage:** %v\n", data["memory_usage"]))
	content.WriteString(fmt.Sprintf("**CPU Usage:** %v\n", data["cpu_usage"]))
	content.WriteString(fmt.Sprintf("**Goroutine Count:** %v\n", data["goroutine_count"]))
	content.WriteString(fmt.Sprintf("**Performance Score:** %v\n", data["performance_score"]))
	
	if responseTimes, ok := data["response_times"].(map[string]string); ok {
		content.WriteString("\n## Response Times\n\n")
		content.WriteString(fmt.Sprintf("- **Average:** %s\n", responseTimes["average"]))
		content.WriteString(fmt.Sprintf("- **95th Percentile:** %s\n", responseTimes["p95"]))
		content.WriteString(fmt.Sprintf("- **99th Percentile:** %s\n", responseTimes["p99"]))
	}
	
	return content.String()
}

func (rg *ReportGenerator) formatUsageContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Usage Analysis\n\n")
	content.WriteString(fmt.Sprintf("**Total Commands:** %v\n", data["total_commands"]))
	content.WriteString(fmt.Sprintf("**Commands per Session:** %.2f\n", data["commands_per_session"]))
	content.WriteString(fmt.Sprintf("**Average Command Time:** %v\n", data["average_command_time"]))
	
	return content.String()
}

func (rg *ReportGenerator) formatGitContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Git Operations\n\n")
	content.WriteString(fmt.Sprintf("**Total Commits:** %v\n", data["total_commits"]))
	content.WriteString(fmt.Sprintf("**Total Pushes:** %v\n", data["total_pushes"]))
	content.WriteString(fmt.Sprintf("**Total Pulls:** %v\n", data["total_pulls"]))
	content.WriteString(fmt.Sprintf("**Branches Created:** %v\n", data["branches_created"]))
	
	return content.String()
}

func (rg *ReportGenerator) formatErrorContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Error Analysis\n\n")
	content.WriteString(fmt.Sprintf("**Total Errors:** %v\n", data["total_errors"]))
	content.WriteString(fmt.Sprintf("**Error Rate:** %v\n", data["error_rate"]))
	
	return content.String()
}

func (rg *ReportGenerator) formatTrendContent(data map[string]interface{}) string {
	var content strings.Builder
	
	content.WriteString("# Trend Analysis\n\n")
	content.WriteString(fmt.Sprintf("**Usage Trend:** %v\n", data["usage_trend"]))
	content.WriteString(fmt.Sprintf("**Performance Trend:** %v\n", data["performance_trend"]))
	content.WriteString(fmt.Sprintf("**Error Trend:** %v\n", data["error_trend"]))
	content.WriteString(fmt.Sprintf("**Growth Rate:** %v\n", data["growth_rate"]))
	
	return content.String()
}

func (rg *ReportGenerator) formatChartContent(data map[string]interface{}) string {
	return "[Chart data would be rendered here based on format]"
}

func (rg *ReportGenerator) formatTableContent(data map[string]interface{}) string {
	return "[Table data would be rendered here based on format]"
}

// generateSummary generates a report summary
func (rg *ReportGenerator) generateSummary(stats *MonitoringStats, sessions map[string]*SessionContext) ReportSummary {
	insights := []string{}
	recommendations := []string{}
	trends := make(map[string]string)

	// Generate insights
	if stats.Overview.ErrorRate > 0.05 {
		insights = append(insights, fmt.Sprintf("Error rate of %.2f%% is above recommended threshold", stats.Overview.ErrorRate*100))
		recommendations = append(recommendations, "Review error logs and implement error reduction strategies")
	}

	if len(sessions) > 50 {
		insights = append(insights, "High number of active sessions detected")
		recommendations = append(recommendations, "Consider implementing session limits or cleanup policies")
	}

	if stats.SystemMetrics.Performance.Memory.MemoryPercent > 80 {
		insights = append(insights, "Memory usage is approaching critical levels")
		recommendations = append(recommendations, "Monitor memory usage and consider optimization")
	}

	// Generate trends
	trends["usage"] = string(stats.Trends.UsageGrowth.Direction)
	trends["performance"] = string(stats.Trends.PerformanceTrend.Direction)
	trends["errors"] = string(stats.Trends.ErrorTrend.Direction)

	return ReportSummary{
		TotalEvents:        stats.UsageStats.TotalEvents,
		TotalSessions:      stats.Overview.TotalSessions,
		TotalCommands:      stats.Overview.TotalCommands,
		AverageSessionTime: stats.Overview.AverageSessionTime,
		ErrorRate:          stats.Overview.ErrorRate,
		TopPrograms:        stats.Overview.TopPrograms,
		TopRepositories:    stats.Overview.TopRepositories,
		KeyInsights:        insights,
		Recommendations:    recommendations,
		Trends:             trends,
	}
}

// exportReport exports the report in the specified format
func (rg *ReportGenerator) exportReport(report *Report) (string, error) {
	timestamp := report.GeneratedAt.Format("20060102-150405")
	filename := fmt.Sprintf("%s_%s_%s.%s", report.Type, report.ID, timestamp, report.Format)
	filepath := filepath.Join(rg.config.StorageDir, filename)

	switch report.Format {
	case "json":
		return rg.exportJSON(report, filepath)
	case "csv":
		return rg.exportCSV(report, filepath)
	case "html":
		return rg.exportHTML(report, filepath)
	case "markdown":
		return rg.exportMarkdown(report, filepath)
	default:
		return rg.exportJSON(report, filepath)
	}
}

// exportJSON exports report as JSON
func (rg *ReportGenerator) exportJSON(report *Report, filepath string) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	return filepath, os.WriteFile(filepath, data, 0644)
}

// exportCSV exports report as CSV
func (rg *ReportGenerator) exportCSV(report *Report, filepath string) (string, error) {
	file, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"Section", "Key", "Value"})

	// Write summary data
	summaryData := [][]string{
		{"Summary", "Total Events", fmt.Sprintf("%d", report.Summary.TotalEvents)},
		{"Summary", "Total Sessions", fmt.Sprintf("%d", report.Summary.TotalSessions)},
		{"Summary", "Total Commands", fmt.Sprintf("%d", report.Summary.TotalCommands)},
		{"Summary", "Error Rate", fmt.Sprintf("%.4f", report.Summary.ErrorRate)},
		{"Summary", "Average Session Time", report.Summary.AverageSessionTime.String()},
	}

	for _, row := range summaryData {
		writer.Write(row)
	}

	return filepath, nil
}

// exportHTML exports report as HTML
func (rg *ReportGenerator) exportHTML(report *Report, filepath string) (string, error) {
	html := rg.generateHTMLReport(report)
	return filepath, os.WriteFile(filepath, []byte(html), 0644)
}

// exportMarkdown exports report as Markdown
func (rg *ReportGenerator) exportMarkdown(report *Report, filepath string) (string, error) {
	markdown := rg.generateMarkdownReport(report)
	return filepath, os.WriteFile(filepath, []byte(markdown), 0644)
}

// generateHTMLReport generates HTML report content
func (rg *ReportGenerator) generateHTMLReport(report *Report) string {
	var html strings.Builder

	html.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + report.Title + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { border-bottom: 2px solid #3498db; padding-bottom: 20px; margin-bottom: 30px; }
        .section { margin-bottom: 30px; }
        .metric { background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 10px 0; }
        .insights { background: #e8f5e8; padding: 15px; border-radius: 5px; }
        .recommendations { background: #fff3cd; padding: 15px; border-radius: 5px; }
        table { width: 100%; border-collapse: collapse; margin: 15px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>`)

	// Header
	html.WriteString(fmt.Sprintf(`
    <div class="header">
        <h1>%s</h1>
        <p><strong>Generated:</strong> %s</p>
        <p><strong>Time Range:</strong> %s - %s</p>
        <p><strong>Report ID:</strong> %s</p>
    </div>`,
		report.Title,
		report.GeneratedAt.Format(time.RFC3339),
		report.TimeRange.Start.Format(time.RFC3339),
		report.TimeRange.End.Format(time.RFC3339),
		report.ID))

	// Summary
	html.WriteString(`<div class="section">
        <h2>Executive Summary</h2>`)
	
	html.WriteString(fmt.Sprintf(`
        <div class="metric">
            <strong>Total Events:</strong> %d<br>
            <strong>Total Sessions:</strong> %d<br>
            <strong>Total Commands:</strong> %d<br>
            <strong>Error Rate:</strong> %.2f%%<br>
            <strong>Average Session Time:</strong> %s
        </div>`,
		report.Summary.TotalEvents,
		report.Summary.TotalSessions,
		report.Summary.TotalCommands,
		report.Summary.ErrorRate*100,
		report.Summary.AverageSessionTime.String()))

	// Key Insights
	if len(report.Summary.KeyInsights) > 0 {
		html.WriteString(`<div class="insights">
            <h3>Key Insights</h3>
            <ul>`)
		for _, insight := range report.Summary.KeyInsights {
			html.WriteString(fmt.Sprintf("<li>%s</li>", insight))
		}
		html.WriteString("</ul></div>")
	}

	// Recommendations
	if len(report.Summary.Recommendations) > 0 {
		html.WriteString(`<div class="recommendations">
            <h3>Recommendations</h3>
            <ul>`)
		for _, rec := range report.Summary.Recommendations {
			html.WriteString(fmt.Sprintf("<li>%s</li>", rec))
		}
		html.WriteString("</ul></div>")
	}

	html.WriteString("</div>")

	// Sections
	for _, section := range report.Sections {
		html.WriteString(fmt.Sprintf(`
        <div class="section">
            <h2>%s</h2>
            <div>%s</div>
        </div>`, section.Title, section.Content))
	}

	html.WriteString("</body></html>")
	return html.String()
}

// generateMarkdownReport generates Markdown report content
func (rg *ReportGenerator) generateMarkdownReport(report *Report) string {
	var md strings.Builder

	// Header
	md.WriteString(fmt.Sprintf("# %s\n\n", report.Title))
	md.WriteString(fmt.Sprintf("**Generated:** %s\n", report.GeneratedAt.Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("**Time Range:** %s - %s\n", report.TimeRange.Start.Format(time.RFC3339), report.TimeRange.End.Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("**Report ID:** %s\n\n", report.ID))

	// Summary
	md.WriteString("## Executive Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Total Events:** %d\n", report.Summary.TotalEvents))
	md.WriteString(fmt.Sprintf("- **Total Sessions:** %d\n", report.Summary.TotalSessions))
	md.WriteString(fmt.Sprintf("- **Total Commands:** %d\n", report.Summary.TotalCommands))
	md.WriteString(fmt.Sprintf("- **Error Rate:** %.2f%%\n", report.Summary.ErrorRate*100))
	md.WriteString(fmt.Sprintf("- **Average Session Time:** %s\n\n", report.Summary.AverageSessionTime.String()))

	// Key Insights
	if len(report.Summary.KeyInsights) > 0 {
		md.WriteString("### Key Insights\n\n")
		for _, insight := range report.Summary.KeyInsights {
			md.WriteString(fmt.Sprintf("- %s\n", insight))
		}
		md.WriteString("\n")
	}

	// Recommendations
	if len(report.Summary.Recommendations) > 0 {
		md.WriteString("### Recommendations\n\n")
		for _, rec := range report.Summary.Recommendations {
			md.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		md.WriteString("\n")
	}

	// Sections
	for _, section := range report.Sections {
		md.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
		md.WriteString(fmt.Sprintf("%s\n\n", section.Content))
	}

	return md.String()
}

// applyFilters applies filters to monitoring stats
func (rg *ReportGenerator) applyFilters(stats *MonitoringStats, filters ReportFilters, timeRange TimeRange) *MonitoringStats {
	// For now, return stats as-is
	// In a real implementation, this would filter the data based on the provided filters
	filteredStats := *stats
	filteredStats.UsageStats.TimeRange = timeRange
	return &filteredStats
}

// scheduleReports handles automatic report generation
func (rg *ReportGenerator) scheduleReports() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		// Check daily reports
		if rg.config.Schedule.Daily && now.Format("15:04") == rg.config.Schedule.DailyAt {
			rg.generateScheduledReport("daily", time.Hour*24)
		}

		// Check weekly reports
		if rg.config.Schedule.Weekly && now.Weekday().String() == rg.config.Schedule.WeeklyOn {
			rg.generateScheduledReport("weekly", time.Hour*24*7)
		}

		// Check monthly reports
		if rg.config.Schedule.Monthly && now.Day() == rg.config.Schedule.MonthlyOn {
			rg.generateScheduledReport("monthly", time.Hour*24*30)
		}
	}
}

// generateScheduledReport generates a scheduled report
func (rg *ReportGenerator) generateScheduledReport(reportType string, duration time.Duration) {
	now := time.Now()
	timeRange := TimeRange{
		Start: now.Add(-duration),
		End:   now,
	}

	_, err := rg.GenerateReport(reportType, timeRange)
	if err != nil {
		fmt.Printf("Failed to generate scheduled %s report: %v\n", reportType, err)
	}
}

// loadDefaultTemplates loads default report templates
func (rg *ReportGenerator) loadDefaultTemplates() {
	// Daily Summary Report Template
	rg.templates["daily"] = &ReportTemplate{
		ID:          "daily",
		Name:        "Daily Summary Report",
		Description: "Daily overview of system activity and performance",
		Type:        "summary",
		Format:      "html",
		Sections: []ReportSection{
			{ID: "overview", Title: "Overview", Type: "overview"},
			{ID: "sessions", Title: "Session Activity", Type: "sessions"},
			{ID: "performance", Title: "Performance Metrics", Type: "performance"},
			{ID: "errors", Title: "Error Analysis", Type: "errors"},
		},
	}

	// Weekly Detailed Report Template
	rg.templates["weekly"] = &ReportTemplate{
		ID:          "weekly",
		Name:        "Weekly Detailed Report",
		Description: "Comprehensive weekly analysis with trends",
		Type:        "detailed",
		Format:      "html",
		Sections: []ReportSection{
			{ID: "overview", Title: "Overview", Type: "overview"},
			{ID: "sessions", Title: "Session Analysis", Type: "sessions"},
			{ID: "usage", Title: "Usage Patterns", Type: "usage"},
			{ID: "performance", Title: "Performance Analysis", Type: "performance"},
			{ID: "git", Title: "Git Operations", Type: "git"},
			{ID: "trends", Title: "Trend Analysis", Type: "trends"},
			{ID: "errors", Title: "Error Analysis", Type: "errors"},
		},
	}

	// Monthly Executive Report Template
	rg.templates["monthly"] = &ReportTemplate{
		ID:          "monthly",
		Name:        "Monthly Executive Report",
		Description: "Executive summary with strategic insights",
		Type:        "summary",
		Format:      "html",
		Sections: []ReportSection{
			{ID: "overview", Title: "Executive Summary", Type: "overview"},
			{ID: "trends", Title: "Monthly Trends", Type: "trends"},
			{ID: "performance", Title: "System Health", Type: "performance"},
		},
	}
}

// GetAvailableReports returns list of available report types
func (rg *ReportGenerator) GetAvailableReports() []string {
	reports := make([]string, 0, len(rg.templates))
	for id := range rg.templates {
		reports = append(reports, id)
	}
	sort.Strings(reports)
	return reports
}

// GetReportHistory returns list of generated reports
func (rg *ReportGenerator) GetReportHistory() ([]Report, error) {
	if !rg.config.Enabled {
		return nil, fmt.Errorf("reporting is disabled")
	}

	files, err := os.ReadDir(rg.config.StorageDir)
	if err != nil {
		return nil, err
	}

	var reports []Report
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			// Try to load report metadata
			// This is a simplified version - would need full implementation
			info, err := file.Info()
			if err != nil {
				continue
			}

			reports = append(reports, Report{
				ID:          strings.TrimSuffix(file.Name(), ".json"),
				GeneratedAt: info.ModTime(),
				Size:        info.Size(),
				FilePath:    filepath.Join(rg.config.StorageDir, file.Name()),
			})
		}
	}

	// Sort by generation time (newest first)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].GeneratedAt.After(reports[j].GeneratedAt)
	})

	return reports, nil
}

// DefaultReportConfig returns default report configuration
func DefaultReportConfig() *ReportConfig {
	return &ReportConfig{
		Enabled:       true,
		StorageDir:    "~/.claude-squad/monitoring/reports",
		DefaultFormat: "html",
		AutoGenerate:  false,
		Schedule: ReportSchedule{
			Daily:     false,
			Weekly:    false,
			Monthly:   false,
			DailyAt:   "02:00",
			WeeklyOn:  "Sunday",
			MonthlyOn: 1,
		},
		Recipients:    []string{},
		Templates:     make(map[string]ReportTemplate),
		RetentionDays: 90,
		CompressOld:   true,
	}
}