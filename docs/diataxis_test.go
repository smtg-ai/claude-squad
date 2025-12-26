package docs

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewDiataxisFramework(t *testing.T) {
	config := &FrameworkConfig{
		MaxConcurrentWorkers:  5,
		EnableSyntaxHighlight: true,
	}

	fw := NewDiataxisFramework(config)

	if fw == nil {
		t.Fatal("Framework should not be nil")
	}

	if fw.config.MaxConcurrentWorkers != 5 {
		t.Errorf("Expected 5 workers, got %d", fw.config.MaxConcurrentWorkers)
	}
}

func TestAddDocument(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	doc := &Document{
		ID:          "test-doc",
		Type:        Tutorial,
		Title:       "Test Document",
		Description: "A test document",
		Content:     "This is test content",
	}

	err := fw.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	// Try to add the same document again
	err = fw.AddDocument(doc)
	if err == nil {
		t.Error("Should not allow duplicate document IDs")
	}
}

func TestGetDocument(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	doc := &Document{
		ID:      "test-doc",
		Type:    Tutorial,
		Title:   "Test Document",
		Content: "Content",
	}

	fw.AddDocument(doc)

	retrieved, err := fw.GetDocument("test-doc")
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}

	if retrieved.ID != doc.ID {
		t.Errorf("Expected ID %s, got %s", doc.ID, retrieved.ID)
	}

	if retrieved.Title != doc.Title {
		t.Errorf("Expected title %s, got %s", doc.Title, retrieved.Title)
	}
}

func TestGetDocumentsByType(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	tutorials := []*Document{
		{ID: "tut1", Type: Tutorial, Title: "Tutorial 1", Content: "Content"},
		{ID: "tut2", Type: Tutorial, Title: "Tutorial 2", Content: "Content"},
	}

	howtos := []*Document{
		{ID: "how1", Type: HowTo, Title: "How-To 1", Content: "Content"},
	}

	for _, doc := range tutorials {
		fw.AddDocument(doc)
	}
	for _, doc := range howtos {
		fw.AddDocument(doc)
	}

	retrievedTutorials := fw.GetDocumentsByType(Tutorial)
	if len(retrievedTutorials) != 2 {
		t.Errorf("Expected 2 tutorials, got %d", len(retrievedTutorials))
	}

	retrievedHowtos := fw.GetDocumentsByType(HowTo)
	if len(retrievedHowtos) != 1 {
		t.Errorf("Expected 1 how-to, got %d", len(retrievedHowtos))
	}
}

func TestConcurrentProcessing(t *testing.T) {
	fw := NewDiataxisFramework(&FrameworkConfig{
		MaxConcurrentWorkers: 10,
	})

	// Add multiple documents
	for i := 0; i < 20; i++ {
		doc := &Document{
			ID:      fmt.Sprintf("doc-%d", i),
			Type:    Tutorial,
			Title:   fmt.Sprintf("Document %d", i),
			Content: "# Test Content\n\n```go\nfunc test() {}\n```",
		}
		fw.AddDocument(doc)
	}

	ctx := context.Background()
	err := fw.ProcessAllDocuments(ctx)
	if err != nil {
		t.Fatalf("Failed to process documents: %v", err)
	}

	// Verify all documents were processed
	docs := fw.GetAllDocuments()
	for _, doc := range docs {
		if doc.ProcessedContent == "" {
			t.Errorf("Document %s was not processed", doc.ID)
		}
	}
}

func TestValidation(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	// Add a valid document
	validDoc := &Document{
		ID:          "valid-doc",
		Type:        Tutorial,
		Title:       "Valid Tutorial",
		Description: "A well-structured tutorial",
		Content:     "# Tutorial\n\nStep 1: Do this\nStep 2: Do that\n\n```bash\ncommand\n```",
		Version:     "1.0",
	}

	// Add an invalid document (missing required fields)
	invalidDoc := &Document{
		ID:      "invalid-doc",
		Type:    Tutorial,
		Content: "", // Empty content
	}

	fw.AddDocument(validDoc)
	fw.AddDocument(invalidDoc)

	ctx := context.Background()
	report, err := fw.ValidateAllDocuments(ctx)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if report.FailedCount == 0 {
		t.Error("Expected at least one failed validation")
	}

	if len(report.Issues) == 0 {
		t.Error("Expected validation issues")
	}
}

func TestQualityCalculation(t *testing.T) {
	calculator := NewQualityCalculator()

	highQualityDoc := &Document{
		ID:          "high-quality",
		Type:        Tutorial,
		Title:       "Comprehensive Tutorial",
		Description: "A very detailed and comprehensive tutorial with lots of information",
		Content: `# Tutorial

## Introduction

This is a comprehensive tutorial with multiple sections.

## Step 1

Detailed instructions here.

` + "```go\nfunc example() {}\n```" + `

## Step 2

More instructions.

` + "```bash\ncommand --flag\n```" + `

## Conclusion

Summary of what was learned.`,
		Metadata: map[string]interface{}{
			"difficulty": "beginner",
			"duration":   "30min",
			"category":   "getting-started",
		},
		Tags:        []string{"tutorial", "beginner", "getting-started"},
		Version:     "1.0",
		Author:      "Test Author",
		RelatedDocs: []string{"related-1", "related-2"},
	}

	score := calculator.Calculate(highQualityDoc)

	if score < 50 {
		t.Errorf("Expected high quality score (>50), got %.2f", score)
	}

	// Test low quality document
	lowQualityDoc := &Document{
		ID:      "low-quality",
		Type:    Tutorial,
		Title:   "Short",
		Content: "Brief content",
	}

	lowScore := calculator.Calculate(lowQualityDoc)

	if lowScore >= score {
		t.Errorf("Low quality doc should have lower score than high quality doc")
	}
}

func TestProgressTracking(t *testing.T) {
	tracker := NewProgressTracker()

	tracker.Start(10)

	for i := 0; i < 7; i++ {
		tracker.IncrementCompleted()
	}

	for i := 0; i < 2; i++ {
		tracker.IncrementFailed()
	}

	if tracker.Completed() != 7 {
		t.Errorf("Expected 7 completed, got %d", tracker.Completed())
	}

	if tracker.Failed() != 2 {
		t.Errorf("Expected 2 failed, got %d", tracker.Failed())
	}

	progress := tracker.Progress()
	expected := 90.0 // (7+2)/10 * 100
	if progress != expected {
		t.Errorf("Expected progress %.2f%%, got %.2f%%", expected, progress)
	}
}

func TestContextCancellation(t *testing.T) {
	fw := NewDiataxisFramework(&FrameworkConfig{
		MaxConcurrentWorkers: 10,
	})

	// Add many documents
	for i := 0; i < 100; i++ {
		fw.AddDocument(&Document{
			ID:      fmt.Sprintf("doc-%d", i),
			Type:    Tutorial,
			Title:   "Test",
			Content: "Content",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := fw.ProcessAllDocuments(ctx)

	// Should get context deadline exceeded or context canceled
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

// Note: Uses fmt.Sprintf from standard library

// ==================== VALIDATION RULE TESTS ====================

func TestRequiredFieldsRule(t *testing.T) {
	rule := &RequiredFieldsRule{}

	tests := []struct {
		name      string
		doc       *Document
		wantError bool
	}{
		{
			name: "valid document with all required fields",
			doc: &Document{
				ID:      "valid-1",
				Type:    Tutorial,
				Title:   "Valid Document",
				Content: "Some content",
			},
			wantError: false,
		},
		{
			name: "missing ID",
			doc: &Document{
				ID:      "",
				Type:    Tutorial,
				Title:   "No ID",
				Content: "Content",
			},
			wantError: true,
		},
		{
			name: "missing title",
			doc: &Document{
				ID:      "test-1",
				Type:    Tutorial,
				Title:   "",
				Content: "Content",
			},
			wantError: true,
		},
		{
			name: "missing type",
			doc: &Document{
				ID:      "test-1",
				Type:    "",
				Title:   "Title",
				Content: "Content",
			},
			wantError: true,
		},
		{
			name: "missing content",
			doc: &Document{
				ID:      "test-1",
				Type:    Tutorial,
				Title:   "Title",
				Content: "",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := rule.Validate(tt.doc)
			hasErrors := len(issues) > 0
			if hasErrors != tt.wantError {
				t.Errorf("RequiredFieldsRule.Validate() errors = %v, wantError %v", len(issues), tt.wantError)
			}
		})
	}
}

func TestContentLengthRule(t *testing.T) {
	rule := &ContentLengthRule{}

	tests := []struct {
		name          string
		contentLength int
		wantWarning   bool
	}{
		{"content too short", 50, true},
		{"content just right", 500, false},
		{"content long enough", 2000, false},
		{"content too long", 60000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := string(make([]byte, tt.contentLength))
			doc := &Document{
				ID:      "test",
				Type:    Tutorial,
				Title:   "Test",
				Content: content,
			}

			issues := rule.Validate(doc)
			hasWarning := len(issues) > 0

			if hasWarning != tt.wantWarning {
				t.Errorf("ContentLengthRule.Validate() warnings = %v, wantWarning %v", len(issues), tt.wantWarning)
			}
		})
	}
}

func TestCodeBlockValidityRule(t *testing.T) {
	rule := &CodeBlockValidityRule{}

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "valid code blocks",
			content:   "# Title\n\n```go\nfunc test() {}\n```\n\nMore text\n\n```bash\necho test\n```",
			wantError: false,
		},
		{
			name:      "unclosed code block",
			content:   "# Title\n\n```go\nfunc test() {}\n",
			wantError: true,
		},
		{
			name:      "code block without language",
			content:   "# Title\n\n```\ncode here\n```",
			wantError: false, // This triggers a warning, not error
		},
		{
			name:      "no code blocks",
			content:   "Just plain text",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				ID:      "test",
				Type:    Tutorial,
				Title:   "Test",
				Content: tt.content,
			}

			issues := rule.Validate(doc)
			hasErrors := false
			for _, issue := range issues {
				if issue.Severity == "error" {
					hasErrors = true
					break
				}
			}

			if hasErrors != tt.wantError {
				t.Errorf("CodeBlockValidityRule.Validate() errors = %v, wantError %v (issues: %v)", hasErrors, tt.wantError, issues)
			}
		})
	}
}

func TestDiataxisStructureRule_Tutorial(t *testing.T) {
	rule := &DiataxisStructureRule{}

	validTutorial := &Document{
		ID:      "tut-1",
		Type:    Tutorial,
		Title:   "Tutorial",
		Content: "# Learning Objectives\n\nGoal: Learn something\n\n1. First step\n2. Second step\n\n```go\nfunc example() {}\n```",
	}

	invalidTutorial := &Document{
		ID:      "tut-2",
		Type:    Tutorial,
		Title:   "Tutorial",
		Content: "Just some random content without tutorial structure",
	}

	validIssues := rule.Validate(validTutorial)
	if len(validIssues) > 0 {
		t.Errorf("Valid tutorial should not have issues, got: %v", validIssues)
	}

	invalidIssues := rule.Validate(invalidTutorial)
	if len(invalidIssues) == 0 {
		t.Error("Invalid tutorial should have structure warnings")
	}
}

func TestDiataxisStructureRule_HowTo(t *testing.T) {
	rule := &DiataxisStructureRule{}

	validHowTo := &Document{
		ID:      "how-1",
		Type:    HowTo,
		Title:   "How To",
		Content: "# How to solve this problem\n\n## Solution\n\n1. First step\n2. Second step\n\n```bash\ncommand\n```",
	}

	invalidHowTo := &Document{
		ID:      "how-2",
		Type:    HowTo,
		Title:   "How To",
		Content: "Some content without problem/solution structure",
	}

	validIssues := rule.Validate(validHowTo)
	if len(validIssues) > 0 {
		t.Errorf("Valid how-to should not have issues, got: %v", validIssues)
	}

	invalidIssues := rule.Validate(invalidHowTo)
	if len(invalidIssues) == 0 {
		t.Error("Invalid how-to should have structure warnings")
	}
}

func TestDiataxisStructureRule_Reference(t *testing.T) {
	rule := &DiataxisStructureRule{}

	validReference := &Document{
		ID:      "ref-1",
		Type:    Reference,
		Title:   "API Reference",
		Content: "# Function: doSomething\n\n## Parameters:\n\n| Name | Type | Description |\n|------|------|-------------|\n| param1 | string | Input parameter |\n\n## Returns:\n\nboolean value\n\n```go\nfunc doSomething(param1 string) bool\n```",
	}

	invalidReference := &Document{
		ID:      "ref-2",
		Type:    Reference,
		Title:   "Reference",
		Content: "Just some text without API details",
	}

	validIssues := rule.Validate(validReference)
	if len(validIssues) > 0 {
		t.Errorf("Valid reference should not have issues, got: %v", validIssues)
	}

	invalidIssues := rule.Validate(invalidReference)
	if len(invalidIssues) == 0 {
		t.Error("Invalid reference should have structure warnings")
	}
}

func TestDiataxisStructureRule_Explanation(t *testing.T) {
	rule := &DiataxisStructureRule{}

	validExplanation := &Document{
		ID:      "exp-1",
		Type:    Explanation,
		Title:   "Understanding Concepts",
		Content: "# Background\n\nWhy this concept matters\n\n# Concept\n\nTo understand this...",
	}

	invalidExplanation := &Document{
		ID:      "exp-2",
		Type:    Explanation,
		Title:   "Explanation",
		Content: "Some content without conceptual explanation",
	}

	validIssues := rule.Validate(validExplanation)
	if len(validIssues) > 0 {
		t.Errorf("Valid explanation should not have issues, got: %v", validIssues)
	}

	invalidIssues := rule.Validate(invalidExplanation)
	if len(invalidIssues) == 0 {
		t.Error("Invalid explanation should have structure warnings")
	}
}

func TestLinkValidityRule(t *testing.T) {
	rule := &LinkValidityRule{}

	tests := []struct {
		name       string
		content    string
		wantIssues int
	}{
		{
			name:       "valid links",
			content:    "[Link 1](https://example.com) and [Link 2](/docs/page)",
			wantIssues: 0,
		},
		{
			name:       "placeholder links",
			content:    "[TODO Link](#) and [Pending](TODO)",
			wantIssues: 2,
		},
		{
			name:       "mixed valid and invalid",
			content:    "[Good](https://example.com) [Bad](#)",
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				ID:      "test",
				Type:    Tutorial,
				Title:   "Test",
				Content: tt.content,
			}

			issues := rule.Validate(doc)
			if len(issues) != tt.wantIssues {
				t.Errorf("LinkValidityRule.Validate() issues = %d, want %d (issues: %v)", len(issues), tt.wantIssues, issues)
			}
		})
	}
}

func TestMetadataRule(t *testing.T) {
	rule := &MetadataRule{}

	minimalDoc := &Document{
		ID:      "test-1",
		Type:    Tutorial,
		Title:   "Test",
		Content: "Content",
	}

	fullDoc := &Document{
		ID:          "test-2",
		Type:        Tutorial,
		Title:       "Test",
		Content:     "Content",
		Description: "Full description",
		Tags:        []string{"tag1", "tag2"},
		Version:     "1.0",
	}

	minimalIssues := rule.Validate(minimalDoc)
	if len(minimalIssues) == 0 {
		t.Error("Minimal document should have info-level recommendations")
	}

	fullIssues := rule.Validate(fullDoc)
	if len(fullIssues) > 0 {
		t.Errorf("Full document should not have issues, got: %v", fullIssues)
	}
}

func TestCrossReferenceRule(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	// Add a valid reference document
	fw.AddDocument(&Document{
		ID:      "doc-exists",
		Type:    Tutorial,
		Title:   "Existing Document",
		Content: "Content",
	})

	rule := &CrossReferenceRule{framework: fw}

	tests := []struct {
		name         string
		relatedDocs  []string
		prereqs      []string
		wantErrors   int
		wantWarnings int
	}{
		{
			name:         "all references exist",
			relatedDocs:  []string{"doc-exists"},
			prereqs:      []string{},
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name:         "missing related doc",
			relatedDocs:  []string{"doc-missing"},
			prereqs:      []string{},
			wantErrors:   1,
			wantWarnings: 0,
		},
		{
			name:         "missing prerequisite",
			relatedDocs:  []string{},
			prereqs:      []string{"prereq-missing"},
			wantErrors:   0,
			wantWarnings: 1,
		},
		{
			name:         "mixed valid and invalid",
			relatedDocs:  []string{"doc-exists", "doc-missing"},
			prereqs:      []string{"prereq-missing"},
			wantErrors:   1,
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &Document{
				ID:            "test",
				Type:          Tutorial,
				Title:         "Test",
				Content:       "Content",
				RelatedDocs:   tt.relatedDocs,
				Prerequisites: tt.prereqs,
			}

			issues := rule.Validate(doc)

			errorCount := 0
			warningCount := 0
			for _, issue := range issues {
				if issue.Severity == "error" {
					errorCount++
				} else if issue.Severity == "warning" {
					warningCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("CrossReferenceRule.Validate() errors = %d, want %d", errorCount, tt.wantErrors)
			}
			if warningCount != tt.wantWarnings {
				t.Errorf("CrossReferenceRule.Validate() warnings = %d, want %d", warningCount, tt.wantWarnings)
			}
		})
	}
}

// ==================== DOCUMENT VALIDATE METHOD TESTS ====================

func TestDocumentValidate(t *testing.T) {
	tests := []struct {
		name           string
		doc            *Document
		wantStatus     ValidationStatus
		wantIssueCount int
	}{
		{
			name: "perfect document",
			doc: &Document{
				ID:          "perfect-1",
				Type:        Tutorial,
				Title:       "Perfect Tutorial",
				Description: "A comprehensive tutorial with all best practices",
				Content:     "# Tutorial\n\nGoal: Master the topic and learn best practices\n\n1. Learn this\n2. Learn that\n\n```go\nfunc example() {}\n```",
				Tags:        []string{"tutorial", "beginner"},
				Version:     "1.0",
			},
			wantStatus:     ValidationPassed,
			wantIssueCount: 0,
		},
		{
			name: "document with warnings",
			doc: &Document{
				ID:      "warning-1",
				Type:    Tutorial,
				Title:   "Short Tutorial",
				Content: "Brief content",
			},
			wantStatus:     ValidationFailed, // Will have errors due to missing structure
			wantIssueCount: 0,                // Will have warnings but not counted as issues
		},
		{
			name: "document with errors",
			doc: &Document{
				ID:      "error-1",
				Type:    Tutorial,
				Title:   "",
				Content: "",
			},
			wantStatus:     ValidationFailed,
			wantIssueCount: 0, // Will have errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := tt.doc.Validate()

			if tt.doc.ValidationStatus != tt.wantStatus {
				t.Errorf("Document.Validate() status = %v, want %v (issues: %v)", tt.doc.ValidationStatus, tt.wantStatus, issues)
			}
		})
	}
}

// ==================== EDGE CASE TESTS ====================

func TestAddDocument_EdgeCases(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	// Test empty ID
	emptyIDDoc := &Document{
		ID:      "",
		Type:    Tutorial,
		Title:   "Test",
		Content: "Content",
	}

	err := fw.AddDocument(emptyIDDoc)
	if err == nil {
		t.Error("AddDocument should reject empty ID")
	}

	// Test duplicate ID
	validDoc := &Document{
		ID:      "dup-test",
		Type:    Tutorial,
		Title:   "Test",
		Content: "Content",
	}

	err = fw.AddDocument(validDoc)
	if err != nil {
		t.Fatalf("Failed to add first document: %v", err)
	}

	err = fw.AddDocument(validDoc)
	if err == nil {
		t.Error("AddDocument should reject duplicate ID")
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	_, err := fw.GetDocument("non-existent")
	if err == nil {
		t.Error("GetDocument should return error for non-existent document")
	}
}

func TestGetDocumentsByType_Empty(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	tutorials := fw.GetDocumentsByType(Tutorial)
	if len(tutorials) != 0 {
		t.Errorf("Expected 0 tutorials, got %d", len(tutorials))
	}
}

func TestGetDocumentsByType_AllTypes(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	types := []DocType{Tutorial, HowTo, Reference, Explanation}

	for i, docType := range types {
		doc := &Document{
			ID:      fmt.Sprintf("doc-%d", i),
			Type:    docType,
			Title:   fmt.Sprintf("Document %d", i),
			Content: "Content",
		}
		fw.AddDocument(doc)
	}

	// Verify each type has exactly one document
	for _, docType := range types {
		docs := fw.GetDocumentsByType(docType)
		if len(docs) != 1 {
			t.Errorf("Expected 1 document of type %s, got %d", docType, len(docs))
		}
	}
}

func TestInvalidDocumentType(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	invalidDoc := &Document{
		ID:      "invalid-type",
		Type:    DocType("invalid"),
		Title:   "Invalid",
		Content: "Content",
	}

	// Should still add (type validation happens during Validate)
	err := fw.AddDocument(invalidDoc)
	if err != nil {
		t.Errorf("AddDocument should accept document with invalid type (validation happens later): %v", err)
	}
}

// ==================== STATISTICS TESTS ====================

func TestGetStatistics(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	// Add documents with various statuses
	docs := []*Document{
		{
			ID:               "doc-1",
			Type:             Tutorial,
			Title:            "Tutorial 1",
			Content:          "Content",
			ValidationStatus: ValidationPassed,
			QualityScore:     80.0,
		},
		{
			ID:               "doc-2",
			Type:             HowTo,
			Title:            "How-To 1",
			Content:          "Content",
			ValidationStatus: ValidationPassed,
			QualityScore:     90.0,
		},
		{
			ID:               "doc-3",
			Type:             Reference,
			Title:            "Reference 1",
			Content:          "Content",
			ValidationStatus: ValidationFailed,
			QualityScore:     40.0,
		},
	}

	for _, doc := range docs {
		fw.AddDocument(doc)
	}

	stats := fw.GetStatistics()

	if stats.TotalDocuments != 3 {
		t.Errorf("Expected 3 total documents, got %d", stats.TotalDocuments)
	}

	if stats.DocumentsByType[Tutorial] != 1 {
		t.Errorf("Expected 1 tutorial, got %d", stats.DocumentsByType[Tutorial])
	}

	if stats.DocumentsByType[HowTo] != 1 {
		t.Errorf("Expected 1 how-to, got %d", stats.DocumentsByType[HowTo])
	}

	if stats.DocumentsByType[Reference] != 1 {
		t.Errorf("Expected 1 reference, got %d", stats.DocumentsByType[Reference])
	}

	if stats.ValidationStats[ValidationPassed] != 2 {
		t.Errorf("Expected 2 passed, got %d", stats.ValidationStats[ValidationPassed])
	}

	if stats.ValidationStats[ValidationFailed] != 1 {
		t.Errorf("Expected 1 failed, got %d", stats.ValidationStats[ValidationFailed])
	}

	expectedAvg := (80.0 + 90.0 + 40.0) / 3.0
	if stats.AverageQualityScore != expectedAvg {
		t.Errorf("Expected average quality score %.2f, got %.2f", expectedAvg, stats.AverageQualityScore)
	}
}

func TestGetStatistics_Empty(t *testing.T) {
	fw := NewDiataxisFramework(nil)

	stats := fw.GetStatistics()

	if stats.TotalDocuments != 0 {
		t.Errorf("Expected 0 total documents, got %d", stats.TotalDocuments)
	}

	if stats.AverageQualityScore != 0 {
		t.Errorf("Expected 0 average quality score, got %.2f", stats.AverageQualityScore)
	}
}

// ==================== EXAMPLE FUNCTIONS ====================

// ExampleRequiredFieldsRule demonstrates validation of required document fields
func ExampleRequiredFieldsRule() {
	rule := &RequiredFieldsRule{}

	// Document missing title
	doc := &Document{
		ID:      "test-1",
		Type:    Tutorial,
		Title:   "", // Missing
		Content: "Some content",
	}

	issues := rule.Validate(doc)
	for _, issue := range issues {
		fmt.Printf("[%s] %s\n", issue.Severity, issue.Message)
	}

	// Output:
	// [error] Document title is required
}

// ExampleDiataxisStructureRule demonstrates type-specific structure validation
func ExampleDiataxisStructureRule() {
	rule := &DiataxisStructureRule{}

	// Well-structured tutorial
	tutorial := &Document{
		ID:      "tut-1",
		Type:    Tutorial,
		Title:   "Getting Started",
		Content: "# Learning Objectives\n\nGoal: Learn the basics\n\n1. First step\n2. Second step\n\n```go\nfunc example() {}\n```",
	}

	issues := rule.Validate(tutorial)
	if len(issues) == 0 {
		fmt.Println("Tutorial structure is valid")
	}

	// Output:
	// Tutorial structure is valid
}

// ExampleLinkValidityRule demonstrates link validation in documents
func ExampleLinkValidityRule() {
	rule := &LinkValidityRule{}

	doc := &Document{
		ID:      "test",
		Type:    Tutorial,
		Title:   "Test",
		Content: "[Good Link](https://example.com) [Placeholder](#) [TODO](TODO)",
	}

	issues := rule.Validate(doc)
	fmt.Printf("Found %d link issues\n", len(issues))

	// Output:
	// Found 2 link issues
}
