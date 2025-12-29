package overlay

import (
	"testing"
	"time"
)

func TestNewToastOverlay(t *testing.T) {
	to := NewToastOverlay()
	if to == nil {
		t.Fatal("expected non-nil ToastOverlay")
	}
	if to.IsVisible() {
		t.Error("expected no toast to be visible initially")
	}
}

func TestToastOverlay_ShowSuccess(t *testing.T) {
	to := NewToastOverlay()
	to.ShowSuccess("Operation completed")

	if !to.IsVisible() {
		t.Error("expected toast to be visible after ShowSuccess")
	}

	toast := to.GetCurrentToast()
	if toast == nil {
		t.Fatal("expected toast to exist")
	}
	if toast.Message != "Operation completed" {
		t.Errorf("expected message 'Operation completed', got '%s'", toast.Message)
	}
	if toast.Type != ToastTypeSuccess {
		t.Errorf("expected type ToastTypeSuccess, got %v", toast.Type)
	}
}

func TestToastOverlay_ShowError(t *testing.T) {
	to := NewToastOverlay()
	to.ShowError("Something went wrong")

	if !to.IsVisible() {
		t.Error("expected toast to be visible after ShowError")
	}

	toast := to.GetCurrentToast()
	if toast == nil {
		t.Fatal("expected toast to exist")
	}
	if toast.Type != ToastTypeError {
		t.Errorf("expected type ToastTypeError, got %v", toast.Type)
	}
}

func TestToastOverlay_ShowInfo(t *testing.T) {
	to := NewToastOverlay()
	to.ShowInfo("Information message")

	if !to.IsVisible() {
		t.Error("expected toast to be visible after ShowInfo")
	}

	toast := to.GetCurrentToast()
	if toast == nil {
		t.Fatal("expected toast to exist")
	}
	if toast.Type != ToastTypeInfo {
		t.Errorf("expected type ToastTypeInfo, got %v", toast.Type)
	}
}

func TestToastOverlay_Clear(t *testing.T) {
	to := NewToastOverlay()
	to.ShowSuccess("Test message")

	if !to.IsVisible() {
		t.Error("expected toast to be visible")
	}

	to.Clear()

	if to.IsVisible() {
		t.Error("expected toast to be hidden after Clear")
	}
}

func TestToastOverlay_Expiration(t *testing.T) {
	to := NewToastOverlay()
	// Use a very short duration for testing
	to.Show("Test", ToastTypeInfo, 10*time.Millisecond)

	if !to.IsVisible() {
		t.Error("expected toast to be visible immediately")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	if to.IsVisible() {
		t.Error("expected toast to be hidden after expiration")
	}
}

func TestToastOverlay_Render(t *testing.T) {
	to := NewToastOverlay()

	// Test render with no toast
	result := to.Render()
	if result != "" {
		t.Errorf("expected empty string when no toast, got '%s'", result)
	}

	// Test render with toast
	to.ShowSuccess("Test message")
	result = to.Render()
	if result == "" {
		t.Error("expected non-empty string when toast is visible")
	}
}

func TestToastOverlay_SetWidth(t *testing.T) {
	to := NewToastOverlay()
	to.SetWidth(100)
	// Just ensure it doesn't panic
	to.ShowSuccess("Test")
	_ = to.Render()
}
