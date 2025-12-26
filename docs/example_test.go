package docs

import (
	"context"
	"fmt"
	"time"
)

// ExampleDiataxisFramework_AddDocument demonstrates how to create and add
// different types of Diataxis documents to the framework.
func ExampleDiataxisFramework_AddDocument() {
	fw := NewDiataxisFramework(nil)

	// Create a Tutorial
	tutorial := &Document{
		ID:          "tutorial-basics",
		Type:        Tutorial,
		Title:       "Basic Tutorial",
		Description: "Learn the fundamentals step by step",
		Content:     "# Tutorial\n\nStep 1: Do this\nStep 2: Do that\n\n```bash\ncommand\n```",
		Tags:        []string{"beginner", "tutorial"},
	}

	// Create a How-To guide
	howto := &Document{
		ID:          "howto-deploy",
		Type:        HowTo,
		Title:       "How to Deploy",
		Description: "Deploy your application to production",
		Content:     "# Deployment\n\nProblem: Need to deploy\nSolution: Use these steps\n\n```bash\ndeploy.sh\n```",
		Tags:        []string{"deployment", "production"},
	}

	// Create a Reference document
	reference := &Document{
		ID:          "api-reference",
		Type:        Reference,
		Title:       "API Reference",
		Description: "Complete API documentation",
		Content:     "# API\n\n## Function: Process\n\nParameters: input string\nReturn: output string\n\n```go\nfunc Process(input string) string\n```",
		Tags:        []string{"api", "reference"},
	}

	// Create an Explanation document
	explanation := &Document{
		ID:          "explain-architecture",
		Type:        Explanation,
		Title:       "Understanding Architecture",
		Description: "Deep dive into system architecture concepts",
		Content:     "# Architecture\n\nConcept: Microservices\nWhy: Scalability and maintainability\n\nBackground: The evolution of...",
		Tags:        []string{"architecture", "concepts"},
	}

	// Add all documents
	fw.AddDocument(tutorial)
	fw.AddDocument(howto)
	fw.AddDocument(reference)
	fw.AddDocument(explanation)

	// Get documents by type
	tutorials := fw.GetDocumentsByType(Tutorial)
	howtos := fw.GetDocumentsByType(HowTo)
	references := fw.GetDocumentsByType(Reference)
	explanations := fw.GetDocumentsByType(Explanation)

	fmt.Printf("Tutorials: %d\n", len(tutorials))
	fmt.Printf("How-Tos: %d\n", len(howtos))
	fmt.Printf("References: %d\n", len(references))
	fmt.Printf("Explanations: %d\n", len(explanations))

	// Output:
	// Tutorials: 1
	// How-Tos: 1
	// References: 1
	// Explanations: 1
}

// ExampleDiataxisFramework_ValidateAllDocuments demonstrates concurrent
// validation of multiple documents using the framework.
func ExampleDiataxisFramework_ValidateAllDocuments() {
	fw := NewDiataxisFramework(nil)

	// Add multiple documents
	docs := []*Document{
		{
			ID:      "valid-doc",
			Type:    Tutorial,
			Title:   "Valid Tutorial",
			Content: "# Tutorial\n\nStep 1: Learn\nStep 2: Practice\n\n```go\nfunc main() {}\n```",
			Version: "1.0.0",
		},
		{
			ID:      "short-doc",
			Type:    HowTo,
			Title:   "Short Guide",
			Content: "Brief content",
		},
		{
			ID:      "missing-title",
			Type:    Reference,
			Content: "Content without title",
		},
	}

	for _, doc := range docs {
		fw.AddDocument(doc)
	}

	// Validate all documents concurrently
	ctx := context.Background()
	report, err := fw.ValidateAllDocuments(ctx)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Print validation report
	fmt.Printf("Total: %d\n", report.TotalDocuments)
	fmt.Printf("Failed: %d\n", report.FailedCount)
	fmt.Printf("Warnings: %d\n", report.WarningsCount)
	fmt.Printf("Has Issues: %t\n", len(report.Issues) > 0)

	// Output:
	// Total: 3
	// Failed: 2
	// Warnings: 1
	// Has Issues: true
}

// ExampleDiataxisFramework_ProcessAllDocuments demonstrates concurrent
// processing of multiple documents through the processing pipeline.
func ExampleDiataxisFramework_ProcessAllDocuments() {
	fw := NewDiataxisFramework(&FrameworkConfig{
		MaxConcurrentWorkers:  5,
		EnableSyntaxHighlight: true,
	})

	// Add documents with markdown content
	docs := []*Document{
		{
			ID:      "doc-1",
			Type:    Tutorial,
			Title:   "Tutorial 1",
			Content: "# Tutorial\n\nLearn **bold** and *italic* text.\n\n```go\nfunc test() {}\n```",
		},
		{
			ID:      "doc-2",
			Type:    HowTo,
			Title:   "How-To 1",
			Content: "# How-To\n\n- Item 1\n- Item 2\n\n```bash\necho 'hello'\n```",
		},
	}

	for _, doc := range docs {
		fw.AddDocument(doc)
	}

	// Process all documents concurrently
	ctx := context.Background()
	err := fw.ProcessAllDocuments(ctx)
	if err != nil {
		fmt.Printf("Processing error: %v\n", err)
		return
	}

	// Check that documents were processed
	allDocs := fw.GetAllDocuments()
	processedCount := 0
	for _, doc := range allDocs {
		if doc.ProcessedContent != "" {
			processedCount++
		}
	}

	fmt.Printf("Processed: %d\n", processedCount)

	// Output:
	// Processed: 2
}

// ExampleFrameworkStatistics demonstrates how to retrieve and interpret
// statistics about your documentation collection.
func ExampleFrameworkStatistics() {
	fw := NewDiataxisFramework(nil)

	// Add various documents
	docs := []*Document{
		{ID: "tut-1", Type: Tutorial, Title: "Tutorial 1", Content: "Content", QualityScore: 75.0},
		{ID: "tut-2", Type: Tutorial, Title: "Tutorial 2", Content: "Content", QualityScore: 80.0},
		{ID: "how-1", Type: HowTo, Title: "How-To 1", Content: "Content", QualityScore: 70.0},
		{ID: "ref-1", Type: Reference, Title: "Reference 1", Content: "Content", QualityScore: 85.0},
	}

	for _, doc := range docs {
		fw.AddDocument(doc)
	}

	// Validate to set status
	for _, doc := range docs {
		doc.Validate()
	}

	// Get statistics
	stats := fw.GetStatistics()

	fmt.Printf("Total: %d\n", stats.TotalDocuments)
	fmt.Printf("Tutorials: %d\n", stats.DocumentsByType[Tutorial])
	fmt.Printf("How-Tos: %d\n", stats.DocumentsByType[HowTo])
	fmt.Printf("References: %d\n", stats.DocumentsByType[Reference])
	fmt.Printf("Avg Quality: %.0f\n", stats.AverageQualityScore)

	// Output:
	// Total: 4
	// Tutorials: 2
	// How-Tos: 1
	// References: 1
	// Avg Quality: 78
}

// ExampleDocument demonstrates creating a complete, well-formed tutorial document
// with all recommended fields and proper structure.
func ExampleDocument() {
	doc := &Document{
		ID:          "comprehensive-tutorial",
		Type:        Tutorial,
		Title:       "Comprehensive Claude Squad Tutorial",
		Description: "A complete guide to mastering Claude Squad's multi-agent orchestration capabilities with practical examples and best practices",
		Content: `# Comprehensive Tutorial

## Learning Objectives

By the end of this tutorial, you will be able to:
- Understand multi-agent orchestration patterns
- Implement concurrent task processing
- Monitor and debug agent interactions

## Prerequisites

Before starting, ensure you have:
- Go 1.21 or later installed
- Basic understanding of concurrency
- Familiarity with the command line

## Step 1: Environment Setup

Set up your development environment:

` + "```bash\n# Install Claude Squad\ngo get github.com/seanchatmangpt/claude-squad\n\n# Verify installation\ngo version\n```" + `

## Step 2: Create Your First Agent

Create a simple agent:

` + "```go\npackage main\n\nimport (\n    \"fmt\"\n    \"github.com/seanchatmangpt/claude-squad/squad\"\n)\n\nfunc main() {\n    agent := squad.NewAgent(\"worker-1\")\n    fmt.Printf(\"Agent %s ready\\n\", agent.Name())\n}\n```" + `

## Step 3: Run Concurrent Tasks

Execute multiple tasks in parallel:

` + "```go\ntasks := []Task{\n    {ID: \"task-1\", Action: \"process\"},\n    {ID: \"task-2\", Action: \"analyze\"},\n}\n\nfor _, task := range tasks {\n    go agent.Execute(task)\n}\n```" + `

## What You've Learned

Congratulations! You now understand:
- Agent creation and initialization
- Concurrent task execution
- Basic error handling

## Next Steps

Continue learning with:
- Advanced orchestration patterns
- Performance optimization techniques
- Production deployment strategies`,
		FilePath: "/docs/tutorials/comprehensive-tutorial.md",
		Metadata: map[string]interface{}{
			"difficulty":        "beginner",
			"estimated_time":    "30 minutes",
			"prerequisites":     []string{"Go basics", "CLI familiarity"},
			"learning_path":     "fundamentals",
			"interactive":       true,
			"code_downloadable": true,
		},
		Tags:          []string{"tutorial", "beginner", "getting-started", "agents", "concurrency"},
		CreatedAt:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Version:       "1.2.0",
		Author:        "Claude Squad Team",
		RelatedDocs:   []string{"api-reference", "howto-deploy", "explain-architecture"},
		Prerequisites: []string{"installation-guide", "environment-setup"},
	}

	// Validate the document
	issues := doc.Validate()

	fmt.Printf("Document Type: %s\n", doc.Type)
	fmt.Printf("Document Title: %s\n", doc.Title)
	fmt.Printf("Tags: %d\n", len(doc.Tags))
	fmt.Printf("Version: %s\n", doc.Version)
	fmt.Printf("Has Issues: %t\n", len(issues) > 0)
	fmt.Printf("Validation Status: %s\n", doc.ValidationStatus)

	// Output:
	// Document Type: tutorial
	// Document Title: Comprehensive Claude Squad Tutorial
	// Tags: 5
	// Version: 1.2.0
	// Has Issues: true
	// Validation Status: warnings
}

// ExampleQualityCalculator_Calculate demonstrates how document quality
// is scored based on various factors.
func ExampleQualityCalculator_Calculate() {
	calculator := NewQualityCalculator()

	// High-quality document with all features
	highQualityDoc := &Document{
		ID:          "high-quality",
		Type:        Tutorial,
		Title:       "Excellent Tutorial",
		Description: "A comprehensive, well-documented tutorial with detailed examples and explanations",
		Content: `# Tutorial

## Introduction
Detailed introduction with clear learning objectives.

## Step 1
Comprehensive instructions with code examples.

` + "```go\nfunc example() {\n    fmt.Println(\"Hello\")\n}\n```" + `

## Step 2
More detailed instructions.

` + "```bash\necho 'test'\n```" + `

## Step 3
Final steps with verification.

## Conclusion
Summary of what was learned.`,
		Metadata: map[string]interface{}{
			"difficulty":  "beginner",
			"duration":    "30min",
			"category":    "getting-started",
			"interactive": true,
		},
		Tags:        []string{"tutorial", "beginner", "getting-started", "fundamentals"},
		Version:     "2.0.0",
		Author:      "Expert Author",
		RelatedDocs: []string{"related-1", "related-2", "related-3", "related-4"},
	}

	// Calculate quality score
	score := calculator.Calculate(highQualityDoc)

	fmt.Printf("Quality Score: %.0f\n", score)

	// Output:
	// Quality Score: 70
}

// ExampleValidationIssue demonstrates how validation issues are structured
// and how to interpret them.
func ExampleValidationIssue() {
	// Create a document with validation issues
	doc := &Document{
		ID:      "problematic-doc",
		Type:    Tutorial,
		Title:   "",      // Missing title - ERROR
		Content: "Short", // Too short - WARNING
	}

	// Validate and get issues
	issues := doc.Validate()

	// Count issues by severity
	errors := 0
	warnings := 0
	infos := 0
	for _, issue := range issues {
		switch issue.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		case "info":
			infos++
		}
	}

	fmt.Printf("Errors: %d\n", errors)
	fmt.Printf("Warnings: %d\n", warnings)
	fmt.Printf("Infos: %d\n", infos)
	fmt.Printf("Total Issues: %d\n", len(issues))

	// Output:
	// Errors: 3
	// Warnings: 2
	// Infos: 3
	// Total Issues: 8
}

// ExampleMarkdownParser demonstrates parsing markdown content to HTML
// with support for GitHub Flavored Markdown.
func ExampleMarkdownParser() {
	parser := NewMarkdownParser()

	markdown := `# Hello World

This is **bold** and *italic* text.

- Item 1
- Item 2

` + "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"

	html, err := parser.Parse(markdown)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Check that HTML was generated
	if len(html) > 0 {
		fmt.Println("HTML generated successfully")
	}

	// Output:
	// HTML generated successfully
}

// ExampleCodeExtractor demonstrates extracting code examples from
// markdown content for analysis and documentation.
func ExampleCodeExtractor() {
	extractor := NewCodeExtractor()

	content := `# Tutorial

Here's a Go example:

` + "```go\nfunc example() {\n    return true\n}\n```" + `

And a bash example:

` + "```bash\necho 'hello world'\n```"

	examples := extractor.Extract(content)

	fmt.Printf("Code examples found: %d\n", len(examples))
	for i, ex := range examples {
		fmt.Printf("Example %d: %s\n", i+1, ex.Language)
	}

	// Output:
	// Code examples found: 2
	// Example 1: go
	// Example 2: bash
}
