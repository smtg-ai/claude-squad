package docs

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewDiataxisFramework(t *testing.T) {
	config := &FrameworkConfig{
		MaxConcurrentWorkers: 5,
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
		Tags:    []string{"tutorial", "beginner", "getting-started"},
		Version: "1.0",
		Author:  "Test Author",
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
