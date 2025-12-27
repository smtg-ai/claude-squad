// Package agent8 implements a deterministic performance harness for regression detection.
//
// This package provides tools for measuring workload execution times and detecting
// performance regressions between commits. It is NOT a benchmarking tool - it is
// designed solely for reproducible regression detection.
package agent8

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"
)

// WorkloadFunc represents a deterministic operation to measure.
// The operation must be repeatable and produce consistent behavior across runs.
type WorkloadFunc func(ctx context.Context) error

// Workload defines a named, repeatable operation for timing measurement.
type Workload struct {
	Name      string
	Operation WorkloadFunc
}

// Environment captures system context for reproducibility verification.
type Environment struct {
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GoVersion string `json:"go_version"`
	NumCPU    int    `json:"num_cpu"`
}

// TimingReport captures timing statistics for N runs of a workload.
type TimingReport struct {
	WorkloadName string      `json:"workload_name"`
	Runs         []int64     `json:"runs_ns"`      // Durations in nanoseconds
	Mean         int64       `json:"mean_ns"`      // Mean duration
	Min          int64       `json:"min_ns"`       // Minimum duration
	Max          int64       `json:"max_ns"`       // Maximum duration
	StdDev       float64     `json:"stddev_ns"`    // Standard deviation
	Environment  Environment `json:"environment"`  // Execution environment
	Timestamp    int64       `json:"timestamp"`    // Unix nanoseconds
}

// Baseline represents a saved timing report for regression comparison.
type Baseline struct {
	Version string       `json:"version"` // Harness version
	Report  TimingReport `json:"report"`  // Timing measurements
}

// Harness provides deterministic workload measurement and regression detection.
type Harness struct {
	// DefaultSampleSize is the number of runs to perform (must be ≥ 5)
	DefaultSampleSize int

	// RegressionThreshold is the percent increase that triggers regression detection
	// Default: 10.0 (10%)
	RegressionThreshold float64
}

// NewHarness creates a new performance harness with sensible defaults.
func NewHarness() *Harness {
	return &Harness{
		DefaultSampleSize:   10,
		RegressionThreshold: 10.0, // 10% regression threshold
	}
}

// RunWorkload executes a workload N times and returns timing statistics.
//
// Algorithm:
// 1. Warmup run to prime caches
// 2. Execute workload N times
// 3. Compute statistics (mean, min, max, stddev)
// 4. Capture environment context
//
// Invariants:
// - sampleSize must be ≥ 5
// - All runs are executed sequentially
// - Results are deterministic modulo system variance
func (h *Harness) RunWorkload(ctx context.Context, workload Workload) (*TimingReport, error) {
	sampleSize := h.DefaultSampleSize
	if sampleSize < 5 {
		return nil, fmt.Errorf("sample size must be ≥ 5, got %d", sampleSize)
	}

	if workload.Operation == nil {
		return nil, fmt.Errorf("workload operation cannot be nil")
	}

	// Warmup run to prime caches and stabilize scheduler
	if err := workload.Operation(ctx); err != nil {
		return nil, fmt.Errorf("warmup run failed: %w", err)
	}

	// Measure N runs
	runs := make([]int64, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		start := time.Now()
		if err := workload.Operation(ctx); err != nil {
			return nil, fmt.Errorf("run %d failed: %w", i+1, err)
		}
		duration := time.Since(start)
		runs = append(runs, duration.Nanoseconds())
	}

	// Compute statistics using Welford's algorithm for numerical stability
	mean, min, max, stddev := computeStats(runs)

	// Capture environment for reproducibility context
	env := captureEnvironment()

	report := &TimingReport{
		WorkloadName: workload.Name,
		Runs:         runs,
		Mean:         mean,
		Min:          min,
		Max:          max,
		StdDev:       stddev,
		Environment:  env,
		Timestamp:    time.Now().UnixNano(),
	}

	return report, nil
}

// SaveBaseline persists a timing report as a baseline for future regression checks.
//
// The baseline is saved in JSON format with versioning to enable schema evolution.
//
// Invariants:
// - Report is serialized to valid JSON
// - File is written atomically (temp file + rename)
// - Baseline version is recorded
func (h *Harness) SaveBaseline(filename string, report *TimingReport) error {
	if report == nil {
		return fmt.Errorf("cannot save nil report")
	}

	baseline := Baseline{
		Version: "1.0.0",
		Report:  *report,
	}

	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baseline: %w", err)
	}

	// Write atomically: temp file + rename
	tmpFile := filename + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filename); err != nil {
		os.Remove(tmpFile) // Cleanup on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// LoadBaseline loads a previously saved baseline from JSON.
//
// Invariants:
// - File must be valid JSON matching Baseline schema
// - Version compatibility is checked
func (h *Harness) LoadBaseline(filename string) (*Baseline, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var baseline Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline: %w", err)
	}

	// Version compatibility check (currently only v1.0.0 supported)
	if baseline.Version != "1.0.0" {
		return nil, fmt.Errorf("unsupported baseline version: %s", baseline.Version)
	}

	return &baseline, nil
}

// DetectRegression compares current timing against baseline to detect performance degradation.
//
// Returns:
// - isRegression: true if current performance is worse than baseline by more than threshold
// - percentChange: the percent difference (positive = slower, negative = faster)
// - error: if comparison is invalid (e.g., mismatched workloads)
//
// Algorithm:
//   Δ = (current.Mean - baseline.Mean) / baseline.Mean × 100
//   isRegression = Δ > threshold
//
// Invariants:
// - Workload names must match
// - Baseline cannot be nil
// - Percent change is signed (positive = regression, negative = improvement)
func (h *Harness) DetectRegression(baseline *Baseline, current *TimingReport) (bool, float64, error) {
	if baseline == nil {
		return false, 0, fmt.Errorf("baseline cannot be nil")
	}
	if current == nil {
		return false, 0, fmt.Errorf("current report cannot be nil")
	}

	// Verify workload names match
	if baseline.Report.WorkloadName != current.WorkloadName {
		return false, 0, fmt.Errorf(
			"workload mismatch: baseline=%q, current=%q",
			baseline.Report.WorkloadName,
			current.WorkloadName,
		)
	}

	// Prevent division by zero
	if baseline.Report.Mean == 0 {
		return false, 0, fmt.Errorf("baseline mean is zero, cannot compute percent change")
	}

	// Compute percent change: Δ = (current - baseline) / baseline × 100
	delta := float64(current.Mean-baseline.Report.Mean) / float64(baseline.Report.Mean) * 100.0

	// Check if change exceeds threshold (positive delta = regression)
	isRegression := delta > h.RegressionThreshold

	return isRegression, delta, nil
}

// computeStats calculates mean, min, max, and standard deviation using Welford's algorithm.
//
// Welford's algorithm is numerically stable and computes variance in a single pass.
//
// Reference: Welford, B.P. (1962). "Note on a method for calculating corrected sums of
// squares and products". Technometrics. 4 (3): 419–420.
func computeStats(runs []int64) (mean int64, min int64, max int64, stddev float64) {
	n := len(runs)
	if n == 0 {
		return 0, 0, 0, 0
	}

	// Initialize
	min = runs[0]
	max = runs[0]
	var m float64 = 0     // Running mean
	var m2 float64 = 0    // Running sum of squared deviations

	// Welford's online algorithm for numerical stability
	for i, x := range runs {
		// Update min/max
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}

		// Welford's algorithm for mean and variance
		delta := float64(x) - m
		m += delta / float64(i+1)
		delta2 := float64(x) - m
		m2 += delta * delta2
	}

	mean = int64(m)

	// Standard deviation (population, not sample)
	if n > 1 {
		variance := m2 / float64(n)
		stddev = math.Sqrt(variance)
	}

	return mean, min, max, stddev
}

// captureEnvironment records system context for reproducibility verification.
func captureEnvironment() Environment {
	return Environment{
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		GoVersion: runtime.Version(),
		NumCPU:    runtime.NumCPU(),
	}
}
