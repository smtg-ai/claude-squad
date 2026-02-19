package session

import (
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// EmbeddedTerminal provides a zero-latency embedded terminal view.
//
// Architecture: creates a dedicated `tmux attach-session` PTY, reads its
// output stream directly through a VT emulator (charmbracelet/x/vt), and
// renders from the emulator's in-memory screen buffer. No subprocess calls per frame.
//
// Data flow:
//
//	PTY stdout  → readLoop → emu.Write()        (updates screen state)
//	PTY stdin   ← responseLoop ← emu.Read()     (terminal query responses)
//	User keys   → SendKey → PTY stdin           (zero latency, bypasses emulator)
//	Display     ← Render() ← renderLoop cache   (decoupled from emulator lock)
//
// Signal-driven rendering: readLoop signals dataReady after each Write(),
// renderLoop wakes immediately and snapshots the screen into the cache,
// then signals renderReady so the display tick fires without fixed sleeps.
type EmbeddedTerminal struct {
	ptmx *os.File  // dedicated attach PTY
	cmd  *exec.Cmd // tmux attach-session process
	emu  *vt.SafeEmulator

	cancel chan struct{}

	// Signal channels (buffered, cap 1) for event-driven rendering.
	// readLoop signals dataReady after emu.Write(); renderLoop waits on it.
	// renderLoop signals renderReady after cache update; display tick waits on it.
	dataReady   chan struct{}
	renderReady chan struct{}

	// Render cache — written by renderLoop, read by Render().
	// cacheMu is only held for the time it takes to swap a string and
	// flip a bool, so it never blocks the Bubble Tea event loop.
	cacheMu sync.Mutex
	cached  string
	hasNew  bool
}

// NewEmbeddedTerminal creates an embedded terminal connected to a tmux session.
// It spawns a dedicated `tmux attach-session` process with its own PTY,
// reads the output stream through a VT emulator, and renders from memory.
func NewEmbeddedTerminal(sessionName string, cols, rows int) (*EmbeddedTerminal, error) {
	emu := vt.NewSafeEmulator(cols, rows)

	// Create a dedicated tmux attach for this terminal view
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	})
	if err != nil {
		return nil, err
	}

	t := &EmbeddedTerminal{
		ptmx:        ptmx,
		cmd:         cmd,
		emu:         emu,
		cancel:      make(chan struct{}),
		dataReady:   make(chan struct{}, 1),
		renderReady: make(chan struct{}, 1),
	}

	go t.readLoop()
	go t.responseLoop()
	go t.renderLoop()
	return t, nil
}

// readLoop continuously reads PTY output and feeds it to the VT emulator.
func (t *EmbeddedTerminal) readLoop() {
	buf := make([]byte, 32768)
	for {
		select {
		case <-t.cancel:
			return
		default:
		}

		n, err := t.ptmx.Read(buf)
		if n > 0 {
			t.emu.Write(buf[:n])
			// Signal renderLoop that new data was processed.
			// Non-blocking: if a signal is already pending, skip.
			select {
			case t.dataReady <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// responseLoop reads terminal query responses from the VT emulator and pipes
// them back to the PTY. Without this, query responses block emu.Write() on
// the emulator's internal io.Pipe and deadlock the SafeEmulator mutex.
func (t *EmbeddedTerminal) responseLoop() {
	buf := make([]byte, 256)
	for {
		n, err := t.emu.Read(buf)
		if n > 0 {
			t.ptmx.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// renderLoop snapshots the emulator screen into the cache whenever new data
// arrives. It wakes on dataReady (signaled by readLoop) instead of polling,
// so the cache is updated within microseconds of new PTY data arriving.
func (t *EmbeddedTerminal) renderLoop() {
	var lastRender string
	for {
		// Wait for readLoop to signal new data, or cancel.
		select {
		case <-t.dataReady:
		case <-t.cancel:
			return
		}

		// Drain any extra pending signals so we render the latest state.
		drainChannel(t.dataReady)

		// May briefly block while readLoop holds the emulator write lock.
		// That's fine — it doesn't block the Bubble Tea event loop.
		content := t.emu.Render()

		if content != lastRender {
			t.cacheMu.Lock()
			t.cached = content
			t.hasNew = true
			t.cacheMu.Unlock()
			lastRender = content

			// Signal the display tick that new content is available.
			select {
			case t.renderReady <- struct{}{}:
			default:
			}
		}
	}
}

// drainChannel discards any pending signals on a buffered channel.
func drainChannel(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// SendKey writes raw bytes directly to the PTY.
func (t *EmbeddedTerminal) SendKey(data []byte) error {
	_, err := t.ptmx.Write(data)
	return err
}

// Render returns the latest cached screen content. This never blocks on the
// emulator lock — it only touches the lightweight cacheMu for microseconds.
// Returns ("", false) if nothing changed since the last call.
func (t *EmbeddedTerminal) Render() (string, bool) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if !t.hasNew {
		return "", false
	}
	t.hasNew = false
	return t.cached, true
}

// WaitForRender blocks until new rendered content is available in the cache,
// or until the timeout expires. Used by the Bubble Tea display tick to wake
// immediately when content changes instead of polling on a fixed interval.
func (t *EmbeddedTerminal) WaitForRender(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-t.renderReady:
	case <-t.cancel:
	case <-timer.C:
	}
}

// Resize updates the terminal dimensions.
func (t *EmbeddedTerminal) Resize(cols, rows int) {
	t.emu.Resize(cols, rows)
	if t.ptmx != nil {
		_ = pty.Setsize(t.ptmx, &pty.Winsize{
			Cols: uint16(cols),
			Rows: uint16(rows),
		})
	}
}

// Close shuts down the terminal: stops all goroutines, closes the PTY,
// and kills the tmux attach process.
func (t *EmbeddedTerminal) Close() {
	select {
	case <-t.cancel:
		return // already closed
	default:
		close(t.cancel)
	}

	// Close the emulator first — this closes the internal io.Pipe,
	// causing responseLoop to exit via io.EOF from emu.Read().
	t.emu.Close()

	if t.ptmx != nil {
		t.ptmx.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
}
