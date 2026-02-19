package overlay

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToastManager(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	require.NotNil(t, tm, "NewToastManager should return a non-nil manager")
	assert.Empty(t, tm.toasts, "new manager should have no toasts")
	assert.Equal(t, &s, tm.spinner, "spinner should be stored")
}

func TestToastTypes(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	infoID := tm.Info("info message")
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, ToastInfo, tm.toasts[0].Type, "Info() should create ToastInfo")
	assert.Equal(t, "info message", tm.toasts[0].Message)
	assert.NotEmpty(t, infoID)

	successID := tm.Success("success message")
	require.Len(t, tm.toasts, 2)
	assert.Equal(t, ToastSuccess, tm.toasts[1].Type, "Success() should create ToastSuccess")
	assert.Equal(t, "success message", tm.toasts[1].Message)
	assert.NotEmpty(t, successID)

	errorID := tm.Error("error message")
	require.Len(t, tm.toasts, 3)
	assert.Equal(t, ToastError, tm.toasts[2].Type, "Error() should create ToastError")
	assert.Equal(t, "error message", tm.toasts[2].Message)
	assert.NotEmpty(t, errorID)

	loadingID := tm.Loading("loading message")
	require.Len(t, tm.toasts, 4)
	assert.Equal(t, ToastLoading, tm.toasts[3].Type, "Loading() should create ToastLoading")
	assert.Equal(t, "loading message", tm.toasts[3].Message)
	assert.NotEmpty(t, loadingID, "Loading() must return a non-empty ID")
	assert.Zero(t, tm.toasts[3].Duration, "Loading toasts should have zero duration (no auto-dismiss)")

	// Verify all IDs are unique.
	ids := map[string]bool{infoID: true, successID: true, errorID: true, loadingID: true}
	assert.Len(t, ids, 4, "all toast IDs should be unique")
}

func TestResolveNonexistentID(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	// Resolving a non-existent ID should not panic and should be a no-op.
	assert.NotPanics(t, func() {
		tm.Resolve("does-not-exist", ToastSuccess, "resolved")
	}, "Resolve with non-existent ID should not panic")
	assert.Empty(t, tm.toasts, "toasts should remain empty after resolving non-existent ID")

	// Tick on empty manager should also not panic.
	assert.NotPanics(t, func() {
		tm.Tick()
	}, "Tick on empty manager should not panic")
}

func TestToastAnimationPhases(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	_ = tm.Info("hello")
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseSlidingIn, tm.toasts[0].Phase, "new toast should start in PhaseSlidingIn")

	// Advance past SlideInDuration -> should become PhaseVisible.
	tm.toasts[0].PhaseStart = time.Now().Add(-SlideInDuration - time.Millisecond)
	tm.Tick()
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseVisible, tm.toasts[0].Phase, "should transition to PhaseVisible after slide-in")

	// Advance past InfoDismissAfter -> should become PhaseSlidingOut.
	tm.toasts[0].PhaseStart = time.Now().Add(-InfoDismissAfter - time.Millisecond)
	tm.Tick()
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseSlidingOut, tm.toasts[0].Phase, "should transition to PhaseSlidingOut after visible duration")

	// Advance past SlideOutDuration -> should be removed (PhaseDone).
	tm.toasts[0].PhaseStart = time.Now().Add(-SlideOutDuration - time.Millisecond)
	tm.Tick()
	assert.Empty(t, tm.toasts, "toast should be removed after slide-out completes")
}

func TestLoadingToastDoesNotAutoDismiss(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	id := tm.Loading("working...")
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseSlidingIn, tm.toasts[0].Phase)

	// Advance past slide-in -> PhaseVisible.
	tm.toasts[0].PhaseStart = time.Now().Add(-SlideInDuration - time.Millisecond)
	tm.Tick()
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseVisible, tm.toasts[0].Phase, "loading toast should reach PhaseVisible")

	// Even after a long time, loading toast should stay in PhaseVisible.
	tm.toasts[0].PhaseStart = time.Now().Add(-1 * time.Minute)
	tm.Tick()
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseVisible, tm.toasts[0].Phase, "loading toast should not auto-dismiss")

	// Resolve the loading toast to success.
	tm.Resolve(id, ToastSuccess, "done!")
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, ToastSuccess, tm.toasts[0].Type, "resolved toast should be ToastSuccess")
	assert.Equal(t, "done!", tm.toasts[0].Message)
	assert.Equal(t, PhaseVisible, tm.toasts[0].Phase, "resolved toast should be PhaseVisible")

	// Advance past SuccessDismissAfter -> should become PhaseSlidingOut.
	tm.toasts[0].PhaseStart = time.Now().Add(-SuccessDismissAfter - time.Millisecond)
	tm.Tick()
	require.Len(t, tm.toasts, 1)
	assert.Equal(t, PhaseSlidingOut, tm.toasts[0].Phase, "resolved toast should begin sliding out after dismiss duration")
}

func TestToastViewRendersContent(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	_ = tm.Info("hello world")
	require.Len(t, tm.toasts, 1)

	// Force the toast into PhaseVisible so it renders without animation offset.
	tm.toasts[0].Phase = PhaseVisible
	tm.toasts[0].PhaseStart = time.Now()

	view := tm.View()
	assert.NotEmpty(t, view, "View() should return non-empty string for active toast")
	assert.Contains(t, view, "hello world", "View() should contain the toast message text")
}

func TestToastViewEmpty(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	view := tm.View()
	assert.Empty(t, view, "View() should return empty string when there are no toasts")
}

func TestToastViewStacking(t *testing.T) {
	s := spinner.New()
	tm := NewToastManager(&s)

	_ = tm.Info("first message")
	_ = tm.Error("second message")
	require.Len(t, tm.toasts, 2)

	// Force both toasts into PhaseVisible.
	for _, toast := range tm.toasts {
		toast.Phase = PhaseVisible
		toast.PhaseStart = time.Now()
	}

	view := tm.View()
	assert.Contains(t, view, "first message", "View() should contain the first toast message")
	assert.Contains(t, view, "second message", "View() should contain the second toast message")
}
