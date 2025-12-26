package docs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConcurrentProcessor handles concurrent document processing using worker pools
type ConcurrentProcessor struct {
	maxWorkers       int
	pipeline         *ProcessingPipeline
	progressCallback ProgressCallback
}

// ProgressCallback is called to report processing progress
type ProgressCallback func(docID string, completed bool, err error)

// NewConcurrentProcessor creates a new concurrent processor
func NewConcurrentProcessor(maxWorkers int) *ConcurrentProcessor {
	if maxWorkers <= 0 {
		maxWorkers = MaxConcurrentWorkers
	}
	if maxWorkers > 1000 {
		maxWorkers = 1000 // Cap at reasonable limit
	}

	return &ConcurrentProcessor{
		maxWorkers: maxWorkers,
		pipeline:   NewProcessingPipeline(),
	}
}

// SetProgressCallback sets the progress callback function
func (cp *ConcurrentProcessor) SetProgressCallback(callback ProgressCallback) {
	cp.progressCallback = callback
}

// ProcessDocuments processes multiple documents concurrently
func (cp *ConcurrentProcessor) ProcessDocuments(ctx context.Context, docs []*Document) error {
	// Create worker pool with bounded concurrency
	semaphore := make(chan struct{}, cp.maxWorkers)
	errChan := make(chan error, len(docs))
	var wg sync.WaitGroup

	// Process each document concurrently
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
				if cp.progressCallback != nil {
					cp.progressCallback(d.ID, false, ctx.Err())
				}
				return
			}

			// Process the document
			err := cp.processDocument(ctx, d)
			if err != nil {
				errChan <- fmt.Errorf("failed to process document %s: %w", d.ID, err)
			}

			// Report progress via callback
			if cp.progressCallback != nil {
				cp.progressCallback(d.ID, err == nil, err)
			}
		}(doc)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Collect all errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// Return aggregated error with all error details
		return &ProcessingErrors{
			Count:  len(errs),
			Errors: errs,
		}
	}

	return nil
}

// ProcessingErrors aggregates multiple processing errors
type ProcessingErrors struct {
	Count  int
	Errors []error
}

func (pe *ProcessingErrors) Error() string {
	if pe.Count == 1 {
		return fmt.Sprintf("processing failed: %v", pe.Errors[0])
	}
	return fmt.Sprintf("processing failed with %d errors: first error: %v", pe.Count, pe.Errors[0])
}

// processDocument processes a single document through the pipeline
func (cp *ConcurrentProcessor) processDocument(ctx context.Context, doc *Document) error {
	return cp.pipeline.Process(ctx, doc)
}

// ProcessingPipeline defines the stages of document processing
type ProcessingPipeline struct {
	stages []ProcessingStage
}

// ProcessingStage is a single stage in the processing pipeline
type ProcessingStage interface {
	Name() string
	Process(ctx context.Context, doc *Document) error
}

// NewProcessingPipeline creates a new processing pipeline
func NewProcessingPipeline() *ProcessingPipeline {
	return &ProcessingPipeline{
		stages: []ProcessingStage{
			&MarkdownParsingStage{},
			&SyntaxHighlightingStage{},
			&CodeExtractionStage{},
			&CrossReferenceStage{},
			&MetricsCalculationStage{},
			&HTMLGenerationStage{},
		},
	}
}

// Process runs a document through all pipeline stages
func (pp *ProcessingPipeline) Process(ctx context.Context, doc *Document) error {
	// Validate document type before processing
	if err := validateDocumentType(doc); err != nil {
		return fmt.Errorf("document validation failed: %w", err)
	}

	// Process each stage with timeout protection
	for _, stage := range pp.stages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Create stage-specific context with timeout
			stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := stage.Process(stageCtx, doc)
			cancel()

			if err != nil {
				return fmt.Errorf("stage %s failed: %w", stage.Name(), err)
			}
		}
	}
	return nil
}

// validateDocumentType ensures the document has a valid type
func validateDocumentType(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("document is nil")
	}
	if doc.ID == "" {
		return fmt.Errorf("document ID is empty")
	}

	validTypes := map[DocType]bool{
		Tutorial:    true,
		HowTo:       true,
		Reference:   true,
		Explanation: true,
	}

	if !validTypes[doc.Type] {
		return fmt.Errorf("invalid document type: %s (must be tutorial, howto, reference, or explanation)", doc.Type)
	}

	if doc.Content == "" {
		return fmt.Errorf("document content is empty")
	}

	return nil
}

// MarkdownParsingStage parses markdown content
type MarkdownParsingStage struct{}

func (s *MarkdownParsingStage) Name() string { return "MarkdownParsing" }

func (s *MarkdownParsingStage) Process(ctx context.Context, doc *Document) error {
	// Parse markdown with goldmark (implemented in markdown.go)
	parser := NewMarkdownParser()
	parsed, err := parser.Parse(doc.Content)
	if err != nil {
		return err
	}

	doc.ProcessedContent = parsed
	return nil
}

// SyntaxHighlightingStage adds syntax highlighting to code blocks
type SyntaxHighlightingStage struct{}

func (s *SyntaxHighlightingStage) Name() string { return "SyntaxHighlighting" }

func (s *SyntaxHighlightingStage) Process(ctx context.Context, doc *Document) error {
	// Apply syntax highlighting with chroma (implemented in syntax.go)
	highlighter := NewSyntaxHighlighter()
	highlighted, err := highlighter.Highlight(doc.ProcessedContent)
	if err != nil {
		return err
	}

	doc.ProcessedContent = highlighted
	return nil
}

// CodeExtractionStage extracts code examples from content
type CodeExtractionStage struct{}

func (s *CodeExtractionStage) Name() string { return "CodeExtraction" }

func (s *CodeExtractionStage) Process(ctx context.Context, doc *Document) error {
	// Extract code blocks and examples
	extractor := NewCodeExtractor()
	examples := extractor.Extract(doc.Content)

	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}
	doc.Metadata["code_examples"] = examples

	return nil
}

// CrossReferenceStage resolves cross-references between documents
type CrossReferenceStage struct{}

func (s *CrossReferenceStage) Name() string { return "CrossReference" }

func (s *CrossReferenceStage) Process(ctx context.Context, doc *Document) error {
	// Extract and validate cross-references
	refExtractor := NewReferenceExtractor()
	refs := refExtractor.Extract(doc.Content)

	doc.RelatedDocs = refs
	return nil
}

// MetricsCalculationStage calculates quality metrics
type MetricsCalculationStage struct{}

func (s *MetricsCalculationStage) Name() string { return "MetricsCalculation" }

func (s *MetricsCalculationStage) Process(ctx context.Context, doc *Document) error {
	// Calculate quality score based on various factors
	calculator := NewQualityCalculator()
	score := calculator.Calculate(doc)

	doc.QualityScore = score
	return nil
}

// HTMLGenerationStage generates final HTML output
type HTMLGenerationStage struct{}

func (s *HTMLGenerationStage) Name() string { return "HTMLGeneration" }

func (s *HTMLGenerationStage) Process(ctx context.Context, doc *Document) error {
	// Generate final HTML with templates
	generator := NewHTMLGenerator()
	html, err := generator.Generate(doc)
	if err != nil {
		return err
	}

	doc.ProcessedContent = html
	return nil
}

// BatchProcessor handles batch processing with progress tracking
type BatchProcessor struct {
	processor        *ConcurrentProcessor
	progress         *ProgressTracker
	externalCallback ProgressCallback
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(maxWorkers int) *BatchProcessor {
	return &BatchProcessor{
		processor: NewConcurrentProcessor(maxWorkers),
		progress:  NewProgressTracker(),
	}
}

// SetProgressCallback sets an external progress callback
func (bp *BatchProcessor) SetProgressCallback(callback ProgressCallback) {
	bp.externalCallback = callback
}

// GetProgressTracker returns the progress tracker for real-time monitoring
func (bp *BatchProcessor) GetProgressTracker() *ProgressTracker {
	return bp.progress
}

// ProcessBatch processes a batch of documents with progress tracking
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, docs []*Document) (*ProcessingResult, error) {
	startTime := time.Now()
	bp.progress.Start(len(docs))

	// Set up progress callback to update tracker and call external callback
	bp.processor.SetProgressCallback(func(docID string, completed bool, err error) {
		// Update internal progress tracker
		if completed {
			bp.progress.IncrementCompleted()
		} else {
			bp.progress.IncrementFailed()
		}

		// Call external callback if set
		if bp.externalCallback != nil {
			bp.externalCallback(docID, completed, err)
		}
	})

	// Process documents concurrently
	err := bp.processor.ProcessDocuments(ctx, docs)

	duration := time.Since(startTime)
	bp.progress.Complete()

	// Extract individual errors if processing failed
	var allErrors []error
	if err != nil {
		if procErr, ok := err.(*ProcessingErrors); ok {
			allErrors = procErr.Errors
		} else {
			allErrors = []error{err}
		}
	}

	result := &ProcessingResult{
		TotalDocuments:  len(docs),
		ProcessedCount:  bp.progress.Completed(),
		FailedCount:     bp.progress.Failed(),
		ProcessingTime:  duration,
		DocumentsPerSec: float64(len(docs)) / duration.Seconds(),
		Errors:          allErrors,
	}

	return result, err
}

// ProcessingResult contains results from batch processing
type ProcessingResult struct {
	TotalDocuments  int
	ProcessedCount  int
	FailedCount     int
	ProcessingTime  time.Duration
	DocumentsPerSec float64
	Errors          []error // All errors encountered during processing
}

// ProgressTracker tracks processing progress
type ProgressTracker struct {
	total     int
	completed int
	failed    int
	mu        sync.Mutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{}
}

// Start initializes the progress tracker
func (pt *ProgressTracker) Start(total int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.total = total
	pt.completed = 0
	pt.failed = 0
}

// IncrementCompleted increments the completed count
func (pt *ProgressTracker) IncrementCompleted() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.completed++
}

// IncrementFailed increments the failed count
func (pt *ProgressTracker) IncrementFailed() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.failed++
}

// Complete marks processing as complete
func (pt *ProgressTracker) Complete() {
	// Placeholder for completion logic
}

// Completed returns the number of completed documents
func (pt *ProgressTracker) Completed() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.completed
}

// Failed returns the number of failed documents
func (pt *ProgressTracker) Failed() int {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.failed
}

// Progress returns the current progress as a percentage
func (pt *ProgressTracker) Progress() float64 {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if pt.total == 0 {
		return 0
	}
	return float64(pt.completed+pt.failed) / float64(pt.total) * 100
}
