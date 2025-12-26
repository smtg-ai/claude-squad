package docs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DocType represents the four Diataxis documentation types.
// The Diataxis framework categorizes documentation into four distinct types,
// each serving a different purpose and audience need:
// - Tutorial: Learning-oriented, takes the reader through a series of steps
// - HowTo: Goal-oriented, guides the reader through solving a specific problem
// - Reference: Information-oriented, technical descriptions of how it works
// - Explanation: Understanding-oriented, clarifies and illuminates a particular topic
type DocType string

const (
	// Tutorial represents learning-oriented documentation that takes the reader
	// through a series of steps to complete a project. Tutorials are lessons
	// that teach by doing, helping newcomers get started.
	Tutorial DocType = "tutorial"

	// HowTo represents goal-oriented documentation that guides the reader through
	// solving a specific real-world problem. How-to guides are directions that
	// take the reader through steps to achieve a specific end.
	HowTo DocType = "howto"

	// Reference represents information-oriented documentation that provides
	// technical descriptions of the machinery and how to operate it.
	// Reference guides are technical descriptions of APIs, functions, and features.
	Reference DocType = "reference"

	// Explanation represents understanding-oriented documentation that clarifies
	// and illuminates a particular topic. Explanations are discussions that
	// deepen the reader's understanding of a subject.
	Explanation DocType = "explanation"
)

// MaxConcurrentWorkers defines the maximum number of concurrent documentation processors
const MaxConcurrentWorkers = 10

// Document represents a single documentation unit in the Diataxis framework
type Document struct {
	ID          string                 `json:"id"`
	Type        DocType                `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Content     string                 `json:"content"`
	FilePath    string                 `json:"file_path"`
	Metadata    map[string]interface{} `json:"metadata"`
	Tags        []string               `json:"tags"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Version     string                 `json:"version"`
	Author      string                 `json:"author,omitempty"`

	// Cross-references and relationships
	RelatedDocs   []string `json:"related_docs,omitempty"`
	Prerequisites []string `json:"prerequisites,omitempty"`

	// Validation and quality metrics
	ValidationStatus ValidationStatus `json:"validation_status"`
	QualityScore     float64          `json:"quality_score"`

	// Processed content (HTML, syntax highlighted, etc.)
	ProcessedContent string `json:"processed_content,omitempty"`
}

// ValidationStatus represents the validation state of a document
type ValidationStatus string

const (
	ValidationPending  ValidationStatus = "pending"
	ValidationPassed   ValidationStatus = "passed"
	ValidationFailed   ValidationStatus = "failed"
	ValidationWarnings ValidationStatus = "warnings"
)

// DiataxisFramework manages the entire documentation system
type DiataxisFramework struct {
	documents  map[string]*Document
	docsByType map[DocType][]*Document
	processor  *ConcurrentProcessor
	validator  *DocumentValidator
	generator  *DocumentGenerator
	mu         sync.RWMutex

	// Configuration
	config *FrameworkConfig
}

// FrameworkConfig holds configuration for the Diataxis framework
type FrameworkConfig struct {
	MaxConcurrentWorkers     int
	EnableSyntaxHighlight    bool
	EnableCrossRefValidation bool
	EnableMetrics            bool
	OutputFormat             string // html, markdown, json
	TemplateDir              string
	OutputDir                string
}

// NewDiataxisFramework creates a new Diataxis documentation framework
func NewDiataxisFramework(config *FrameworkConfig) *DiataxisFramework {
	if config == nil {
		config = &FrameworkConfig{
			MaxConcurrentWorkers:     MaxConcurrentWorkers,
			EnableSyntaxHighlight:    true,
			EnableCrossRefValidation: true,
			EnableMetrics:            true,
			OutputFormat:             "html",
		}
	}

	fw := &DiataxisFramework{
		documents:  make(map[string]*Document),
		docsByType: make(map[DocType][]*Document),
		config:     config,
	}

	fw.processor = NewConcurrentProcessor(config.MaxConcurrentWorkers)
	fw.validator = NewDocumentValidator(fw)
	fw.generator = NewDocumentGenerator(fw)

	return fw
}

// AddDocument adds a document to the framework
func (fw *DiataxisFramework) AddDocument(doc *Document) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}

	if _, exists := fw.documents[doc.ID]; exists {
		return fmt.Errorf("document with ID %s already exists", doc.ID)
	}

	// Set timestamps
	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	fw.documents[doc.ID] = doc
	fw.docsByType[doc.Type] = append(fw.docsByType[doc.Type], doc)

	return nil
}

// GetDocument retrieves a document by ID
func (fw *DiataxisFramework) GetDocument(id string) (*Document, error) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	doc, exists := fw.documents[id]
	if !exists {
		return nil, fmt.Errorf("document with ID %s not found", id)
	}

	return doc, nil
}

// GetDocumentsByType retrieves all documents of a specific type
func (fw *DiataxisFramework) GetDocumentsByType(docType DocType) []*Document {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	return fw.docsByType[docType]
}

// GetAllDocuments retrieves all documents
func (fw *DiataxisFramework) GetAllDocuments() []*Document {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	docs := make([]*Document, 0, len(fw.documents))
	for _, doc := range fw.documents {
		docs = append(docs, doc)
	}

	return docs
}

// ProcessAllDocuments processes all documents concurrently
func (fw *DiataxisFramework) ProcessAllDocuments(ctx context.Context) error {
	docs := fw.GetAllDocuments()
	return fw.processor.ProcessDocuments(ctx, docs)
}

// ValidateAllDocuments validates all documents concurrently
func (fw *DiataxisFramework) ValidateAllDocuments(ctx context.Context) (*ValidationReport, error) {
	docs := fw.GetAllDocuments()
	return fw.validator.ValidateDocuments(ctx, docs)
}

// GenerateDocumentation generates the complete documentation site
func (fw *DiataxisFramework) GenerateDocumentation(ctx context.Context) error {
	return fw.generator.Generate(ctx)
}

// Validate validates a single document for Diataxis compliance
// This is a convenience method that can be called without a full framework
func (doc *Document) Validate() []ValidationIssue {
	var issues []ValidationIssue

	// Create standalone validator rules
	rules := []ValidationRule{
		&RequiredFieldsRule{},
		&ContentLengthRule{},
		&CodeBlockValidityRule{},
		&MetadataRule{},
		&DiataxisStructureRule{},
		&LinkValidityRule{},
	}

	// Run all validation rules
	for _, rule := range rules {
		if ruleIssues := rule.Validate(doc); len(ruleIssues) > 0 {
			issues = append(issues, ruleIssues...)
		}
	}

	// Update validation status based on issues
	if len(issues) == 0 {
		doc.ValidationStatus = ValidationPassed
	} else {
		hasErrors := false
		for _, issue := range issues {
			if issue.Severity == "error" {
				hasErrors = true
				break
			}
		}

		if hasErrors {
			doc.ValidationStatus = ValidationFailed
		} else {
			doc.ValidationStatus = ValidationWarnings
		}
	}

	return issues
}

// NewTutorial creates a new Tutorial document with default settings.
// Tutorials are learning-oriented and take the reader through a series of steps.
func NewTutorial(id, title string) *Document {
	return &Document{
		ID:               id,
		Type:             Tutorial,
		Title:            title,
		Metadata:         make(map[string]interface{}),
		Tags:             make([]string, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ValidationStatus: ValidationPending,
		RelatedDocs:      make([]string, 0),
		Prerequisites:    make([]string, 0),
	}
}

// NewHowTo creates a new HowTo document with default settings.
// HowTo guides are goal-oriented and help readers solve specific problems.
func NewHowTo(id, title string) *Document {
	return &Document{
		ID:               id,
		Type:             HowTo,
		Title:            title,
		Metadata:         make(map[string]interface{}),
		Tags:             make([]string, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ValidationStatus: ValidationPending,
		RelatedDocs:      make([]string, 0),
		Prerequisites:    make([]string, 0),
	}
}

// NewReference creates a new Reference document with default settings.
// Reference guides are information-oriented technical descriptions.
func NewReference(id, title string) *Document {
	return &Document{
		ID:               id,
		Type:             Reference,
		Title:            title,
		Metadata:         make(map[string]interface{}),
		Tags:             make([]string, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ValidationStatus: ValidationPending,
		RelatedDocs:      make([]string, 0),
		Prerequisites:    make([]string, 0),
	}
}

// NewExplanation creates a new Explanation document with default settings.
// Explanations are understanding-oriented and deepen comprehension of topics.
func NewExplanation(id, title string) *Document {
	return &Document{
		ID:               id,
		Type:             Explanation,
		Title:            title,
		Metadata:         make(map[string]interface{}),
		Tags:             make([]string, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ValidationStatus: ValidationPending,
		RelatedDocs:      make([]string, 0),
		Prerequisites:    make([]string, 0),
	}
}

// GetStatistics returns statistics about the documentation
func (fw *DiataxisFramework) GetStatistics() *FrameworkStatistics {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	stats := &FrameworkStatistics{
		TotalDocuments:  len(fw.documents),
		DocumentsByType: make(map[DocType]int),
		ValidationStats: make(map[ValidationStatus]int),
	}

	for docType, docs := range fw.docsByType {
		stats.DocumentsByType[docType] = len(docs)
	}

	for _, doc := range fw.documents {
		stats.ValidationStats[doc.ValidationStatus]++
		stats.AverageQualityScore += doc.QualityScore
	}

	if stats.TotalDocuments > 0 {
		stats.AverageQualityScore /= float64(stats.TotalDocuments)
	}

	return stats
}

// FrameworkStatistics holds statistics about the documentation framework
type FrameworkStatistics struct {
	TotalDocuments      int                      `json:"total_documents"`
	DocumentsByType     map[DocType]int          `json:"documents_by_type"`
	ValidationStats     map[ValidationStatus]int `json:"validation_stats"`
	AverageQualityScore float64                  `json:"average_quality_score"`
}

// ValidationReport contains results from document validation
type ValidationReport struct {
	TotalDocuments int
	PassedCount    int
	FailedCount    int
	WarningsCount  int
	Issues         []ValidationIssue
	ProcessingTime time.Duration
}

// ValidationIssue represents a validation problem
type ValidationIssue struct {
	DocumentID string
	Severity   string // error, warning, info
	Message    string
	Location   string
}
