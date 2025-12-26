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
	var reportMu sync.Mutex // Protect counter updates

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

			// Update document status and counters (protected by mutex)
			reportMu.Lock()
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
			reportMu.Unlock()
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
		issues = append(issues, r.validateTutorial(doc, content)...)

	case HowTo:
		issues = append(issues, r.validateHowTo(doc, content)...)

	case Reference:
		issues = append(issues, r.validateReference(doc, content)...)

	case Explanation:
		issues = append(issues, r.validateExplanation(doc, content)...)
	}

	return issues
}

// validateTutorial ensures tutorial structure (learning-oriented, step-by-step)
func (r *DiataxisStructureRule) validateTutorial(doc *Document, content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for learning objectives
	if !containsKeywords(content, []string{"learn", "objective", "goal", "by the end"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Tutorial must include learning objectives. Add a section like: '## Learning Objectives' or 'By the end of this tutorial, you will...'",
			Location:   "content.structure",
		})
	}

	// Check for numbered steps or step indicators
	hasNumberedSteps := regexp.MustCompile(`(?m)^(\d+\.|Step \d+|###?\s*Step)`).MatchString(content)
	if !hasNumberedSteps {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Tutorial must have numbered steps. Use '1.', '2.', '3.' or '## Step 1', '## Step 2', etc.",
			Location:   "content.structure",
		})
	}

	// Check for hands-on elements (code blocks)
	hasCodeBlocks := regexp.MustCompile("```").MatchString(content)
	if !hasCodeBlocks {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Tutorial should include practical code examples in code blocks (```language ... ```)",
			Location:   "content.examples",
		})
	}

	// Warn if it contains imperative language that sounds like a how-to
	if containsKeywords(content, []string{"if you want to", "to achieve", "in order to"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "Tutorial detected goal-oriented language. Consider if this should be a 'howto' instead. Tutorials should focus on learning, not solving specific problems.",
			Location:   "content.style",
		})
	}

	return issues
}

// validateHowTo ensures how-to guide structure (problem/solution oriented)
func (r *DiataxisStructureRule) validateHowTo(doc *Document, content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for problem statement
	hasProblemStatement := containsKeywords(content, []string{"problem", "how to", "to achieve", "if you want to", "in order to"})
	if !hasProblemStatement {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "How-to guide must state the problem being solved. Start with 'How to...', 'To achieve...', or 'If you want to...'",
			Location:   "content.problem",
		})
	}

	// Check for solution/steps section
	hasSolutionStructure := regexp.MustCompile(`(?i)(solution|steps?|procedure)`).MatchString(content) ||
		regexp.MustCompile(`(?m)^(\d+\.|##\s*)`).MatchString(content)
	if !hasSolutionStructure {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "How-to guide must provide clear steps. Add numbered steps (1., 2., 3.) or sections (## Solution, ## Steps)",
			Location:   "content.solution",
		})
	}

	// Check for practical examples
	hasCodeBlocks := regexp.MustCompile("```").MatchString(content)
	if !hasCodeBlocks {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "How-to guide should include code examples showing the solution in practice",
			Location:   "content.examples",
		})
	}

	// Warn if it contains tutorial-style learning language
	if containsKeywords(content, []string{"learn", "understand", "explore", "discover"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "How-to guide contains learning-oriented language. Consider if this should be a 'tutorial' instead. How-tos should be goal-focused, not learning-focused.",
			Location:   "content.style",
		})
	}

	return issues
}

// validateReference ensures reference doc structure (technical details)
func (r *DiataxisStructureRule) validateReference(doc *Document, content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for technical API elements
	hasTechnicalDetails := containsKeywords(content, []string{"parameter", "return", "argument", "type", "api", "function", "method", "property", "field"})
	if !hasTechnicalDetails {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Reference documentation must describe technical details. Include: Parameters, Return values, Types, or API signatures",
			Location:   "content.technical_details",
		})
	}

	// Check for structured parameter documentation
	hasParameterDocs := regexp.MustCompile(`(?i)(parameter|param|argument|arg)s?:`).MatchString(content) ||
		regexp.MustCompile(`\|\s*Name\s*\|\s*Type\s*\|`).MatchString(content)
	if !hasParameterDocs && containsKeywords(content, []string{"function", "method", "api"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Reference should document parameters formally. Use 'Parameters:' section or table format: | Name | Type | Description |",
			Location:   "content.parameters",
		})
	}

	// Check for return value documentation
	hasReturnDocs := containsKeywords(content, []string{"return", "returns", "response", "output"})
	if !hasReturnDocs && containsKeywords(content, []string{"function", "method", "api"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Reference should document return values. Add 'Returns:' section describing what the function/method returns",
			Location:   "content.returns",
		})
	}

	// Check for code examples (signature/usage)
	hasCodeBlocks := regexp.MustCompile("```").MatchString(content)
	if !hasCodeBlocks {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Reference should include code signatures or usage examples in code blocks",
			Location:   "content.examples",
		})
	}

	// Warn if contains explanatory language
	if containsKeywords(content, []string{"why", "because", "reason", "understanding"}) {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "info",
			Message:    "Reference contains explanatory language. Consider moving 'why' discussions to a separate 'explanation' document. References should be information-dense, not explanatory.",
			Location:   "content.style",
		})
	}

	return issues
}

// validateExplanation ensures explanation structure (concept-oriented)
func (r *DiataxisStructureRule) validateExplanation(doc *Document, content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for conceptual focus
	hasConceptualContent := containsKeywords(content, []string{"concept", "why", "because", "reason", "background", "understand", "theory", "principle"})
	if !hasConceptualContent {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "error",
			Message:    "Explanation must focus on concepts and understanding. Include discussions of 'why', concepts, background, or principles",
			Location:   "content.conceptual",
		})
	}

	// Check for context/background section
	hasContextSection := regexp.MustCompile(`(?i)(##\s*(background|context|overview|introduction)|why\s+)`).MatchString(content)
	if !hasContextSection {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Explanation should provide context/background. Add a '## Background' or '## Overview' section explaining the 'why'",
			Location:   "content.context",
		})
	}

	// Warn if contains imperative/instructional language (should be in how-to or tutorial)
	imperativeCount := countImperativeVerbs(content)
	if imperativeCount > 5 {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    fmt.Sprintf("Explanation contains %d imperative instructions (e.g., 'click', 'run', 'install'). Consider if these steps belong in a 'tutorial' or 'howto' instead. Explanations should discuss concepts, not give instructions.", imperativeCount),
			Location:   "content.style",
		})
	}

	// Warn if contains step-by-step structure
	hasNumberedSteps := regexp.MustCompile(`(?m)^(\d+\.|Step \d+)`).MatchString(content)
	if hasNumberedSteps {
		issues = append(issues, ValidationIssue{
			DocumentID: doc.ID,
			Severity:   "warning",
			Message:    "Explanation contains numbered steps. Consider if this should be a 'tutorial' or 'howto' instead. Explanations should discuss concepts, not provide step-by-step instructions.",
			Location:   "content.structure",
		})
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

// countImperativeVerbs counts imperative/instructional verbs in content
func countImperativeVerbs(content string) int {
	// Common imperative verbs used in instructions
	imperativeVerbs := []string{
		"click", "run", "install", "execute", "start", "stop",
		"create", "delete", "remove", "add", "update", "modify",
		"open", "close", "save", "load", "download", "upload",
		"configure", "set", "enable", "disable", "activate",
		"navigate", "go to", "select", "choose", "enter", "type",
	}

	count := 0
	contentLower := content

	for _, verb := range imperativeVerbs {
		// Match imperative usage (start of sentence or after newline/period)
		pattern := fmt.Sprintf(`(?im)(^|[\.\n])\s*%s\b`, regexp.QuoteMeta(verb))
		matches := regexp.MustCompile(pattern).FindAllString(contentLower, -1)
		count += len(matches)
	}

	return count
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
