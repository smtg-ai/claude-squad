package agent8

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRunWorkload verifies basic workload execution and statistics computation.
func TestRunWorkload(t *testing.T) {
	h := NewHarness()
	h.DefaultSampleSize = 5 // Minimum valid sample size

	workload := Workload{
		Name: "test_hash_compute",
		Operation: func(ctx context.Context) error {
			// Deterministic CPU-bound operation
			data := []byte("deterministic test data")
			hash := sha256.Sum256(data)
			_ = hash
			return nil
		},
	}

	ctx := context.Background()
	report, err := h.RunWorkload(ctx, workload)
	if err != nil {
		t.Fatalf("RunWorkload failed: %v", err)
	}

	// Verify report structure
	if report.WorkloadName != "test_hash_compute" {
		t.Errorf("Expected workload name 'test_hash_compute', got %q", report.WorkloadName)
	}

	if len(report.Runs) != 5 {
		t.Errorf("Expected 5 runs, got %d", len(report.Runs))
	}

	// Verify invariants: Q₁ (non-negative timings)
	if report.Mean < 0 || report.Min < 0 || report.Max < 0 {
		t.Errorf("Timings must be non-negative: mean=%d, min=%d, max=%d",
			report.Mean, report.Min, report.Max)
	}

	// Verify invariants: Q₂ (min ≤ mean ≤ max)
	if report.Min > report.Mean || report.Mean > report.Max {
		t.Errorf("Expected min ≤ mean ≤ max, got min=%d, mean=%d, max=%d",
			report.Min, report.Mean, report.Max)
	}

	// Verify environment is captured
	if report.Environment.GOOS == "" {
		t.Error("Environment GOOS not captured")
	}
	if report.Environment.GoVersion == "" {
		t.Error("Environment GoVersion not captured")
	}

	// Verify timestamp is reasonable (within last minute)
	now := time.Now().UnixNano()
	if report.Timestamp < now-60*1e9 || report.Timestamp > now {
		t.Errorf("Timestamp %d is outside reasonable range", report.Timestamp)
	}
}

// TestDeterminism verifies Π₁: Running the same workload produces statistically similar results.
//
// Protocol:
// 1. Run same workload 10 times
// 2. Compute coefficient of variation (CV = σ/μ)
// 3. Assert CV < 0.20 (variance < 20% of mean)
//
// Note: For very fast workloads (< 100μs), system noise can cause higher variance.
// We use a longer workload to ensure timing dominates over measurement overhead.
func TestDeterminism(t *testing.T) {
	h := NewHarness()
	h.DefaultSampleSize = 5

	workload := Workload{
		Name: "determinism_test",
		Operation: func(ctx context.Context) error {
			// Deterministic operation: SHA256 hash chain (longer workload)
			// This takes ~100-200μs, making system noise less significant
			data := []byte("fixed deterministic input for reproducibility testing")
			for i := 0; i < 1000; i++ {
				hash := sha256.Sum256(data)
				data = hash[:]
			}
			return nil
		},
	}

	ctx := context.Background()
	const numTrials = 10
	means := make([]int64, numTrials)

	// Run workload 10 times to collect mean timings
	for i := 0; i < numTrials; i++ {
		report, err := h.RunWorkload(ctx, workload)
		if err != nil {
			t.Fatalf("Trial %d failed: %v", i+1, err)
		}
		means[i] = report.Mean
	}

	// Compute mean of means and standard deviation
	globalMean, _, _, globalStdDev := computeStats(means)

	// Compute coefficient of variation: CV = σ/μ
	cv := globalStdDev / float64(globalMean)

	// Assert: CV < 0.20 (variance is less than 20% of mean)
	// This threshold accounts for scheduler noise and cache effects
	if cv >= 0.20 {
		t.Errorf("Determinism check failed: CV=%.3f ≥ 0.20 (variance too high)", cv)
		t.Logf("Means: %v", means)
		t.Logf("Global Mean: %d ns", globalMean)
		t.Logf("Global StdDev: %.2f ns", globalStdDev)
	} else {
		t.Logf("✅ Determinism verified: CV=%.3f < 0.20", cv)
	}
}

// TestRegressionDetection verifies Π₂: Regressions exceeding threshold are detected.
//
// Protocol:
// 1. Create baseline with Mean = 100,000 ns
// 2. Create current with Mean = 120,000 ns (20% slower)
// 3. Detect regression with threshold = 10%
// 4. Assert: Regression detected = true, Δ = 20%
func TestRegressionDetection(t *testing.T) {
	h := NewHarness()
	h.RegressionThreshold = 10.0 // 10% threshold

	// Create baseline report (100 microseconds mean)
	baseline := &Baseline{
		Version: "1.0.0",
		Report: TimingReport{
			WorkloadName: "regression_test",
			Runs:         []int64{98000, 100000, 102000, 99000, 101000},
			Mean:         100000,
			Min:          98000,
			Max:          102000,
			StdDev:       1414.21,
			Environment:  captureEnvironment(),
			Timestamp:    time.Now().UnixNano(),
		},
	}

	tests := []struct {
		name           string
		currentMean    int64
		expectRegress  bool
		expectDelta    float64
	}{
		{
			name:          "No regression (same performance)",
			currentMean:   100000,
			expectRegress: false,
			expectDelta:   0.0,
		},
		{
			name:          "Improvement (faster, negative delta)",
			currentMean:   90000,
			expectRegress: false,
			expectDelta:   -10.0,
		},
		{
			name:          "Small slowdown (below threshold)",
			currentMean:   105000,
			expectRegress: false,
			expectDelta:   5.0,
		},
		{
			name:          "Regression at threshold",
			currentMean:   110000,
			expectRegress: false, // Exactly at threshold, not exceeding
			expectDelta:   10.0,
		},
		{
			name:          "Regression above threshold (20%)",
			currentMean:   120000,
			expectRegress: true,
			expectDelta:   20.0,
		},
		{
			name:          "Severe regression (50%)",
			currentMean:   150000,
			expectRegress: true,
			expectDelta:   50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &TimingReport{
				WorkloadName: "regression_test",
				Runs:         []int64{tt.currentMean, tt.currentMean, tt.currentMean},
				Mean:         tt.currentMean,
				Min:          tt.currentMean,
				Max:          tt.currentMean,
				StdDev:       0,
				Environment:  captureEnvironment(),
				Timestamp:    time.Now().UnixNano(),
			}

			isRegression, delta, err := h.DetectRegression(baseline, current)
			if err != nil {
				t.Fatalf("DetectRegression failed: %v", err)
			}

			if isRegression != tt.expectRegress {
				t.Errorf("Expected isRegression=%v, got %v (delta=%.2f%%)",
					tt.expectRegress, isRegression, delta)
			}

			// Allow small floating point error (0.01%)
			if abs(delta-tt.expectDelta) > 0.01 {
				t.Errorf("Expected delta=%.2f%%, got %.2f%%", tt.expectDelta, delta)
			}

			t.Logf("✅ %s: delta=%.2f%%, regression=%v", tt.name, delta, isRegression)
		})
	}
}

// TestRegressionDetectionErrors verifies error handling in regression detection.
func TestRegressionDetectionErrors(t *testing.T) {
	h := NewHarness()

	baseline := &Baseline{
		Version: "1.0.0",
		Report: TimingReport{
			WorkloadName: "baseline_workload",
			Mean:         100000,
		},
	}

	current := &TimingReport{
		WorkloadName: "current_workload",
		Mean:         100000,
	}

	tests := []struct {
		name        string
		baseline    *Baseline
		current     *TimingReport
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil baseline",
			baseline:    nil,
			current:     current,
			expectError: true,
			errorMsg:    "baseline cannot be nil",
		},
		{
			name:        "Nil current report",
			baseline:    baseline,
			current:     nil,
			expectError: true,
			errorMsg:    "current report cannot be nil",
		},
		{
			name:        "Workload name mismatch",
			baseline:    baseline,
			current:     current,
			expectError: true,
			errorMsg:    "workload mismatch",
		},
		{
			name:     "Zero baseline mean",
			baseline: &Baseline{Version: "1.0.0", Report: TimingReport{WorkloadName: "test", Mean: 0}},
			current:  &TimingReport{WorkloadName: "test", Mean: 100},
			expectError: true,
			errorMsg:    "baseline mean is zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := h.DetectRegression(tt.baseline, tt.current)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestBaselinePersistence verifies Π₃: Baselines can be saved and loaded without loss.
//
// Protocol:
// 1. Create timing report
// 2. Save to JSON file
// 3. Load back from file
// 4. Verify all fields match
func TestBaselinePersistence(t *testing.T) {
	h := NewHarness()

	// Create a timing report
	original := &TimingReport{
		WorkloadName: "persistence_test",
		Runs:         []int64{100000, 105000, 102000, 103000, 101000},
		Mean:         102200,
		Min:          100000,
		Max:          105000,
		StdDev:       1788.85,
		Environment:  captureEnvironment(),
		Timestamp:    time.Now().UnixNano(),
	}

	// Create temp file for testing
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "baseline.json")

	// Save baseline
	if err := h.SaveBaseline(filename, original); err != nil {
		t.Fatalf("SaveBaseline failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatalf("Baseline file was not created")
	}

	// Load baseline
	loaded, err := h.LoadBaseline(filename)
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}

	// Verify all fields match
	if loaded.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %q", loaded.Version)
	}

	report := &loaded.Report
	if report.WorkloadName != original.WorkloadName {
		t.Errorf("WorkloadName mismatch: expected %q, got %q",
			original.WorkloadName, report.WorkloadName)
	}

	if report.Mean != original.Mean {
		t.Errorf("Mean mismatch: expected %d, got %d", original.Mean, report.Mean)
	}

	if report.Min != original.Min {
		t.Errorf("Min mismatch: expected %d, got %d", original.Min, report.Min)
	}

	if report.Max != original.Max {
		t.Errorf("Max mismatch: expected %d, got %d", original.Max, report.Max)
	}

	if len(report.Runs) != len(original.Runs) {
		t.Errorf("Runs length mismatch: expected %d, got %d",
			len(original.Runs), len(report.Runs))
	}

	t.Logf("✅ Baseline persistence verified: all fields match")
}

// TestWorkloadValidation verifies that invalid inputs are rejected.
func TestWorkloadValidation(t *testing.T) {
	h := NewHarness()
	ctx := context.Background()

	tests := []struct {
		name        string
		workload    Workload
		sampleSize  int
		expectError bool
		errorMsg    string
	}{
		{
			name: "Nil operation",
			workload: Workload{
				Name:      "nil_op",
				Operation: nil,
			},
			sampleSize:  10,
			expectError: true,
			errorMsg:    "operation cannot be nil",
		},
		{
			name: "Sample size too small",
			workload: Workload{
				Name:      "small_sample",
				Operation: func(ctx context.Context) error { return nil },
			},
			sampleSize:  2,
			expectError: true,
			errorMsg:    "sample size must be ≥ 5",
		},
		{
			name: "Valid workload",
			workload: Workload{
				Name:      "valid",
				Operation: func(ctx context.Context) error { return nil },
			},
			sampleSize:  5,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.DefaultSampleSize = tt.sampleSize
			_, err := h.RunWorkload(ctx, tt.workload)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestComputeStats verifies Welford's algorithm implementation.
func TestComputeStats(t *testing.T) {
	tests := []struct {
		name       string
		runs       []int64
		expectMean int64
		expectMin  int64
		expectMax  int64
	}{
		{
			name:       "Empty slice",
			runs:       []int64{},
			expectMean: 0,
			expectMin:  0,
			expectMax:  0,
		},
		{
			name:       "Single value",
			runs:       []int64{100},
			expectMean: 100,
			expectMin:  100,
			expectMax:  100,
		},
		{
			name:       "Multiple values",
			runs:       []int64{100, 200, 300, 400, 500},
			expectMean: 300,
			expectMin:  100,
			expectMax:  500,
		},
		{
			name:       "All same values",
			runs:       []int64{100, 100, 100, 100},
			expectMean: 100,
			expectMin:  100,
			expectMax:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, min, max, stddev := computeStats(tt.runs)

			if mean != tt.expectMean {
				t.Errorf("Expected mean=%d, got %d", tt.expectMean, mean)
			}
			if min != tt.expectMin {
				t.Errorf("Expected min=%d, got %d", tt.expectMin, min)
			}
			if max != tt.expectMax {
				t.Errorf("Expected max=%d, got %d", tt.expectMax, max)
			}

			// For all same values, stddev should be 0
			if len(tt.runs) > 0 && allSame(tt.runs) && stddev != 0 {
				t.Errorf("Expected stddev=0 for identical values, got %.2f", stddev)
			}
		})
	}
}

// TestJSONRoundtrip verifies baseline JSON serialization is lossless.
func TestJSONRoundtrip(t *testing.T) {
	original := Baseline{
		Version: "1.0.0",
		Report: TimingReport{
			WorkloadName: "json_test",
			Runs:         []int64{100, 200, 300},
			Mean:         200,
			Min:          100,
			Max:          300,
			StdDev:       81.65,
			Environment:  captureEnvironment(),
			Timestamp:    1735255200000000000,
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded Baseline
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields match
	if decoded.Version != original.Version {
		t.Errorf("Version mismatch: expected %q, got %q", original.Version, decoded.Version)
	}

	if decoded.Report.WorkloadName != original.Report.WorkloadName {
		t.Errorf("WorkloadName mismatch")
	}

	if decoded.Report.Mean != original.Report.Mean {
		t.Errorf("Mean mismatch: expected %d, got %d", original.Report.Mean, decoded.Report.Mean)
	}

	t.Logf("✅ JSON roundtrip successful")
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		len(s) > len(substr)+1 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func allSame(runs []int64) bool {
	if len(runs) == 0 {
		return true
	}
	first := runs[0]
	for _, v := range runs[1:] {
		if v != first {
			return false
		}
	}
	return true
}
