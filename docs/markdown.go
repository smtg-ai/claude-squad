package docs

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// MarkdownParser handles advanced markdown parsing with goldmark
type MarkdownParser struct {
	markdown goldmark.Markdown
}

// NewMarkdownParser creates a new markdown parser with advanced features
func NewMarkdownParser() *MarkdownParser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,              // GitHub Flavored Markdown
			extension.Table,            // Tables
			extension.Strikethrough,    // Strikethrough text
			extension.TaskList,         // Task lists
			extension.Linkify,          // Auto-link URLs
			extension.Footnote,         // Footnotes
			extension.DefinitionList,   // Definition lists
			extension.Typographer,      // Smart quotes and dashes
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),       // Hard line breaks
			html.WithXHTML(),           // XHTML output
			html.WithUnsafe(),          // Allow raw HTML (for advanced features)
		),
	)

	return &MarkdownParser{
		markdown: md,
	}
}

// Parse parses markdown content and returns HTML
func (mp *MarkdownParser) Parse(content string) (string, error) {
	var buf bytes.Buffer

	if err := mp.markdown.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("failed to parse markdown: %w", err)
	}

	return buf.String(), nil
}

// ParseWithFrontmatter parses markdown with YAML frontmatter
func (mp *MarkdownParser) ParseWithFrontmatter(content string) (map[string]interface{}, string, error) {
	frontmatter, body := extractFrontmatter(content)

	var metadata map[string]interface{}
	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
			return nil, "", fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	parsed, err := mp.Parse(body)
	if err != nil {
		return nil, "", err
	}

	return metadata, parsed, nil
}

// extractFrontmatter extracts YAML frontmatter from markdown content
func extractFrontmatter(content string) (frontmatter, body string) {
	// Match YAML frontmatter (--- ... ---)
	re := regexp.MustCompile(`(?ms)^---\s*\n(.*?)\n---\s*\n(.*)$`)
	matches := re.FindStringSubmatch(content)

	if len(matches) == 3 {
		return matches[1], matches[2]
	}

	return "", content
}

// MarkdownGenerator generates markdown content from structured data
type MarkdownGenerator struct {
	templates map[DocType]string
}

// NewMarkdownGenerator creates a new markdown generator
func NewMarkdownGenerator() *MarkdownGenerator {
	return &MarkdownGenerator{
		templates: map[DocType]string{
			Tutorial:    tutorialTemplate,
			HowTo:       howtoTemplate,
			Reference:   referenceTemplate,
			Explanation: explanationTemplate,
		},
	}
}

// Generate generates markdown content for a document
func (mg *MarkdownGenerator) Generate(doc *Document) (string, error) {
	template, exists := mg.templates[doc.Type]
	if !exists {
		return "", fmt.Errorf("no template found for document type: %s", doc.Type)
	}

	// Replace placeholders with actual values
	content := strings.ReplaceAll(template, "{{title}}", doc.Title)
	content = strings.ReplaceAll(content, "{{description}}", doc.Description)
	content = strings.ReplaceAll(content, "{{content}}", doc.Content)
	content = strings.ReplaceAll(content, "{{version}}", doc.Version)

	// Add frontmatter
	frontmatter := generateFrontmatter(doc)
	return frontmatter + "\n\n" + content, nil
}

// generateFrontmatter creates YAML frontmatter for a document
func generateFrontmatter(doc *Document) string {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("id: %s\n", doc.ID))
	buf.WriteString(fmt.Sprintf("type: %s\n", doc.Type))
	buf.WriteString(fmt.Sprintf("title: %s\n", doc.Title))
	buf.WriteString(fmt.Sprintf("description: %s\n", doc.Description))
	buf.WriteString(fmt.Sprintf("version: %s\n", doc.Version))

	if len(doc.Tags) > 0 {
		buf.WriteString("tags:\n")
		for _, tag := range doc.Tags {
			buf.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
	}

	if len(doc.RelatedDocs) > 0 {
		buf.WriteString("related:\n")
		for _, related := range doc.RelatedDocs {
			buf.WriteString(fmt.Sprintf("  - %s\n", related))
		}
	}

	buf.WriteString("---")
	return buf.String()
}

// Document templates for each Diataxis type
const tutorialTemplate = `# {{title}}

{{description}}

## Learning Objectives

By the end of this tutorial, you will be able to:
- Understand the basics
- Complete practical exercises
- Build confidence with hands-on experience

## Prerequisites

Before starting this tutorial, you should have:
- Basic knowledge of the tools
- A working development environment

## Step-by-Step Guide

{{content}}

## What You've Learned

In this tutorial, you've learned:
- Key concepts and techniques
- Practical application
- Next steps for further learning

## Next Steps

Continue your learning journey with:
- Related tutorials
- How-to guides for specific tasks
- Reference documentation for detailed information
`

const howtoTemplate = `# {{title}}

{{description}}

## Problem

This guide shows you how to accomplish a specific task efficiently.

## Solution

{{content}}

## Prerequisites

- Required tools and setup
- Necessary permissions or access

## Steps

Follow these steps to complete the task:

1. **Step 1**: First action
2. **Step 2**: Next action
3. **Step 3**: Final verification

## Troubleshooting

Common issues and solutions:
- **Problem**: Description
  - **Solution**: Fix

## Related Guides

- Related how-to guides
- Reference documentation
`

const referenceTemplate = "# {{title}}\n\n" +
	"{{description}}\n\n" +
	"## Overview\n\n" +
	"Technical reference for {{title}}.\n\n" +
	"## API Reference\n\n" +
	"{{content}}\n\n" +
	"## Parameters\n\n" +
	"| Parameter | Type | Required | Description |\n" +
	"|-----------|------|----------|-------------|\n" +
	"| name      | string | Yes    | Parameter description |\n\n" +
	"## Return Values\n\n" +
	"| Value | Type | Description |\n" +
	"|-------|------|-------------|\n" +
	"| result | type | Return value description |\n\n" +
	"## Examples\n\n" +
	"```go\n" +
	"// Example usage\n" +
	"example()\n" +
	"```\n\n" +
	"## Error Codes\n\n" +
	"| Code | Description |\n" +
	"|------|-------------|\n" +
	"| E001 | Error description |\n\n" +
	"## See Also\n\n" +
	"- Related references\n" +
	"- Additional documentation\n"

const explanationTemplate = `# {{title}}

{{description}}

## Introduction

{{content}}

## Background

Understanding the context and history behind this concept.

## Key Concepts

### Concept 1

Detailed explanation of the first key concept.

### Concept 2

Detailed explanation of the second key concept.

## Why This Matters

The importance and relevance of this topic:
- Benefit 1
- Benefit 2
- Benefit 3

## Real-World Applications

How this concept applies in practice:
- Use case 1
- Use case 2
- Use case 3

## Common Misconceptions

Clarifying common misunderstandings:
1. **Misconception**: Incorrect belief
   - **Reality**: Actual truth

## Further Reading

- Academic papers
- Blog posts
- Books and resources
`

// TableOfContentsGenerator generates a table of contents from markdown
type TableOfContentsGenerator struct{}

// NewTableOfContentsGenerator creates a new TOC generator
func NewTableOfContentsGenerator() *TableOfContentsGenerator {
	return &TableOfContentsGenerator{}
}

// Generate creates a table of contents from markdown content
func (toc *TableOfContentsGenerator) Generate(content string) string {
	var buf bytes.Buffer
	buf.WriteString("## Table of Contents\n\n")

	// Extract headings (h2, h3, h4)
	re := regexp.MustCompile(`(?m)^(#{2,4})\s+(.+)$`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		level := len(match[1]) - 2 // h2=0, h3=1, h4=2
		heading := match[2]
		anchor := strings.ToLower(heading)
		anchor = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(anchor, "-")
		anchor = strings.Trim(anchor, "-")

		indent := strings.Repeat("  ", level)
		buf.WriteString(fmt.Sprintf("%s- [%s](#%s)\n", indent, heading, anchor))
	}

	return buf.String()
}
