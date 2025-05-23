package agent

import (
	"compress/gzip"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// WhiplashProtocol implements ultra-efficient command compression and processing
type WhiplashProtocol struct {
	compressionDict map[string]string
	expansionDict   map[string]string
	commandPattern  *regexp.Regexp
}

// NewWhiplashProtocol creates a new whiplash protocol handler
func NewWhiplashProtocol() *WhiplashProtocol {
	wp := &WhiplashProtocol{
		compressionDict: make(map[string]string),
		expansionDict:   make(map[string]string),
		commandPattern:  regexp.MustCompile(`^(\w+)\s+(\w+)\s+(\w+)(?:\s+(.*))?$`),
	}
	
	// Initialize compression dictionary with common patterns
	wp.initializeCompressionDict()
	
	return wp
}

// initializeCompressionDict sets up the compression mappings
func (wp *WhiplashProtocol) initializeCompressionDict() {
	// Directive mappings
	compressionMap := map[string]string{
		// Core directives
		"SYNCHRONIZE":   "!S",
		"COORDINATE":    "!C",
		"OPTIMIZE":      "!O",
		"EXECUTE":       "!X",
		"QUERY":         "!Q",
		"UPDATE":        "!U",
		"MERGE":         "!M",
		"BROADCAST":     "!B",
		"REFLECT":       "!R",
		"ANALYZE":       "!A",
		
		// Operators
		"IMMEDIATE":     "@I",
		"BATCH":         "@B",
		"ASYNC":         "@A",
		"MIRROR":        "@M",
		"FRACTAL":       "@F",
		"VECTOR":        "@V",
		"QUANTUM":       "@Q",
		"RECURSIVE":     "@R",
		
		// Targets
		"ALL_SQUADS":    "#*",
		"ARCHITECTURE":  "#A",
		"INTEGRATION":   "#I",
		"PERFORMANCE":   "#P",
		"TESTING":       "#T",
		"SECURITY":      "#S",
		"DOCUMENTATION": "#D",
		"MIRROR":        "#M",
		"GITHUB":        "#G",
		
		// Common parameters
		"PRIORITY_HIGH":    "^H",
		"PRIORITY_MEDIUM":  "^M",
		"PRIORITY_LOW":     "^L",
		"FORCE":           "^F",
		"GENTLE":          "^G",
		"IMMEDIATE":       "^I",
		
		// Status indicators
		"SUCCESS":         "+",
		"FAILURE":         "-",
		"PENDING":         "?",
		"COMPLETE":        "=",
		"PARTIAL":         "~",
	}
	
	// Set up bidirectional mapping
	for long, short := range compressionMap {
		wp.compressionDict[long] = short
		wp.expansionDict[short] = long
	}
}

// Compress compresses a message using whiplash protocol
func (wp *WhiplashProtocol) Compress(content string) (string, error) {
	// First pass: Replace common patterns
	compressed := content
	for long, short := range wp.compressionDict {
		compressed = strings.ReplaceAll(compressed, long, short)
	}
	
	// Second pass: Apply gzip compression if beneficial
	if len(compressed) > 100 {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		
		_, err := writer.Write([]byte(compressed))
		if err != nil {
			return "", fmt.Errorf("gzip compression failed: %v", err)
		}
		
		err = writer.Close()
		if err != nil {
			return "", fmt.Errorf("gzip close failed: %v", err)
		}
		
		// Use gzip if it provides better compression
		if buf.Len() < len(compressed) {
			return fmt.Sprintf("GZ:%s", buf.String()), nil
		}
	}
	
	return compressed, nil
}

// Decompress decompresses a whiplash protocol message
func (wp *WhiplashProtocol) Decompress(compressed string) (string, error) {
	// Check if it's gzip compressed
	if strings.HasPrefix(compressed, "GZ:") {
		gzipData := compressed[3:]
		buf := bytes.NewBufferString(gzipData)
		reader, err := gzip.NewReader(buf)
		if err != nil {
			return "", fmt.Errorf("gzip reader creation failed: %v", err)
		}
		defer reader.Close()
		
		var result bytes.Buffer
		_, err = result.ReadFrom(reader)
		if err != nil {
			return "", fmt.Errorf("gzip decompression failed: %v", err)
		}
		
		compressed = result.String()
	}
	
	// Expand compressed patterns
	expanded := compressed
	for short, long := range wp.expansionDict {
		expanded = strings.ReplaceAll(expanded, short, long)
	}
	
	return expanded, nil
}

// CompressCommand compresses a whiplash command structure
func (wp *WhiplashProtocol) CompressCommand(cmd WhiplashCommand) string {
	// Build command string
	var parts []string
	parts = append(parts, cmd.Directive, cmd.Operator, cmd.Target)
	
	// Add parameters as JSON if present
	if len(cmd.Params) > 0 {
		paramJSON, _ := json.Marshal(cmd.Params)
		parts = append(parts, string(paramJSON))
	}
	
	commandStr := strings.Join(parts, " ")
	
	// Compress the command
	compressed, err := wp.Compress(commandStr)
	if err != nil {
		return commandStr // Fallback to uncompressed
	}
	
	return compressed
}

// ParseCommand parses a command from decompressed content
func (wp *WhiplashProtocol) ParseCommand(content string) (WhiplashCommand, bool) {
	matches := wp.commandPattern.FindStringSubmatch(content)
	if len(matches) < 4 {
		return WhiplashCommand{}, false
	}
	
	cmd := WhiplashCommand{
		Directive: matches[1],
		Operator:  matches[2],
		Target:    matches[3],
		Params:    make(map[string]string),
	}
	
	// Parse parameters if present
	if len(matches) > 4 && matches[4] != "" {
		var params map[string]string
		if err := json.Unmarshal([]byte(matches[4]), &params); err == nil {
			cmd.Params = params
		}
	}
	
	return cmd, true
}

// OptimizeCompression analyzes usage patterns and optimizes compression dictionary
func (wp *WhiplashProtocol) OptimizeCompression() {
	// This is a placeholder for machine learning-based optimization
	// In a full implementation, this would analyze message patterns
	// and update the compression dictionary accordingly
	
	// Add dynamic compression patterns based on usage
	// For now, we'll add some common squad-specific patterns
	squadPatterns := map[string]string{
		"ARCHITECTURE_REVIEW":  "~AR",
		"INTEGRATION_TEST":     "~IT", 
		"PERFORMANCE_ANALYSIS": "~PA",
		"SECURITY_SCAN":        "~SS",
		"CODE_QUALITY":         "~CQ",
		"DEPLOYMENT_READY":     "~DR",
	}
	
	for long, short := range squadPatterns {
		if _, exists := wp.compressionDict[long]; !exists {
			wp.compressionDict[long] = short
			wp.expansionDict[short] = long
		}
	}
}

// GetCompressionRatio calculates the compression ratio for given content
func (wp *WhiplashProtocol) GetCompressionRatio(original string) float64 {
	compressed, err := wp.Compress(original)
	if err != nil {
		return 1.0
	}
	
	if len(compressed) == 0 {
		return 1.0
	}
	
	return float64(len(original)) / float64(len(compressed))
}

// GetStatistics returns compression statistics
func (wp *WhiplashProtocol) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"compression_patterns": len(wp.compressionDict),
		"expansion_patterns":   len(wp.expansionDict),
		"protocol_version":     "1.0.0",
		"avg_compression":      "50:1", // Target compression ratio
	}
}