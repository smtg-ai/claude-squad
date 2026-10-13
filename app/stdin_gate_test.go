package app

import (
	"os"
	"testing"
	"time"
)

// While attached, the TUI's stdin reader must not consume input meant for the session.
func TestGatedStdinBlocksWhileAttached(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	g := &gatedStdin{File: r}

	g.mu.Lock() // attached
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := g.Read(buf)
		got <- string(buf[:n])
	}()

	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case s := <-got:
		t.Fatalf("TUI read %q while attached", s)
	case <-time.After(100 * time.Millisecond):
	}

	g.mu.Unlock() // detached
	select {
	case s := <-got:
		if s != "x" {
			t.Fatalf("got %q, want %q", s, "x")
		}
	case <-time.After(time.Second):
		t.Fatal("TUI read did not resume after detach")
	}
}

func TestAttachAndWaitReleasesGateOnError(t *testing.T) {
	err := attachAndWait(func() (chan struct{}, error) { return nil, os.ErrClosed })
	if err != os.ErrClosed {
		t.Fatalf("got %v, want %v", err, os.ErrClosed)
	}
	if !uiStdin.mu.TryLock() {
		t.Fatal("gate still held after failed attach")
	}
	uiStdin.mu.Unlock()
}
