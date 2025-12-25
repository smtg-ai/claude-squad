package cmd

import (
	"claude-squad/docs"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// DocsCommand creates the documentation command
func DocsCommand() *cobra.Command {
	var outputDir string
	var validateOnly bool
	var statsOnly bool
	var inputDir string

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage Diataxis documentation framework",
		Long: `Generate, validate, and manage documentation using the Diataxis framework.

The Diataxis framework organizes documentation into four types:
  - Tutorials: Learning-oriented, step-by-step guides
  - How-To Guides: Task-oriented, problem-solving guides
  - Reference: Information-oriented, technical descriptions
  - Explanation: Understanding-oriented, conceptual discussions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDocsGeneration(inputDir, outputDir, validateOnly, statsOnly)
		},
	}

	// Subcommands
	cmd.AddCommand(docsGenerateCommand())
	cmd.AddCommand(docsValidateCommand())
	cmd.AddCommand(docsInitCommand())
	cmd.AddCommand(docsStatsCommand())

	// Flags
	cmd.PersistentFlags().StringVarP(&outputDir, "output", "o", "./docs-output", "Output directory for generated documentation")
	cmd.PersistentFlags().StringVarP(&inputDir, "input", "i", "./documentation", "Input directory containing documentation markdown files")
	cmd.Flags().BoolVarP(&validateOnly, "validate", "v", false, "Only validate documentation without generating output")
	cmd.Flags().BoolVarP(&statsOnly, "stats", "s", false, "Only show documentation statistics")

	return cmd
}

// docsGenerateCommand creates the generate subcommand
func docsGenerateCommand() *cobra.Command {
	var outputDir string
	var inputDir string
	var workers int

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate documentation site from markdown files",
		Long: `Process all markdown files and generate a complete documentation site
using the Diataxis framework with concurrent processing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config := &docs.FrameworkConfig{
				MaxConcurrentWorkers:     workers,
				EnableSyntaxHighlight:    true,
				EnableCrossRefValidation: true,
				EnableMetrics:            true,
				OutputFormat:             "html",
				OutputDir:                outputDir,
			}

			framework := docs.NewDiataxisFramework(config)

			// Load documents from input directory
			if err := loadDocumentsFromDirectory(framework, inputDir); err != nil {
				return fmt.Errorf("failed to load documents: %w", err)
			}

			// Generate documentation
			ctx := context.Background()
			if err := framework.GenerateDocumentation(ctx); err != nil {
				return fmt.Errorf("failed to generate documentation: %w", err)
			}

			fmt.Printf("✓ Documentation generated successfully at %s\n", outputDir)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./docs-output", "Output directory")
	cmd.Flags().StringVarP(&inputDir, "input", "i", "./documentation", "Input directory")
	cmd.Flags().IntVarP(&workers, "workers", "w", 10, "Number of concurrent workers")

	return cmd
}

// docsValidateCommand creates the validate subcommand
func docsValidateCommand() *cobra.Command {
	var inputDir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate documentation files",
		Long:  `Validate all documentation files for correctness, structure, and Diataxis compliance.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config := &docs.FrameworkConfig{
				MaxConcurrentWorkers:     10,
				EnableCrossRefValidation: true,
			}

			framework := docs.NewDiataxisFramework(config)

			// Load documents
			if err := loadDocumentsFromDirectory(framework, inputDir); err != nil {
				return fmt.Errorf("failed to load documents: %w", err)
			}

			// Validate
			ctx := context.Background()
			report, err := framework.ValidateAllDocuments(ctx)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Output results
			if jsonOutput {
				reportJSON, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(reportJSON))
			} else {
				reporter := docs.NewValidationReporter()
				fmt.Println(reporter.GenerateReport(report))
			}

			// Exit with error code if validation failed
			if report.FailedCount > 0 {
				return fmt.Errorf("validation failed: %d documents have errors", report.FailedCount)
			}

			fmt.Printf("\n✓ All documents validated successfully\n")
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputDir, "input", "i", "./documentation", "Input directory")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output results as JSON")

	return cmd
}

// docsInitCommand creates the init subcommand
func docsInitCommand() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Diataxis documentation structure",
		Long:  `Create a new documentation structure with example files for all four Diataxis types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create directory structure
			types := []string{"tutorials", "howto", "reference", "explanation"}

			for _, docType := range types {
				dir := filepath.Join(outputDir, docType)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}

				// Create example file
				examplePath := filepath.Join(dir, "example.md")
				example := getExampleContent(docType)

				if err := os.WriteFile(examplePath, []byte(example), 0644); err != nil {
					return fmt.Errorf("failed to write example file: %w", err)
				}

				fmt.Printf("✓ Created %s\n", examplePath)
			}

			// Create README
			readmePath := filepath.Join(outputDir, "README.md")
			readme := getDiataxisREADME()
			if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
				return fmt.Errorf("failed to write README: %w", err)
			}

			fmt.Printf("\n✓ Documentation structure initialized at %s\n", outputDir)
			fmt.Println("\nNext steps:")
			fmt.Println("  1. Edit the example files in each directory")
			fmt.Println("  2. Run 'claude-squad docs generate' to build the documentation site")
			fmt.Println("  3. Run 'claude-squad docs validate' to check for issues")

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./documentation", "Output directory")

	return cmd
}

// docsStatsCommand creates the stats subcommand
func docsStatsCommand() *cobra.Command {
	var inputDir string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show documentation statistics",
		Long:  `Display statistics about the documentation including counts by type and quality scores.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config := &docs.FrameworkConfig{
				MaxConcurrentWorkers: 10,
			}

			framework := docs.NewDiataxisFramework(config)

			// Load documents
			if err := loadDocumentsFromDirectory(framework, inputDir); err != nil {
				return fmt.Errorf("failed to load documents: %w", err)
			}

			// Process documents to calculate quality scores
			ctx := context.Background()
			if err := framework.ProcessAllDocuments(ctx); err != nil {
				return fmt.Errorf("failed to process documents: %w", err)
			}

			// Get statistics
			stats := framework.GetStatistics()

			// Display statistics
			fmt.Println("=== Documentation Statistics ===\n")
			fmt.Printf("Total Documents: %d\n\n", stats.TotalDocuments)

			fmt.Println("Documents by Type:")
			for docType, count := range stats.DocumentsByType {
				fmt.Printf("  %s: %d\n", docType, count)
			}

			fmt.Println("\nValidation Status:")
			for status, count := range stats.ValidationStats {
				fmt.Printf("  %s: %d\n", status, count)
			}

			fmt.Printf("\nAverage Quality Score: %.2f/100\n", stats.AverageQualityScore)

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputDir, "input", "i", "./documentation", "Input directory")

	return cmd
}

// Helper functions

func runDocsGeneration(inputDir, outputDir string, validateOnly, statsOnly bool) error {
	config := &docs.FrameworkConfig{
		MaxConcurrentWorkers:     10,
		EnableSyntaxHighlight:    true,
		EnableCrossRefValidation: true,
		EnableMetrics:            true,
		OutputFormat:             "html",
		OutputDir:                outputDir,
	}

	framework := docs.NewDiataxisFramework(config)

	// Load documents
	if err := loadDocumentsFromDirectory(framework, inputDir); err != nil {
		return fmt.Errorf("failed to load documents: %w", err)
	}

	ctx := context.Background()

	if statsOnly {
		// Process and show stats
		if err := framework.ProcessAllDocuments(ctx); err != nil {
			return err
		}

		stats := framework.GetStatistics()
		fmt.Printf("Total Documents: %d\n", stats.TotalDocuments)
		fmt.Printf("Average Quality Score: %.2f\n", stats.AverageQualityScore)
		return nil
	}

	if validateOnly {
		// Validate only
		report, err := framework.ValidateAllDocuments(ctx)
		if err != nil {
			return err
		}

		reporter := docs.NewValidationReporter()
		fmt.Println(reporter.GenerateReport(report))
		return nil
	}

	// Generate documentation
	if err := framework.GenerateDocumentation(ctx); err != nil {
		return err
	}

	fmt.Printf("✓ Documentation generated at %s\n", outputDir)
	return nil
}

func loadDocumentsFromDirectory(framework *docs.DiataxisFramework, dir string) error {
	// Walk through directory and load markdown files
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-markdown files
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Parse frontmatter
		parser := docs.NewMarkdownParser()
		metadata, body, err := parser.ParseWithFrontmatter(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Create document
		doc := &docs.Document{
			ID:       getDocID(metadata, path),
			Type:     getDocType(metadata, path),
			Title:    getMetadataString(metadata, "title", filepath.Base(path)),
			Description: getMetadataString(metadata, "description", ""),
			Content:  body,
			FilePath: path,
			Metadata: metadata,
			Tags:     getMetadataStringSlice(metadata, "tags"),
			Version:  getMetadataString(metadata, "version", "1.0"),
			Author:   getMetadataString(metadata, "author", ""),
			RelatedDocs: getMetadataStringSlice(metadata, "related"),
			Prerequisites: getMetadataStringSlice(metadata, "prerequisites"),
		}

		// Add to framework
		return framework.AddDocument(doc)
	})
}

func getDocID(metadata map[string]interface{}, path string) string {
	if id, ok := metadata["id"].(string); ok {
		return id
	}
	// Generate ID from filename
	base := filepath.Base(path)
	return base[:len(base)-len(filepath.Ext(base))]
}

func getDocType(metadata map[string]interface{}, path string) docs.DocType {
	if docType, ok := metadata["type"].(string); ok {
		return docs.DocType(docType)
	}

	// Infer from directory
	dir := filepath.Base(filepath.Dir(path))
	switch dir {
	case "tutorials":
		return docs.Tutorial
	case "howto":
		return docs.HowTo
	case "reference":
		return docs.Reference
	case "explanation":
		return docs.Explanation
	default:
		return docs.Tutorial
	}
}

func getMetadataString(metadata map[string]interface{}, key, defaultValue string) string {
	if val, ok := metadata[key].(string); ok {
		return val
	}
	return defaultValue
}

func getMetadataStringSlice(metadata map[string]interface{}, key string) []string {
	if val, ok := metadata[key].([]interface{}); ok {
		result := make([]string, 0, len(val))
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func getExampleContent(docType string) string {
	switch docType {
	case "tutorials":
		return tutorialExample
	case "howto":
		return howtoExample
	case "reference":
		return referenceExample
	case "explanation":
		return explanationExample
	default:
		return ""
	}
}

const tutorialExample = `---
id: getting-started
type: tutorial
title: Getting Started with Claude Squad
description: Learn how to set up and use Claude Squad to manage multiple AI agents
version: 1.0
tags:
  - getting-started
  - tutorial
  - beginner
---

# Getting Started with Claude Squad

Welcome! This tutorial will guide you through setting up and using Claude Squad to manage multiple AI coding assistants.

## Learning Objectives

By the end of this tutorial, you will:
- Understand what Claude Squad is and why it's useful
- Install and configure Claude Squad
- Create and manage your first AI agent session
- Work with multiple agents simultaneously

## Prerequisites

- Git installed on your system
- A terminal/command line interface
- Basic familiarity with command-line tools

## Step 1: Installation

First, let's install Claude Squad:

` + "```bash" + `
# Download and run the installation script
curl -fsSL https://raw.githubusercontent.com/smtg-ai/claude-squad/main/install.sh | bash
` + "```" + `

The installer will download the latest version and set up Claude Squad on your system.

## Step 2: Verify Installation

Check that Claude Squad is installed correctly:

` + "```bash" + `
claude-squad version
` + "```" + `

You should see the version number displayed.

## Step 3: Initialize Your First Session

Navigate to a git repository and start Claude Squad:

` + "```bash" + `
cd /path/to/your/project
claude-squad
` + "```" + `

This opens the Claude Squad interface.

## Step 4: Create an Agent Session

1. Press 'n' to create a new session
2. Enter a description for your task
3. Claude Squad will create an isolated workspace

## What You've Learned

Congratulations! You've learned:
- How to install Claude Squad
- How to start the application
- How to create your first agent session

## Next Steps

- Try creating multiple sessions simultaneously
- Learn about advanced features in the How-To guides
- Read the Reference documentation for detailed command information
`

const howtoExample = `---
id: manage-multiple-agents
type: howto
title: How to Manage Multiple AI Agents Concurrently
description: A practical guide to running and coordinating multiple AI coding assistants
version: 1.0
tags:
  - howto
  - multi-agent
  - concurrent
---

# How to Manage Multiple AI Agents Concurrently

## Problem

You need to work on multiple coding tasks simultaneously, each requiring an AI assistant, without them interfering with each other's work.

## Solution

Claude Squad provides isolated workspaces using git worktrees and tmux sessions, allowing you to run up to 10 concurrent agents safely.

## Prerequisites

- Claude Squad installed
- A git repository
- Basic understanding of git branches

## Steps

### 1. Start Claude Squad

` + "```bash" + `
cd your-project
claude-squad
` + "```" + `

### 2. Create Multiple Sessions

Press 'n' repeatedly to create new sessions. Each session gets:
- An isolated git worktree
- A separate branch (claude/task-description-xxxxx)
- Its own tmux session

### 3. Switch Between Sessions

Use arrow keys or 'j'/'k' to navigate between sessions, then press Enter to attach to a session.

### 4. Monitor All Sessions

View all active sessions in the list view. Each shows:
- Session status (Running/Ready/Paused)
- Current task description
- Git branch name

### 5. Detach from a Session

Press 'Ctrl+b' then 'd' to detach from the current session without stopping it.

## Tips

- **Organize by task type**: Create sessions for specific features or bugs
- **Use descriptive names**: The session description becomes the branch name
- **Monitor progress**: Check diff stats to see what each agent is doing
- **Pause unused sessions**: Press 'p' to pause sessions and free resources

## Troubleshooting

### Problem: "Maximum 10 instances reached"

**Solution**: Pause or kill unused sessions first. Press 'k' on a session to kill it.

### Problem: Sessions are slow to respond

**Solution**: Reduce the number of active (non-paused) sessions. Paused sessions don't consume resources.

## Related Guides

- How to Debug Issues with Multiple Agents
- How to Merge Work from Multiple Sessions
`

const referenceExample = `---
id: cli-reference
type: reference
title: CLI Command Reference
description: Complete reference for all Claude Squad command-line options and flags
version: 1.0
tags:
  - reference
  - cli
  - commands
---

# CLI Command Reference

## Synopsis

` + "```bash" + `
claude-squad [flags]
claude-squad [command] [flags]
` + "```" + `

## Description

Claude Squad is a terminal application for managing multiple AI code assistants simultaneously using isolated workspaces.

## Commands

### Root Command

` + "```bash" + `
claude-squad [flags]
` + "```" + `

Starts the Claude Squad TUI application.

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --program | -p | string | "" | Program to run in new instances |
| --autoyes | -y | bool | false | Automatically accept prompts |

**Example:**

` + "```bash" + `
claude-squad --program "aider --model claude-3-5-sonnet-20241022"
` + "```" + `

### reset

` + "```bash" + `
claude-squad reset
` + "```" + `

Resets all stored instances and cleans up tmux sessions and git worktrees.

**Example:**

` + "```bash" + `
claude-squad reset
` + "```" + `

### debug

` + "```bash" + `
claude-squad debug
` + "```" + `

Prints debug information including config paths and current configuration.

### version

` + "```bash" + `
claude-squad version
` + "```" + `

Prints the version number and release URL.

## Keyboard Shortcuts

### In List View

| Key | Action |
|-----|--------|
| n | Create new instance |
| Enter | Attach to selected instance |
| k | Kill selected instance |
| p | Pause/Resume instance |
| d | Show diff for instance |
| c | Checkout branch |
| q | Quit application |
| ? | Show help |

### In Attached Session

| Key Combination | Action |
|----------------|--------|
| Ctrl+b, d | Detach from session |
| Ctrl+b, [ | Enter scroll mode |

## Configuration

Configuration file location: ` + "`~/.config/claude-squad/config.json`" + `

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| default_program | string | "claude-code" | Default program to run |
| auto_yes | bool | false | Auto-accept prompts |
| daemon_poll_interval | int | 1000 | Poll interval in ms |
| branch_prefix | string | "claude/" | Git branch prefix |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Not in git repository |

## Environment Variables

None.

## See Also

- Configuration Guide
- Troubleshooting Guide
- API Documentation
`

const explanationExample = `---
id: diataxis-framework
type: explanation
title: Understanding the Diataxis Documentation Framework
description: An in-depth explanation of the Diataxis framework and why it matters for documentation
version: 1.0
tags:
  - diataxis
  - documentation
  - concepts
---

# Understanding the Diataxis Documentation Framework

## Introduction

The Diataxis framework is a systematic approach to technical documentation that recognizes four distinct types of documentation, each serving a different purpose and addressing different user needs.

## Background

Created by Daniele Procida, Diataxis emerged from the observation that most documentation fails because it tries to serve multiple purposes simultaneously. By clearly separating documentation into four types, we can create more effective and user-friendly documentation.

## The Four Quadrants

Diataxis organizes documentation along two axes:

1. **Practical vs. Theoretical**
2. **Learning vs. Application**

This creates four distinct quadrants:

### 1. Tutorials (Learning + Practical)

Tutorials are **learning-oriented** and **practical**. They take a learner by the hand through a series of steps to complete a project, building confidence and competence.

**Characteristics:**
- Step-by-step instructions
- Focused on learning by doing
- Concrete, repeatable outcomes
- Friendly, encouraging tone

**Analogy**: Teaching a child to cook

### 2. How-To Guides (Application + Practical)

How-to guides are **task-oriented** and **practical**. They guide users through solving specific problems or accomplishing specific tasks.

**Characteristics:**
- Problem-focused
- Series of steps to achieve a goal
- Assumes basic knowledge
- Direct, concise language

**Analogy**: A recipe

### 3. Reference (Application + Theoretical)

Reference documentation is **information-oriented** and **theoretical**. It describes the machinery - how it works and how to use it.

**Characteristics:**
- Technical descriptions
- Accurate and complete
- Structured for lookup
- Neutral, factual tone

**Analogy**: An encyclopedia

### 4. Explanation (Learning + Theoretical)

Explanation is **understanding-oriented** and **theoretical**. It clarifies and illuminates topics, providing background and context.

**Characteristics:**
- Discusses concepts and ideas
- Provides context and background
- Makes connections
- Reflective, thoughtful tone

**Analogy**: A scholarly article

## Why This Matters

### 1. Clarity of Purpose

When documentation tries to teach, explain, inform, and guide all at once, it succeeds at none of these. Diataxis provides clear boundaries.

### 2. User-Centered Design

Different users need different things at different times:
- A newcomer needs tutorials
- An experienced user solving a problem needs how-to guides
- Someone looking up specifics needs reference
- Someone wanting deeper understanding needs explanation

### 3. Easier Maintenance

With clear categories, it's easier to:
- Identify gaps in documentation
- Know where new content belongs
- Keep each type consistent in style and structure

### 4. Better Searchability

Users can quickly navigate to the type of documentation they need, improving the overall user experience.

## Real-World Applications

### Software Documentation

Popular tools like Django and Stripe use Diataxis-inspired structures:
- Getting Started (Tutorial)
- Guides (How-To)
- API Reference (Reference)
- Architecture Overview (Explanation)

### Technical Writing

Technical writers use Diataxis to:
- Audit existing documentation
- Plan new documentation projects
- Train new team members
- Establish documentation standards

### Developer Experience

Development teams improve DX by:
- Ensuring all four types are covered
- Avoiding mixing types inappropriately
- Tailoring content to user journey stages

## Common Misconceptions

### Misconception 1: "All documentation should be comprehensive"

**Reality**: Each type should be complete **for its purpose**. A tutorial doesn't need to cover every edge case; that's what reference is for.

### Misconception 2: "One document type is enough"

**Reality**: Users need all four types at different stages. Missing any type creates gaps in the user experience.

### Misconception 3: "Diataxis is too rigid"

**Reality**: Diataxis is a framework, not a straitjacket. It provides structure while allowing flexibility in implementation.

## Further Reading

- [Diataxis Official Website](https://diataxis.fr/)
- "The Documentation System" by Daniele Procida
- Case studies from Django, Cloudflare, and Stripe
- Academic research on technical documentation effectiveness
`

func getDiataxisREADME() string {
	return `# Claude Squad Documentation

This directory contains documentation for Claude Squad, organized using the **Diataxis framework**.

## Documentation Structure

The documentation is organized into four categories:

### 📚 Tutorials (/tutorials/)
Learning-oriented guides that take you step-by-step through using Claude Squad.
Start here if you're new to Claude Squad.

### 🛠 How-To Guides (/howto/)
Task-oriented guides that show you how to solve specific problems.
Use these when you have a specific goal in mind.

### 📖 Reference (/reference/)
Technical descriptions and API documentation.
Use these to look up specific details.

### 💡 Explanation (/explanation/)
Understanding-oriented discussions of concepts and design decisions.
Read these to deepen your understanding of how and why Claude Squad works.

## Getting Started

1. **New users**: Start with the tutorials
2. **Specific task**: Check the how-to guides
3. **Looking up details**: Use the reference docs
4. **Want to understand more**: Read the explanations

## Building the Documentation

To generate the documentation website:

` + "```bash" + `
claude-squad docs generate
` + "```" + `

To validate documentation:

` + "```bash" + `
claude-squad docs validate
` + "```" + `

To see statistics:

` + "```bash" + `
claude-squad docs stats
` + "```" + `

## Contributing

When adding new documentation:
1. Determine which Diataxis type it belongs to
2. Place it in the appropriate directory
3. Follow the template for that type
4. Run validation before committing

## Learn More About Diataxis

Visit [diataxis.fr](https://diataxis.fr/) to learn more about the framework.
`
}
