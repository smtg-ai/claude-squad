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
			extension.GFM,            // GitHub Flavored Markdown
			extension.Table,          // Tables
			extension.Strikethrough,  // Strikethrough text
			extension.TaskList,       // Task lists
			extension.Linkify,        // Auto-link URLs
			extension.Footnote,       // Footnotes
			extension.DefinitionList, // Definition lists
			extension.Typographer,    // Smart quotes and dashes
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Hard line breaks
			html.WithXHTML(),     // XHTML output
			html.WithUnsafe(),    // Allow raw HTML (for advanced features)
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

	// Initialize metadata map (even if frontmatter is empty)
	metadata := make(map[string]interface{})

	if frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
			// Provide context about what failed
			preview := frontmatter
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w\nFrontmatter preview: %s", err, preview)
		}
	}

	parsed, err := mp.Parse(body)
	if err != nil {
		return nil, "", err
	}

	return metadata, parsed, nil
}

// ValidateDiataxisMetadata validates required Diataxis fields in frontmatter
func ValidateDiataxisMetadata(metadata map[string]interface{}) error {
	if metadata == nil {
		return fmt.Errorf("metadata is nil")
	}

	// Check required field: type
	typeVal, hasType := metadata["type"]
	if !hasType {
		return fmt.Errorf("missing required field 'type' in frontmatter")
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return fmt.Errorf("field 'type' must be a string, got %T", typeVal)
	}

	// Validate type is one of the valid Diataxis types
	validTypes := map[string]bool{
		"tutorial":    true,
		"howto":       true,
		"reference":   true,
		"explanation": true,
	}

	if !validTypes[typeStr] {
		return fmt.Errorf("invalid document type '%s', must be one of: tutorial, howto, reference, explanation", typeStr)
	}

	// Check required field: title
	titleVal, hasTitle := metadata["title"]
	if !hasTitle {
		return fmt.Errorf("missing required field 'title' in frontmatter")
	}

	if _, ok := titleVal.(string); !ok {
		return fmt.Errorf("field 'title' must be a string, got %T", titleVal)
	}

	// Description is recommended but not strictly required
	// Add warning if missing
	if _, hasDesc := metadata["description"]; !hasDesc {
		// Note: This is a warning, not an error
		// The validator will catch this
	}

	return nil
}

// extractFrontmatter extracts YAML frontmatter from markdown content
// Handles both Unix (\n) and Windows (\r\n) line endings
func extractFrontmatter(content string) (frontmatter, body string) {
	// Match YAML frontmatter (--- ... ---)
	// Updated regex to handle:
	// - Both \n and \r\n line endings
	// - Optional whitespace after delimiters
	// - Missing trailing delimiter (treat rest as body)
	re := regexp.MustCompile(`(?ms)^---\s*(?:\r?\n)(.*?)(?:\r?\n)---\s*(?:\r?\n|$)(.*)$`)
	matches := re.FindStringSubmatch(content)

	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
	}

	// Handle malformed frontmatter (opening --- without closing ---)
	// If content starts with --- but no closing, treat entire content as body
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		// Look for any content after opening ---
		lines := strings.Split(content, "\n")
		if len(lines) > 1 {
			// Malformed frontmatter detected - skip it
			return "", content
		}
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
	buf.WriteString(fmt.Sprintf("id: %s\n", quoteYAMLString(doc.ID)))
	buf.WriteString(fmt.Sprintf("type: %s\n", quoteYAMLString(string(doc.Type))))
	buf.WriteString(fmt.Sprintf("title: %s\n", quoteYAMLString(doc.Title)))
	buf.WriteString(fmt.Sprintf("description: %s\n", quoteYAMLString(doc.Description)))
	buf.WriteString(fmt.Sprintf("version: %s\n", quoteYAMLString(doc.Version)))

	if len(doc.Tags) > 0 {
		buf.WriteString("tags:\n")
		for _, tag := range doc.Tags {
			buf.WriteString(fmt.Sprintf("  - %s\n", quoteYAMLString(tag)))
		}
	}

	if len(doc.RelatedDocs) > 0 {
		buf.WriteString("related:\n")
		for _, related := range doc.RelatedDocs {
			buf.WriteString(fmt.Sprintf("  - %s\n", quoteYAMLString(related)))
		}
	}

	buf.WriteString("---")
	return buf.String()
}

// quoteYAMLString quotes a string for safe YAML output if needed
// Handles special characters that require quoting in YAML
func quoteYAMLString(s string) string {
	if s == "" {
		return `""`
	}

	// Characters that require quoting in YAML
	needsQuoting := strings.ContainsAny(s, ":{}[]!#|>&*@`'\"\n\r\t")

	// Also quote if starts with special chars or looks like a number/bool
	if !needsQuoting {
		trimmed := strings.TrimSpace(s)
		needsQuoting = strings.HasPrefix(trimmed, "-") ||
			strings.HasPrefix(trimmed, "?") ||
			trimmed == "true" ||
			trimmed == "false" ||
			trimmed == "null" ||
			regexp.MustCompile(`^[0-9.]+$`).MatchString(trimmed)
	}

	if needsQuoting {
		// Escape double quotes and backslashes
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}

	return s
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

// MarkdownSection represents a section of markdown with heading and content
type MarkdownSection struct {
	Level   int    // Heading level (1-6)
	Title   string // Section title
	Anchor  string // URL-safe anchor
	Content string // Section content (excluding the heading itself)
}

// SectionExtractor extracts sections from markdown content
type SectionExtractor struct{}

// NewSectionExtractor creates a new section extractor
func NewSectionExtractor() *SectionExtractor {
	return &SectionExtractor{}
}

// ExtractSections extracts all sections from markdown content
// Sections are defined by headings (# through ######)
func (se *SectionExtractor) ExtractSections(content string) []MarkdownSection {
	var sections []MarkdownSection

	// Split content into lines
	lines := strings.Split(content, "\n")

	var currentSection *MarkdownSection
	var contentLines []string

	for i, line := range lines {
		// Check if line is a heading
		headingMatch := regexp.MustCompile(`^(#{1,6})\s+(.+)$`).FindStringSubmatch(line)

		if headingMatch != nil {
			// Save previous section if exists
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
				sections = append(sections, *currentSection)
			}

			// Start new section
			level := len(headingMatch[1])
			title := strings.TrimSpace(headingMatch[2])
			anchor := generateAnchor(title)

			currentSection = &MarkdownSection{
				Level:  level,
				Title:  title,
				Anchor: anchor,
			}
			contentLines = []string{}
		} else if currentSection != nil {
			// Add line to current section content
			contentLines = append(contentLines, line)
		} else if i == 0 && line != "" {
			// Content before first heading
			contentLines = append(contentLines, line)
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
		sections = append(sections, *currentSection)
	}

	return sections
}

// ExtractSectionsByLevel extracts sections of a specific heading level
func (se *SectionExtractor) ExtractSectionsByLevel(content string, level int) []MarkdownSection {
	allSections := se.ExtractSections(content)
	var filtered []MarkdownSection

	for _, section := range allSections {
		if section.Level == level {
			filtered = append(filtered, section)
		}
	}

	return filtered
}

// FindSection finds a section by title (case-insensitive)
func (se *SectionExtractor) FindSection(content string, title string) *MarkdownSection {
	sections := se.ExtractSections(content)
	titleLower := strings.ToLower(title)

	for _, section := range sections {
		if strings.ToLower(section.Title) == titleLower {
			return &section
		}
	}

	return nil
}

// generateAnchor creates a URL-safe anchor from a heading
func generateAnchor(heading string) string {
	anchor := strings.ToLower(heading)
	anchor = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(anchor, "-")
	anchor = strings.Trim(anchor, "-")
	return anchor
}
