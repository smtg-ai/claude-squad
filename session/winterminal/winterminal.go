//go:build windows

package winterminal

import (
	"bytes"
	"claude-squad/cmd"
	"claude-squad/log"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/UserExistsError/conpty"
	"golang.org/x/term"
)

const (
	programClaude = "claude"
	programAider  = "aider"
	programGemini = "gemini"

	defaultCols = 80
	defaultRows = 24
)

// WindowsTerminalSession manages an interactive terminal session on Windows
// using the ConPTY (Pseudo Console) API via the conpty library.
type WindowsTerminalSession struct {
	sessionName   string
	sanitizedName string
	program       string

	// ConPTY handle
	cpty *conpty.ConPty

	// Screen buffer: accumulated output lines
	lines   []string
	partial string
	bufMu   sync.RWMutex

	// Content monitoring for HasUpdated
	prevHash []byte

	// Terminal dimensions
	width  int
	height int

	// Session state
	isRunning bool
	mu        sync.RWMutex

	// Output reader goroutine lifecycle
	outputDone chan struct{}

	// Attach state
	attached atomic.Bool
	attachCh chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	wg       *sync.WaitGroup

	cmdExec cmd.Executor
}

// sanitizeName cleans up the session name for use as an identifier.
func sanitizeName(name string) string {
	str := strings.ReplaceAll(name, " ", "_")
	str = strings.ReplaceAll(str, "-", "_")
	str = strings.ReplaceAll(str, ".", "_")
	return "claudesquad_" + str
}

// NewWindowsTerminalSession creates a new Windows Terminal session.
func NewWindowsTerminalSession(sessionName, program string) *WindowsTerminalSession {
	return &WindowsTerminalSession{
		sessionName:   sessionName,
		sanitizedName: sanitizeName(sessionName),
		program:       program,
		cmdExec:       cmd.MakeExecutor(),
	}
}

// NewWindowsTerminalSessionWithDeps creates a session with a custom executor (for testing).
func NewWindowsTerminalSessionWithDeps(sessionName, program string, cmdExec cmd.Executor) *WindowsTerminalSession {
	return &WindowsTerminalSession{
		sessionName:   sessionName,
		sanitizedName: sanitizeName(sessionName),
		program:       program,
		cmdExec:       cmdExec,
	}
}

// Start creates a ConPTY, launches the program inside it, and begins
// capturing output into the screen buffer.
func (w *WindowsTerminalSession) Start(workDir string) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("session already running: %s", w.sanitizedName)
	}

	cols, rows := w.width, w.height
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}

	commandLine := "cmd.exe /c " + w.program
	opts := []conpty.ConPtyOption{
		conpty.ConPtyDimensions(cols, rows),
	}
	if workDir != "" {
		opts = append(opts, conpty.ConPtyWorkDir(workDir))
	}

	cpty, err := conpty.Start(commandLine, opts...)
	if err != nil {
		w.mu.Unlock()
		return fmt.Errorf("start conpty: %w", err)
	}

	w.cpty = cpty
	w.isRunning = true
	w.outputDone = make(chan struct{})
	w.mu.Unlock()

	go w.readOutput()
	w.handleTrustScreen()
	return nil
}

// readOutput continuously reads from the ConPTY and appends to the screen buffer.
// When attached, it also tees output to os.Stdout.
func (w *WindowsTerminalSession) readOutput() {
	defer close(w.outputDone)
	buf := make([]byte, 4096)
	for {
		n, err := w.cpty.Read(buf)
		if n > 0 {
			data := buf[:n]
			w.appendToBuffer(data)
			if w.attached.Load() {
				_, _ = os.Stdout.Write(data)
			}
		}
		if err != nil {
			return
		}
	}
}

// appendToBuffer parses raw terminal output into lines.
func (w *WindowsTerminalSession) appendToBuffer(data []byte) {
	w.bufMu.Lock()
	defer w.bufMu.Unlock()

	w.partial += string(data)
	for {
		idx := strings.IndexByte(w.partial, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSuffix(w.partial[:idx], "\r")
		w.lines = append(w.lines, line)
		w.partial = w.partial[idx+1:]
	}
}

// handleTrustScreen waits for the "trust files" prompt that claude/aider/gemini
// show on first launch, and automatically confirms it.
func (w *WindowsTerminalSession) handleTrustScreen() {
	var searchString string
	var tapFunc func() error
	var maxWait time.Duration

	if strings.HasSuffix(w.program, programClaude) {
		searchString = "Do you trust the files in this folder?"
		tapFunc = w.TapEnter
		maxWait = 30 * time.Second
	} else if strings.HasSuffix(w.program, programAider) || strings.HasSuffix(w.program, programGemini) {
		searchString = "Open documentation url for more info"
		tapFunc = func() error { return w.SendKeys("D\r") }
		maxWait = 45 * time.Second
	} else {
		return
	}

	start := time.Now()
	sleep := 100 * time.Millisecond
	for time.Since(start) < maxWait {
		time.Sleep(sleep)
		content, err := w.CapturePaneContent()
		if err == nil && strings.Contains(content, searchString) {
			if err := tapFunc(); err != nil {
				log.ErrorLog.Printf("could not tap enter on trust screen: %v", err)
			}
			return
		}
		sleep = time.Duration(float64(sleep) * 1.2)
		if sleep > time.Second {
			sleep = time.Second
		}
	}
}

// Restore checks if the process is still alive and updates the running state.
func (w *WindowsTerminalSession) Restore() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.isRunning = w.doesSessionExistLocked()
	return nil
}

func (w *WindowsTerminalSession) doesSessionExistLocked() bool {
	if w.cpty == nil || w.outputDone == nil {
		return false
	}
	select {
	case <-w.outputDone:
		return false // output reader finished → process exited
	default:
		return true
	}
}

// DoesSessionExist checks if the ConPTY process is still alive.
func (w *WindowsTerminalSession) DoesSessionExist() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.doesSessionExistLocked()
}

// SendKeys writes raw keystrokes to the ConPTY.
func (w *WindowsTerminalSession) SendKeys(keys string) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.isRunning {
		return fmt.Errorf("session not running")
	}
	_, err := w.cpty.Write([]byte(keys))
	return err
}

// TapEnter sends a carriage return to the session.
func (w *WindowsTerminalSession) TapEnter() error {
	return w.SendKeys("\r")
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// HasUpdated checks if output has changed and whether a prompt is visible.
func (w *WindowsTerminalSession) HasUpdated() (updated bool, hasPrompt bool) {
	content, err := w.CapturePaneContent()
	if err != nil {
		return false, false
	}

	plain := stripANSI(content)
	if strings.HasSuffix(w.program, programClaude) {
		hasPrompt = strings.Contains(plain, "No, and tell Claude what to do differently")
	} else if strings.HasPrefix(w.program, programAider) {
		hasPrompt = strings.Contains(plain, "(Y)es/(N)o/(D)on't ask again")
	} else if strings.HasPrefix(w.program, programGemini) {
		hasPrompt = strings.Contains(plain, "Yes, allow once")
	}

	h := sha256.Sum256([]byte(content))
	hash := h[:]
	if !bytes.Equal(hash, w.prevHash) {
		w.prevHash = hash
		return true, hasPrompt
	}
	return false, hasPrompt
}

// CapturePaneContent returns the last screenful of output (height lines).
func (w *WindowsTerminalSession) CapturePaneContent() (string, error) {
	w.bufMu.RLock()
	defer w.bufMu.RUnlock()

	h := w.height
	if h <= 0 {
		h = defaultRows
	}

	start := len(w.lines) - h
	if start < 0 {
		start = 0
	}

	var sb strings.Builder
	for i, line := range w.lines[start:] {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
	}
	if w.partial != "" {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(w.partial)
	}
	return sb.String(), nil
}

// CapturePaneContentWithOptions returns all accumulated output (full history).
func (w *WindowsTerminalSession) CapturePaneContentWithOptions(start, end string) (string, error) {
	w.bufMu.RLock()
	defer w.bufMu.RUnlock()

	var sb strings.Builder
	for i, line := range w.lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
	}
	if w.partial != "" {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(w.partial)
	}
	return sb.String(), nil
}

// SetDetachedSize updates the ConPTY dimensions while detached.
func (w *WindowsTerminalSession) SetDetachedSize(width, height int) error {
	w.mu.Lock()
	w.width = width
	w.height = height
	w.mu.Unlock()

	if w.cpty != nil {
		return w.cpty.Resize(width, height)
	}
	return nil
}

// Attach connects stdin/stdout to the ConPTY for interactive use.
// Returns a channel that is closed when the user detaches (Ctrl+Q).
func (w *WindowsTerminalSession) Attach() (chan struct{}, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.isRunning {
		return nil, fmt.Errorf("session not running")
	}

	w.attachCh = make(chan struct{})
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.wg = &sync.WaitGroup{}

	// Show current content on attach so the user sees the current state.
	content, _ := w.CapturePaneContent()
	if content != "" {
		fmt.Fprint(os.Stdout, content)
	}
	w.attached.Store(true)

	// Stdin reader: forward keystrokes to ConPTY, detect Ctrl+Q to detach.
	go func() {
		timeout := time.After(50 * time.Millisecond)
		buf := make([]byte, 256)
		for {
			nr, err := os.Stdin.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				continue
			}

			// Discard initial terminal control sequences.
			select {
			case <-timeout:
			default:
				log.InfoLog.Printf("nuked initial stdin: %q", buf[:nr])
				continue
			}

			// Ctrl+Q (ASCII 17) → detach
			if nr == 1 && buf[0] == 17 {
				w.detach()
				return
			}

			_, _ = w.cpty.Write(buf[:nr])
		}
	}()

	// Window-size monitor (polling, since Windows has no SIGWINCH).
	w.monitorWindowSize()

	return w.attachCh, nil
}

// monitorWindowSize polls for terminal size changes and resizes the ConPTY.
func (w *WindowsTerminalSession) monitorWindowSize() {
	cols, rows, err := term.GetSize(int(os.Stdin.Fd()))
	if err == nil {
		_ = w.cpty.Resize(cols, rows)
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		lastCols, lastRows := cols, rows
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				c, r, err := term.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					continue
				}
				if c != lastCols || r != lastRows {
					lastCols, lastRows = c, r
					_ = w.cpty.Resize(c, r)
				}
			}
		}
	}()
}

// detach disconnects stdin/stdout without stopping the background process.
func (w *WindowsTerminalSession) detach() {
	w.attached.Store(false)
	if w.cancel != nil {
		w.cancel()
	}
	if w.wg != nil {
		w.wg.Wait()
	}
	if w.attachCh != nil {
		close(w.attachCh)
		w.attachCh = nil
	}
	w.cancel = nil
	w.ctx = nil
	w.wg = nil
}

// DetachSafely disconnects from the session without stopping the process.
func (w *WindowsTerminalSession) DetachSafely() error {
	if w.attachCh == nil {
		return nil
	}
	w.detach()
	return nil
}

// Close terminates the ConPTY process and cleans up resources.
func (w *WindowsTerminalSession) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isRunning {
		return nil
	}

	if w.cpty != nil {
		if err := w.cpty.Close(); err != nil {
			w.isRunning = false
			return err
		}
	}

	// Wait for output reader to finish.
	if w.outputDone != nil {
		<-w.outputDone
	}

	w.isRunning = false
	return nil
}

// CleanupSessions is a no-op on Windows. ConPTY sessions are tied to their
// process lifetime and do not need global enumeration like tmux.
func CleanupSessions(cmdExec cmd.Executor) error {
	return nil
}
