package docs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DocumentGenerator generates complete documentation sites
type DocumentGenerator struct {
	framework *DiataxisFramework
	builder   *SiteBuilder
}

// NewDocumentGenerator creates a new document generator
func NewDocumentGenerator(framework *DiataxisFramework) *DocumentGenerator {
	return &DocumentGenerator{
		framework: framework,
		builder:   NewSiteBuilder(framework.config),
	}
}

// Generate generates the complete documentation site
func (dg *DocumentGenerator) Generate(ctx context.Context) error {
	startTime := time.Now()

	// Step 1: Process all documents
	if err := dg.framework.ProcessAllDocuments(ctx); err != nil {
		return fmt.Errorf("failed to process documents: %w", err)
	}

	// Step 2: Validate all documents
	report, err := dg.framework.ValidateAllDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to validate documents: %w", err)
	}

	// Check if there are critical errors
	if report.FailedCount > 0 {
		return fmt.Errorf("validation failed: %d documents have errors", report.FailedCount)
	}

	// Step 3: Build the documentation site
	if err := dg.builder.Build(ctx, dg.framework.GetAllDocuments()); err != nil {
		return fmt.Errorf("failed to build site: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("Documentation generated successfully in %v\n", duration)

	return nil
}

// SiteBuilder builds the documentation site
type SiteBuilder struct {
	config        *FrameworkConfig
	outputDir     string
	templateEngine *TemplateEngine
}

// NewSiteBuilder creates a new site builder
func NewSiteBuilder(config *FrameworkConfig) *SiteBuilder {
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "./docs-output"
	}

	return &SiteBuilder{
		config:        config,
		outputDir:     outputDir,
		templateEngine: NewTemplateEngine(config.TemplateDir),
	}
}

// Build builds the documentation site
func (sb *SiteBuilder) Build(ctx context.Context, docs []*Document) error {
	// Create output directory
	if err := os.MkdirAll(sb.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build concurrently with worker pool
	semaphore := make(chan struct{}, MaxConcurrentWorkers)
	errChan := make(chan error, len(docs))
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
				errChan <- ctx.Err()
				return
			}

			// Build document page
			if err := sb.buildDocumentPage(d); err != nil {
				errChan <- fmt.Errorf("failed to build page for %s: %w", d.ID, err)
			}
		}(doc)
	}

	// Wait for all builds to complete
	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("build failed with %d errors: %v", len(errs), errs[0])
	}

	// Build index page
	if err := sb.buildIndexPage(docs); err != nil {
		return fmt.Errorf("failed to build index page: %w", err)
	}

	// Copy static assets
	if err := sb.copyAssets(); err != nil {
		return fmt.Errorf("failed to copy assets: %w", err)
	}

	return nil
}

// buildDocumentPage builds a single document page
func (sb *SiteBuilder) buildDocumentPage(doc *Document) error {
	// Create type-specific directory
	typeDir := filepath.Join(sb.outputDir, string(doc.Type))
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return err
	}

	// Generate HTML
	html, err := sb.templateEngine.RenderDocument(doc)
	if err != nil {
		return err
	}

	// Write to file
	filename := filepath.Join(typeDir, doc.ID+".html")
	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		return err
	}

	return nil
}

// buildIndexPage builds the index page
func (sb *SiteBuilder) buildIndexPage(docs []*Document) error {
	// Group documents by type
	docsByType := make(map[DocType][]*Document)
	for _, doc := range docs {
		docsByType[doc.Type] = append(docsByType[doc.Type], doc)
	}

	// Generate index HTML
	html, err := sb.templateEngine.RenderIndex(docsByType)
	if err != nil {
		return err
	}

	// Write to file
	filename := filepath.Join(sb.outputDir, "index.html")
	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		return err
	}

	return nil
}

// copyAssets copies static assets to output directory
func (sb *SiteBuilder) copyAssets() error {
	// Create assets directory
	assetsDir := filepath.Join(sb.outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	// Generate and write CSS
	highlighter := NewSyntaxHighlighter()
	css, err := highlighter.GenerateCSS()
	if err != nil {
		return err
	}

	cssPath := filepath.Join(assetsDir, "syntax.css")
	if err := os.WriteFile(cssPath, []byte(css), 0644); err != nil {
		return err
	}

	// Write main CSS
	mainCSS := generateMainCSS()
	mainCSSPath := filepath.Join(assetsDir, "main.css")
	if err := os.WriteFile(mainCSSPath, []byte(mainCSS), 0644); err != nil {
		return err
	}

	return nil
}

// TemplateEngine handles HTML template rendering
type TemplateEngine struct {
	templateDir string
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine(templateDir string) *TemplateEngine {
	if templateDir == "" {
		templateDir = "./templates"
	}

	return &TemplateEngine{
		templateDir: templateDir,
	}
}

// RenderDocument renders a document to HTML
func (te *TemplateEngine) RenderDocument(doc *Document) (string, error) {
	// Use built-in template
	template := documentHTMLTemplate

	// Replace placeholders
	html := template
	html = replaceAll(html, "{{title}}", doc.Title)
	html = replaceAll(html, "{{type}}", string(doc.Type))
	html = replaceAll(html, "{{description}}", doc.Description)
	html = replaceAll(html, "{{content}}", doc.ProcessedContent)
	html = replaceAll(html, "{{version}}", doc.Version)
	html = replaceAll(html, "{{updated}}", doc.UpdatedAt.Format("2006-01-02"))

	return html, nil
}

// RenderIndex renders the index page
func (te *TemplateEngine) RenderIndex(docsByType map[DocType][]*Document) (string, error) {
	// Use built-in template
	template := indexHTMLTemplate

	// Build document lists for each type
	var sectionsBuilder strings.Builder

	types := []DocType{Tutorial, HowTo, Reference, Explanation}
	typeNames := map[DocType]string{
		Tutorial:    "Tutorials",
		HowTo:       "How-To Guides",
		Reference:   "Reference",
		Explanation: "Explanation",
	}

	for _, docType := range types {
		docs := docsByType[docType]
		if len(docs) == 0 {
			continue
		}

		fmt.Fprintf(&sectionsBuilder, "<section class=\"doc-section %s\">\n", docType)
		fmt.Fprintf(&sectionsBuilder, "<h2>%s</h2>\n", typeNames[docType])
		sectionsBuilder.WriteString("<ul class=\"doc-list\">\n")

		for _, doc := range docs {
			fmt.Fprintf(&sectionsBuilder, "<li><a href=\"%s/%s.html\">%s</a> - %s</li>\n",
				docType, doc.ID, doc.Title, doc.Description)
		}

		sectionsBuilder.WriteString("</ul>\n</section>\n")
	}

	html := replaceAll(template, "{{sections}}", sectionsBuilder.String())

	return html, nil
}

// Helper function to replace all occurrences
func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := indexOf(s, old)
		if i == -1 {
			return result + s
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
}

// Helper function to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// HTML Templates
const documentHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{title}} | Claude Squad Documentation</title>
    <meta name="description" content="{{description}}">
    <link rel="stylesheet" href="../assets/main.css">
    <link rel="stylesheet" href="../assets/syntax.css">
</head>
<body>
    <header>
        <div class="container">
            <h1><a href="../index.html">Claude Squad Docs</a></h1>
            <nav>
                <a href="../tutorial/index.html">Tutorials</a>
                <a href="../howto/index.html">How-To</a>
                <a href="../reference/index.html">Reference</a>
                <a href="../explanation/index.html">Explanation</a>
            </nav>
        </div>
    </header>
    <main class="container">
        <article class="document {{type}}">
            <header class="doc-header">
                <span class="doc-type">{{type}}</span>
                <h1>{{title}}</h1>
                <p class="doc-description">{{description}}</p>
                <div class="doc-meta">
                    <span>Version: {{version}}</span>
                    <span>Updated: {{updated}}</span>
                </div>
            </header>
            <div class="doc-content">
                {{content}}
            </div>
        </article>
    </main>
    <footer>
        <div class="container">
            <p>&copy; 2025 Claude Squad - Diataxis Documentation Framework</p>
        </div>
    </footer>
</body>
</html>`

const indexHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Claude Squad Documentation</title>
    <link rel="stylesheet" href="assets/main.css">
</head>
<body>
    <header>
        <div class="container">
            <h1>Claude Squad Documentation</h1>
            <p class="subtitle">Manage multiple AI agents with ease</p>
        </div>
    </header>
    <main class="container">
        <div class="diataxis-grid">
            <div class="quadrant tutorial">
                <h2>Tutorials</h2>
                <p>Learning-oriented guides to help you get started</p>
            </div>
            <div class="quadrant howto">
                <h2>How-To Guides</h2>
                <p>Task-oriented guides to solve specific problems</p>
            </div>
            <div class="quadrant explanation">
                <h2>Explanation</h2>
                <p>Understanding-oriented discussions of key topics</p>
            </div>
            <div class="quadrant reference">
                <h2>Reference</h2>
                <p>Information-oriented technical descriptions</p>
            </div>
        </div>
        <div class="documentation">
            {{sections}}
        </div>
    </main>
    <footer>
        <div class="container">
            <p>&copy; 2025 Claude Squad - Diataxis Documentation Framework</p>
        </div>
    </footer>
</body>
</html>`

func generateMainCSS() string {
	return `/* Diataxis Documentation CSS */
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
    line-height: 1.6;
    color: #333;
    background: #f8f9fa;
}

.container { max-width: 1200px; margin: 0 auto; padding: 0 20px; }

header {
    background: #2c3e50;
    color: white;
    padding: 20px 0;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

header h1 { font-size: 24px; margin-bottom: 10px; }
header h1 a { color: white; text-decoration: none; }
header nav a { color: white; margin-right: 20px; text-decoration: none; opacity: 0.9; }
header nav a:hover { opacity: 1; text-decoration: underline; }

.subtitle { opacity: 0.9; margin-top: 10px; }

main { padding: 40px 0; min-height: calc(100vh - 200px); }

.diataxis-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
    margin-bottom: 40px;
}

.quadrant {
    padding: 30px;
    border-radius: 8px;
    box-shadow: 0 2px 8px rgba(0,0,0,0.1);
}

.quadrant.tutorial { background: #e3f2fd; border-left: 4px solid #2196f3; }
.quadrant.howto { background: #f3e5f5; border-left: 4px solid #9c27b0; }
.quadrant.explanation { background: #fff3e0; border-left: 4px solid #ff9800; }
.quadrant.reference { background: #e8f5e9; border-left: 4px solid #4caf50; }

.quadrant h2 { margin-bottom: 10px; color: #2c3e50; }
.quadrant p { color: #666; }

.doc-section { margin-bottom: 40px; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.doc-section h2 { color: #2c3e50; margin-bottom: 20px; padding-bottom: 10px; border-bottom: 2px solid #e0e0e0; }
.doc-list { list-style: none; }
.doc-list li { padding: 10px 0; border-bottom: 1px solid #f0f0f0; }
.doc-list li:last-child { border-bottom: none; }
.doc-list a { color: #2196f3; text-decoration: none; font-weight: 500; }
.doc-list a:hover { text-decoration: underline; }

.document { background: white; padding: 40px; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
.doc-header { margin-bottom: 30px; }
.doc-type { display: inline-block; padding: 4px 12px; border-radius: 4px; font-size: 12px; font-weight: 600; text-transform: uppercase; margin-bottom: 10px; }
.document.tutorial .doc-type { background: #e3f2fd; color: #1976d2; }
.document.howto .doc-type { background: #f3e5f5; color: #7b1fa2; }
.document.explanation .doc-type { background: #fff3e0; color: #f57c00; }
.document.reference .doc-type { background: #e8f5e9; color: #388e3c; }

.doc-header h1 { font-size: 32px; color: #2c3e50; margin-bottom: 10px; }
.doc-description { font-size: 18px; color: #666; margin-bottom: 15px; }
.doc-meta { color: #999; font-size: 14px; }
.doc-meta span { margin-right: 20px; }

.doc-content { font-size: 16px; line-height: 1.8; }
.doc-content h2 { margin-top: 30px; margin-bottom: 15px; color: #2c3e50; }
.doc-content h3 { margin-top: 20px; margin-bottom: 10px; color: #34495e; }
.doc-content code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; font-family: 'Monaco', 'Courier New', monospace; font-size: 14px; }
.doc-content pre { background: #2c3e50; color: #ecf0f1; padding: 20px; border-radius: 6px; overflow-x: auto; margin: 20px 0; }
.doc-content pre code { background: none; padding: 0; color: inherit; }
.doc-content ul, .doc-content ol { margin: 15px 0 15px 30px; }
.doc-content li { margin-bottom: 8px; }
.doc-content a { color: #2196f3; text-decoration: none; }
.doc-content a:hover { text-decoration: underline; }
.doc-content table { width: 100%; border-collapse: collapse; margin: 20px 0; }
.doc-content th, .doc-content td { padding: 12px; text-align: left; border: 1px solid #ddd; }
.doc-content th { background: #f8f9fa; font-weight: 600; }

footer {
    background: #2c3e50;
    color: white;
    padding: 20px 0;
    text-align: center;
    margin-top: 40px;
}

footer p { opacity: 0.9; }

@media (max-width: 768px) {
    .diataxis-grid { grid-template-columns: 1fr; }
    .document { padding: 20px; }
    .doc-header h1 { font-size: 24px; }
}
`
}
