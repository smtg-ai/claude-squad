package mcp

import (
	"fmt"
	"log"
)

// logger is the package-level logger. If nil, logging is a no-op.
var logger *log.Logger

// SetLogger sets the file logger for the MCP server package.
func SetLogger(l *log.Logger) {
	logger = l
}

// Log writes a formatted message to the MCP log file. No-op if logger is nil.
func Log(format string, args ...any) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf(format, args...))
	}
}
