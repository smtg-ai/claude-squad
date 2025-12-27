// +build ignore

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"

	agent8 "claude-squad/integrations/kgc/agent-8"
)

func main() {
	h := agent8.NewHarness()

	// Define reference workload for baseline
	workload := agent8.Workload{
		Name: "sha256_hash_chain_1000",
		Operation: func(ctx context.Context) error {
			data := []byte("reference workload for regression detection baseline")
			for i := 0; i < 1000; i++ {
				hash := sha256.Sum256(data)
				data = hash[:]
			}
			return nil
		},
	}

	ctx := context.Background()
	fmt.Println("Running workload to create baseline...")
	report, err := h.RunWorkload(ctx, workload)
	if err != nil {
		log.Fatalf("Failed to run workload: %v", err)
	}

	fmt.Printf("Workload: %s\n", report.WorkloadName)
	fmt.Printf("Mean: %d ns (%.2f μs)\n", report.Mean, float64(report.Mean)/1000.0)
	fmt.Printf("Min: %d ns\n", report.Min)
	fmt.Printf("Max: %d ns\n", report.Max)
	fmt.Printf("StdDev: %.2f ns\n", report.StdDev)

	filename := "baseline.json"
	if err := h.SaveBaseline(filename, report); err != nil {
		log.Fatalf("Failed to save baseline: %v", err)
	}

	fmt.Printf("\n✅ Baseline saved to %s\n", filename)
}
