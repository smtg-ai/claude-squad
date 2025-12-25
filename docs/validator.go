package docs

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// DocumentValidator validates documents concurrently
type DocumentValidator struct {
	framework *DiataxisFramework
	rules     []ValidationRule
}

// NewDocumentValidator creates a new document validator
func NewDocumentValidator(framework *DiataxisFramework) *DocumentValidator {
	return &DocumentValidator{
		framework: framework,
		rules: []ValidationRule{
			&RequiredFieldsRule{},
			&ContentLengthRule{},
			&CodeBlockValidityRule{},
			&CrossReferenceRule{framework},
			&MetadataRule{},
			&DiataxisStructureRule{},
			&LinkValidityRule{},
		},
	}
}

// ValidateDocuments validates multiple documents concurrently
func (dv *DocumentValidator) ValidateDocuments(ctx context.Context, docs []*Document) (*ValidationReport, error) {
	startTime := time.Now()

	report := &ValidationReport{
		TotalDocuments: len(docs),
		Issues:         make([]ValidationIssue, 0),
	}

	// Concurrent validation with worker pool
	semaphore := make(chan struct{}, MaxConcurrentWorkers)
	issuesChan := make(chan []ValidationIssue, len(docs))
	var wg sync.WaitGroup

	for _, doc := range docs {
		wg.Add(1)

		go func(d *Document) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			// Validate the document
			issues := dv.validateDocument(ctx, d)
			issuesChan <- issues

			// Update document status
			if len(issues) == 0 {
				d.ValidationStatus = ValidationPassed
				report.PassedCount++
			} else {
				hasErrors := false
				for _, issue := range issues {
					if issue.Severity == "error" {
						hasErrors = true
						break
					}
				}

				if hasErrors {
					d.ValidationStatus = ValidationFailed
					report.FailedCount++
				} else {
					d.ValidationStatus = ValidationWarnings
					report.WarningsCount++
				}
			}
		}(doc)
	}

	// Wait for all validations to complete
	wg.Wait()
	close(issuesChan)

	// Collect all issues
	for issues := range issuesChan {
		report.Issues = append(report.Issues, issues...)
	}

	report.ProcessingTime = time.Since(startTime)

	return report, nil
}

// validateDocument validates a single document
func (dv *DocumentValidator) validateDocument(ctx context.Context, doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	for _, rule := range dv.rules {
		select {
		case <-ctx.Done():
			return issues
		default:
			if ruleIssues := rule.Validate(doc); len(ruleIssues) > 0 {
				issues = append(issues, ruleIssues...)
			}
		}
	}

	return issues
}

// ValidationRule defines a validation rule interface
type ValidationRule interface {
	Validate(doc *Document) []ValidationIssue
}

// RequiredFieldsRule validates required fields
type RequiredFieldsRule struct{}

func (r *RequiredFieldsRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	if doc.ID == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Document ID is required",
			Location:   "metadata",
		})
	}

	if doc.Title == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Document title is required",
			Location:   "metadata",
		})
	}

	if doc.Type == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Document type is required",
			Location:   "metadata",
		})
	}

	if doc.Content == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Document content is required",
			Location:   "content",
		})
	}

	return issues
}

// ContentLengthRule validates content length
type ContentLengthRule struct{}

func (r *ContentLengthRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	minLength := 100
	maxLength := 50000

	contentLength := len(doc.Content)

	if contentLength < minLength {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    fmt.Sprintf("Content is too short (%d characters, minimum %d)", contentLength, minLength),
			Location:   "content",
		})
	}

	if contentLength > maxLength {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    fmt.Sprintf("Content is too long (%d characters, maximum %d)", contentLength, maxLength),
			Location:   "content",
		})
	}

	return issues
}

// CodeBlockValidityRule validates code blocks
type CodeBlockValidityRule struct{}

func (r *CodeBlockValidityRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	// Check for unclosed code blocks
	openCount := 0
	lines := regexp.MustCompile(`\r?\n`).Split(doc.Content, -1)

	for _, line := range lines {
		if regexp.MustCompile("^```").MatchString(line) {
			openCount++
		}
	}

	if openCount%2 != 0 {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Unclosed code block detected",
			Location:   "content",
		})
	}

	// Check for code blocks without language specification
	unspecifiedCodeBlocks := regexp.MustCompile("```\\s*\\n").FindAllString(doc.Content, -1)
	if len(unspecifiedCodeBlocks) > 0 {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    fmt.Sprintf("Found %d code block(s) without language specification", len(unspecifiedCodeBlocks)),
			Location:   "content",
		})
	}

	return issues
}

// CrossReferenceRule validates cross-references
type CrossReferenceRule struct {
	framework *DiataxisFramework
}

func (r *CrossReferenceRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	// Check if related documents exist
	for _, relatedID := range doc.RelatedDocs {
		if _, err := r.framework.GetDocument(relatedID); err != nil {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "error",
				Message:    fmt.Sprintf("Related document not found: %s", relatedID),
				Location:   "metadata.related_docs",
			})
		}
	}

	// Check if prerequisites exist
	for _, prereqID := range doc.Prerequisites {
		if _, err := r.framework.GetDocument(prereqID); err != nil {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "warning",
				Message:    fmt.Sprintf("Prerequisite document not found: %s", prereqID),
				Location:   "metadata.prerequisites",
			})
		}
	}

	return issues
}

// MetadataRule validates document metadata
type MetadataRule struct{}

func (r *MetadataRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	// Check for recommended metadata
	if doc.Description == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "Document description is recommended",
			Location:   "metadata",
		})
	}

	if len(doc.Tags) == 0 {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "Adding tags improves discoverability",
			Location:   "metadata",
		})
	}

	if doc.Version == "" {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "Document version is recommended for tracking changes",
			Location:   "metadata",
		})
	}

	return issues
}

// DiataxisStructureRule validates Diataxis-specific structure
type DiataxisStructureRule struct{}

func (r *DiataxisStructureRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	content := doc.Content

	switch doc.Type {
	case Tutorial:
		if !containsKeywords(content, []string{"step", "learn", "objective", "goal"}) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "warning",
				Message:    "Tutorial should contain learning objectives and step-by-step instructions",
				Location:   "content",
			})
		}

	case HowTo:
		if !containsKeywords(content, []string{"problem", "solution", "how to"}) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "warning",
				Message:    "How-to guide should clearly state the problem and solution",
				Location:   "content",
			})
		}

	case Reference:
		if !containsKeywords(content, []string{"parameter", "return", "api", "function", "method"}) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "warning",
				Message:    "Reference documentation should include technical details like parameters and return values",
				Location:   "content",
			})
		}

	case Explanation:
		if !containsKeywords(content, []string{"concept", "why", "background", "understand"}) {
			issues = append(issues, ValidationIssue{
				DocumentID: doc.ID,
				Severity:   "warning",
				Message:    "Explanation should focus on concepts and understanding rather than tasks",
				Location:   "content",
			})
		}
	}

	return issues
}

// LinkValidityRule validates links in content
type LinkValidityRule struct{}

func (r *LinkValidityRule) Validate(doc *Document) []ValidationIssue {
	var issues []ValidationIssue

	// Find all markdown links
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := linkRe.FindAllStringSubmatch(doc.Content, -1)

	for _, match := range matches {
		if len(match) == 3 {
			link := match[2]

			// Check for empty links
			if link == "" {
				issues = append(issues, ValidationIssue{
					DocumentID: doc.ID,
					Severity:   "error",
					Message:    fmt.Sprintf("Empty link detected: [%s]()", match[1]),
					Location:   "content",
				})
			}

			// Check for placeholder links
			if link == "#" || link == "TODO" {
				issues = append(issues, ValidationIssue{
					DocumentID: doc.ID,
					Severity:   "warning",
					Message:    fmt.Sprintf("Placeholder link detected: [%s](%s)", match[1], link),
					Location:   "content",
				})
			}
		}
	}

	return issues
}

// containsKeywords checks if content contains any of the keywords
func containsKeywords(content string, keywords []string) bool {
	contentLower := regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	contentLower = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(contentLower, "")
	contentLower = " " + contentLower + " "

	for _, keyword := range keywords {
		pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(keyword))
		if regexp.MustCompile(pattern).MatchString(contentLower) {
			return true
		}
	}

	return false
}

// ValidationReporter generates validation reports
type ValidationReporter struct{}

// NewValidationReporter creates a new validation reporter
func NewValidationReporter() *ValidationReporter {
	return &ValidationReporter{}
}

// GenerateReport generates a human-readable validation report
func (vr *ValidationReporter) GenerateReport(report *ValidationReport) string {
	var buf []string

	buf = append(buf, "=== Validation Report ===\n")
	buf = append(buf, fmt.Sprintf("Total Documents: %d", report.TotalDocuments))
	buf = append(buf, fmt.Sprintf("Passed: %d", report.PassedCount))
	buf = append(buf, fmt.Sprintf("Failed: %d", report.FailedCount))
	buf = append(buf, fmt.Sprintf("Warnings: %d", report.WarningsCount))
	buf = append(buf, fmt.Sprintf("Processing Time: %v\n", report.ProcessingTime))

	if len(report.Issues) > 0 {
		buf = append(buf, "\n=== Issues ===\n")

		// Group issues by severity
		errors := filterIssuesBySeverity(report.Issues, "error")
		warnings := filterIssuesBySeverity(report.Issues, "warning")
		infos := filterIssuesBySeverity(report.Issues, "info")

		if len(errors) > 0 {
			buf = append(buf, "\nERRORS:")
			for _, issue := range errors {
				buf = append(buf, fmt.Sprintf("  [%s] %s - %s", issue.DocumentID, issue.Location, issue.Message))
			}
		}

		if len(warnings) > 0 {
			buf = append(buf, "\nWARNINGS:")
			for _, issue := range warnings {
				buf = append(buf, fmt.Sprintf("  [%s] %s - %s", issue.DocumentID, issue.Location, issue.Message))
			}
		}

		if len(infos) > 0 {
			buf = append(buf, "\nINFO:")
			for _, issue := range infos {
				buf = append(buf, fmt.Sprintf("  [%s] %s - %s", issue.DocumentID, issue.Location, issue.Message))
			}
		}
	}

	return joinStrings(buf, "\n")
}

// filterIssuesBySeverity filters issues by severity
func filterIssuesBySeverity(issues []ValidationIssue, severity string) []ValidationIssue {
	var filtered []ValidationIssue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// joinStrings is a helper to avoid import cycle
func joinStrings(elems []string, sep string) string {
	result := ""
	for i, elem := range elems {
		if i > 0 {
			result += sep
		}
		result += elem
	}
	return result
}
