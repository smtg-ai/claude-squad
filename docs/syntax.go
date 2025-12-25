package docs

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// SyntaxHighlighter handles syntax highlighting for code blocks
type SyntaxHighlighter struct {
	formatter *html.Formatter
	style     *chroma.Style
}

// NewSyntaxHighlighter creates a new syntax highlighter
func NewSyntaxHighlighter() *SyntaxHighlighter {
	return &SyntaxHighlighter{
		formatter: html.New(
			html.WithClasses(true),        // Use CSS classes
			html.WithLineNumbers(true),    // Show line numbers
			html.LineNumbersInTable(true), // Put line numbers in table
			html.TabWidth(4),              // Tab width
		),
		style: styles.Get("monokai"), // Use monokai style
	}
}

// Highlight applies syntax highlighting to HTML content
func (sh *SyntaxHighlighter) Highlight(content string) (string, error) {
	// Find all code blocks in HTML
	re := regexp.MustCompile(`(?s)<code class="language-(\w+)">(.*?)</code>`)

	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract language and code
		matches := re.FindStringSubmatch(match)
		if len(matches) != 3 {
			return match
		}

		language := matches[1]
		code := matches[2]

		// Unescape HTML entities
		code = unescapeHTML(code)

		// Highlight the code
		highlighted, err := sh.highlightCode(code, language)
		if err != nil {
			return match // Return original on error
		}

		return highlighted
	})

	return result, nil
}

// highlightCode highlights a single code block
func (sh *SyntaxHighlighter) highlightCode(code, language string) (string, error) {
	// Get lexer for language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback // Use fallback if language not found
	}
	lexer = chroma.Coalesce(lexer)

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("failed to tokenize: %w", err)
	}

	// Format the tokens
	var buf bytes.Buffer
	err = sh.formatter.Format(&buf, sh.style, iterator)
	if err != nil {
		return "", fmt.Errorf("failed to format: %w", err)
	}

	return buf.String(), nil
}

// HighlightCodeBlock highlights a standalone code block
func (sh *SyntaxHighlighter) HighlightCodeBlock(code, language string) (string, error) {
	return sh.highlightCode(code, language)
}

// GenerateCSS generates CSS for syntax highlighting
func (sh *SyntaxHighlighter) GenerateCSS() (string, error) {
	var buf bytes.Buffer
	err := sh.formatter.WriteCSS(&buf, sh.style)
	if err != nil {
		return "", fmt.Errorf("failed to generate CSS: %w", err)
	}
	return buf.String(), nil
}

// unescapeHTML unescapes basic HTML entities
func unescapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", "\"",
		"&#39;", "'",
	)
	return replacer.Replace(s)
}

// CodeExtractor extracts code examples from content
type CodeExtractor struct {
	codeBlockRe *regexp.Regexp
}

// NewCodeExtractor creates a new code extractor
func NewCodeExtractor() *CodeExtractor {
	return &CodeExtractor{
		codeBlockRe: regexp.MustCompile("(?s)```(\\w+)\\n(.*?)```"),
	}
}

// Extract extracts all code blocks from content
func (ce *CodeExtractor) Extract(content string) []CodeExample {
	matches := ce.codeBlockRe.FindAllStringSubmatch(content, -1)

	examples := make([]CodeExample, 0, len(matches))
	for _, match := range matches {
		if len(match) == 3 {
			examples = append(examples, CodeExample{
				Language: match[1],
				Code:     strings.TrimSpace(match[2]),
			})
		}
	}

	return examples
}

// CodeExample represents a single code example
type CodeExample struct {
	Language    string `json:"language"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Runnable    bool   `json:"runnable"`
}

// ReferenceExtractor extracts cross-references from content
type ReferenceExtractor struct {
	refRe *regexp.Regexp
}

// NewReferenceExtractor creates a new reference extractor
func NewReferenceExtractor() *ReferenceExtractor {
	return &ReferenceExtractor{
		refRe: regexp.MustCompile(`\[([^\]]+)\]\(#([^)]+)\)`),
	}
}

// Extract extracts all cross-references from content
func (re *ReferenceExtractor) Extract(content string) []string {
	matches := re.refRe.FindAllStringSubmatch(content, -1)

	refs := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) == 3 {
			ref := match[2]
			if !seen[ref] {
				refs = append(refs, ref)
				seen[ref] = true
			}
		}
	}

	return refs
}

// QualityCalculator calculates document quality scores
type QualityCalculator struct{}

// NewQualityCalculator creates a new quality calculator
func NewQualityCalculator() *QualityCalculator {
	return &QualityCalculator{}
}

// Calculate calculates a quality score for a document
func (qc *QualityCalculator) Calculate(doc *Document) float64 {
	score := 0.0

	// Content length (0-20 points)
	contentLength := len(doc.Content)
	if contentLength > 5000 {
		score += 20
	} else if contentLength > 2000 {
		score += 15
	} else if contentLength > 500 {
		score += 10
	} else if contentLength > 100 {
		score += 5
	}

	// Has description (0-10 points)
	if len(doc.Description) > 50 {
		score += 10
	} else if len(doc.Description) > 0 {
		score += 5
	}

	// Has metadata (0-10 points)
	if len(doc.Metadata) > 5 {
		score += 10
	} else if len(doc.Metadata) > 0 {
		score += 5
	}

	// Has tags (0-10 points)
	if len(doc.Tags) > 3 {
		score += 10
	} else if len(doc.Tags) > 0 {
		score += 5
	}

	// Has code examples (0-15 points)
	codeExamples := qc.countCodeBlocks(doc.Content)
	if codeExamples > 5 {
		score += 15
	} else if codeExamples > 2 {
		score += 10
	} else if codeExamples > 0 {
		score += 5
	}

	// Has cross-references (0-10 points)
	if len(doc.RelatedDocs) > 3 {
		score += 10
	} else if len(doc.RelatedDocs) > 0 {
		score += 5
	}

	// Has headings (0-10 points)
	headings := qc.countHeadings(doc.Content)
	if headings > 5 {
		score += 10
	} else if headings > 2 {
		score += 5
	}

	// Has version (0-5 points)
	if doc.Version != "" {
		score += 5
	}

	// Has author (0-5 points)
	if doc.Author != "" {
		score += 5
	}

	// Bonus: Well-structured for doc type (0-5 points)
	if qc.isWellStructured(doc) {
		score += 5
	}

	return score
}

// countCodeBlocks counts the number of code blocks in content
func (qc *QualityCalculator) countCodeBlocks(content string) int {
	return strings.Count(content, "```")
}

// countHeadings counts the number of headings in content
func (qc *QualityCalculator) countHeadings(content string) int {
	re := regexp.MustCompile(`(?m)^#{1,6}\s+.+$`)
	return len(re.FindAllString(content, -1))
}

// isWellStructured checks if document follows Diataxis structure
func (qc *QualityCalculator) isWellStructured(doc *Document) bool {
	content := strings.ToLower(doc.Content)

	switch doc.Type {
	case Tutorial:
		return strings.Contains(content, "step") || strings.Contains(content, "objective")
	case HowTo:
		return strings.Contains(content, "problem") || strings.Contains(content, "solution")
	case Reference:
		return strings.Contains(content, "parameter") || strings.Contains(content, "api")
	case Explanation:
		return strings.Contains(content, "concept") || strings.Contains(content, "background")
	default:
		return false
	}
}

// HTMLGenerator generates HTML from processed documents
type HTMLGenerator struct {
	templates map[DocType]string
}

// NewHTMLGenerator creates a new HTML generator
func NewHTMLGenerator() *HTMLGenerator {
	return &HTMLGenerator{
		templates: make(map[DocType]string),
	}
}

// Generate generates HTML for a document
func (hg *HTMLGenerator) Generate(doc *Document) (string, error) {
	// For now, return the processed content
	// In a full implementation, this would use HTML templates
	return doc.ProcessedContent, nil
}
