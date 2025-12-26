package log

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	WarningLog *log.Logger
	InfoLog    *log.Logger
	ErrorLog   *log.Logger
	DebugLog   *log.Logger
)

var debugEnabled = os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1"

var logFileName = filepath.Join(os.TempDir(), "claudesquad.log")

var globalLogFile *os.File

// Initialize should be called once at the beginning of the program to set up logging.
// defer Close() after calling this function. It sets the go log output to the file in
// the os temp directory.

func Initialize(daemon bool) {
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stderr
		fmtS := "%s"
		if daemon {
			fmtS = "[DAEMON] %s"
		}
		InfoLog = log.New(os.Stderr, fmt.Sprintf(fmtS, "INFO:"), log.Ldate|log.Ltime|log.Lshortfile)
		WarningLog = log.New(os.Stderr, fmt.Sprintf(fmtS, "WARNING:"), log.Ldate|log.Ltime|log.Lshortfile)
		ErrorLog = log.New(os.Stderr, fmt.Sprintf(fmtS, "ERROR:"), log.Ldate|log.Ltime|log.Lshortfile)
		if debugEnabled {
			DebugLog = log.New(os.Stderr, fmt.Sprintf(fmtS, "DEBUG:"), log.Ldate|log.Ltime|log.Lshortfile)
		} else {
			DebugLog = log.New(io.Discard, "", 0)
		}
		fmt.Fprintf(os.Stderr, "Warning: using stderr for logging: %v\n", err)
		return
	}

	// Set log format to include timestamp and file/line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	fmtS := "%s"
	if daemon {
		fmtS = "[DAEMON] %s"
	}
	InfoLog = log.New(f, fmt.Sprintf(fmtS, "INFO:"), log.Ldate|log.Ltime|log.Lshortfile)
	WarningLog = log.New(f, fmt.Sprintf(fmtS, "WARNING:"), log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLog = log.New(f, fmt.Sprintf(fmtS, "ERROR:"), log.Ldate|log.Ltime|log.Lshortfile)
	if debugEnabled {
		DebugLog = log.New(f, fmt.Sprintf(fmtS, "DEBUG:"), log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		DebugLog = log.New(io.Discard, "", 0)
	}

	globalLogFile = f
}

func Close() {
	_ = globalLogFile.Close()
	// TODO: maybe only print if verbose flag is set?
	fmt.Println("wrote logs to " + logFileName)
}

// Every is used to log at most once every timeout duration.
type Every struct {
	timeout time.Duration
	timer   *time.Timer
}

func NewEvery(timeout time.Duration) *Every {
	return &Every{timeout: timeout}
}

// ShouldLog returns true if the timeout has passed since the last log.
func (e *Every) ShouldLog() bool {
	if e.timer == nil {
		e.timer = time.NewTimer(e.timeout)
		e.timer.Reset(e.timeout)
		return true
	}

	select {
	case <-e.timer.C:
		e.timer.Reset(e.timeout)
		return true
	default:
		return false
	}
}

// IsDebugEnabled returns true if debug logging is enabled.
func IsDebugEnabled() bool {
	return debugEnabled
}

// SanitizeURL removes credentials from a URL string for safe logging.
// It replaces username/password with "***" to prevent leaking sensitive data in logs.
func SanitizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, redact the entire string to be safe
		return "[INVALID_URL]"
	}

	// If there are credentials, redact them
	if u.User != nil {
		// Get the original password if it exists
		_, hasPassword := u.User.Password()
		if hasPassword {
			u.User = url.UserPassword("***", "***")
		} else {
			u.User = url.User("***")
		}
	}

	return u.String()
}

// SanitizeURLs sanitizes multiple URLs in a string by replacing credentials.
// This is useful for sanitizing log messages that may contain multiple URLs.
func SanitizeURLs(message string) string {
	// Simple heuristic: look for common URL patterns and sanitize them
	// This handles cases like "connecting to http://user:pass@host:port/path"
	words := strings.Fields(message)
	for i, word := range words {
		// Check if this looks like a URL
		if strings.Contains(word, "://") {
			words[i] = SanitizeURL(word)
		}
	}
	return strings.Join(words, " ")
}
