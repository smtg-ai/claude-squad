package overlay

import (
	"testing"

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
}
