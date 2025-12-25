package ollama

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// ExampleMetricsUsage demonstrates how to use the MetricsCollector
// This is not a test, but a runnable example showing best practices
func ExampleMetricsUsage() {
	// Initialize the metrics collector
	mc := NewMetricsCollector()
	defer mc.Close()

	// Start a goroutine to monitor metrics and resource usage
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Update resource metrics every 5 seconds
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			memMB := m.Alloc / 1024 / 1024
			goroutineCount := runtime.NumGoroutine()

			// In a real application, you'd calculate actual CPU usage
			cpuUsage := 0.0

			mc.UpdateResourceMetrics(int64(memMB), cpuUsage, goroutineCount)
		}
	}()

	// Start a goroutine to listen for metrics events
	go func() {
		for event := range mc.MetricsChannel {
			fmt.Printf("Event: Type=%s, Model=%s, Time=%s\n",
				event.Type, event.Model, event.Timestamp.Format("15:04:05"))
		}
	}()

	// Simulate agent operations with multiple models
	models := []string{"llama2", "mistral", "neural-chat"}
	var wg sync.WaitGroup

	// Simulate concurrent requests to different models
	for _, model := range models {
		wg.Add(1)
		go func(modelName string) {
			defer wg.Done()

			// Simulate 20 requests per model
			for i := 0; i < 20; i++ {
				// Simulate latency (50-500ms)
				latency := time.Duration(50+i*20%400) * time.Millisecond
				tokenCount := int64(50 + i*10)

				// Record successful requests
				if i%5 != 0 { // 80% success rate
					start := time.Now()
					time.Sleep(latency) // Simulate processing

					mc.RecordLatency(modelName, time.Since(start))
					mc.RecordTokens(modelName, tokenCount)
					mc.RecordTaskCompletion(true, latency.Seconds()*1000)
				} else {
					// Record errors (20% failure rate)
					mc.RecordError(modelName, ErrorTimeout)
					mc.RecordTaskCompletion(false, 0)
				}

				time.Sleep(100 * time.Millisecond)
			}
		}(model)
	}

	// Wait for all operations to complete
	wg.Wait()

	// Give a moment for metrics events to be processed
	time.Sleep(1 * time.Second)

	// Print comprehensive metrics summary
	fmt.Println(mc.GetSummary())

	// Print per-model summaries
	for _, model := range models {
		summary, err := mc.GetModelSummary(model)
		if err != nil {
			log.Printf("Error getting summary for %s: %v\n", model, err)
			continue
		}
		fmt.Println(summary)

		// Print histogram summary
		histSummary, err := mc.GetHistogramSummary(model)
		if err != nil {
			log.Printf("Error getting histogram for %s: %v\n", model, err)
			continue
		}
		fmt.Println(histSummary)
	}

	// Export metrics to JSON
	jsonData, err := mc.ExportJSON()
	if err != nil {
		log.Fatalf("Error exporting JSON: %v\n", err)
	}

	fmt.Println("JSON Export (first 500 chars):")
	if len(jsonData) > 500 {
		fmt.Println(string(jsonData[:500]))
	} else {
		fmt.Println(string(jsonData))
	}

	// Export to file
	err = mc.ExportJSONToFile("/tmp/ollama_metrics.json")
	if err != nil {
		log.Printf("Error exporting to file: %v\n", err)
	} else {
		fmt.Println("Metrics exported to /tmp/ollama_metrics.json")
	}
}

// ExampleMetricsMonitoring demonstrates real-time metrics monitoring
func ExampleMetricsMonitoring() {
	mc := NewMetricsCollector()
	defer mc.Close()

	// Set up a metrics event listener
	go func() {
		for event := range mc.MetricsChannel {
			switch event.Type {
			case "latency":
				fmt.Printf("[LATENCY] Model: %s, Latency: %v\n", event.Model, event.Value)
			case "error":
				fmt.Printf("[ERROR] Model: %s, Error: %v\n", event.Model, event.Value)
			case "task_complete":
				fmt.Printf("[TASK] Success: %v\n", event.Value)
			case "resource":
				if res, ok := event.Value.(*ResourceMetrics); ok {
					fmt.Printf("[RESOURCE] Memory: %dMB, CPU: %.2f%%, Goroutines: %d\n",
						res.MemoryUsageMB, res.CPUUsagePercent, res.GoroutineCount)
				}
			}
		}
	}()

	// Simulate some activity
	model := "test-model"
	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordTokens(model, 200)
	mc.RecordTaskCompletion(true, 100)
	mc.UpdateResourceMetrics(512, 45.5, 10)

	time.Sleep(100 * time.Millisecond)
}

// ExampleMetricsWithPeriodicExport demonstrates periodic metrics export
func ExampleMetricsWithPeriodicExport(exportInterval time.Duration) {
	mc := NewMetricsCollector()
	defer mc.Close()

	// Create a ticker for periodic exports
	ticker := time.NewTicker(exportInterval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			// Export metrics to file with timestamp
			timestamp := time.Now().Format("2006-01-02_15-04-05")
			filename := fmt.Sprintf("/tmp/metrics_%s.json", timestamp)

			if err := mc.ExportJSONToFile(filename); err != nil {
				log.Printf("Failed to export metrics: %v\n", err)
			} else {
				fmt.Printf("Exported metrics to %s\n", filename)
			}

			// Print summary
			fmt.Println(mc.GetSummary())
		}
	}()

	// Keep the application running
	select {}
}

// ExampleMultiModelMetrics demonstrates tracking multiple models
func ExampleMultiModelMetrics() {
	mc := NewMetricsCollector()
	defer mc.Close()

	models := map[string]int{
		"llama2":      5,  // 5 requests
		"mistral":     10, // 10 requests
		"neural-chat": 8,  // 8 requests
		"dolphin":     12, // 12 requests
	}

	for model, count := range models {
		for i := 0; i < count; i++ {
			// Simulate variable latencies by model
			var latency time.Duration
			switch model {
			case "llama2":
				latency = time.Duration(100+i*10) * time.Millisecond
			case "mistral":
				latency = time.Duration(80+i*8) * time.Millisecond
			case "neural-chat":
				latency = time.Duration(120+i*12) * time.Millisecond
			case "dolphin":
				latency = time.Duration(90+i*9) * time.Millisecond
			}

			if i%3 == 0 {
				mc.RecordError(model, ErrorTimeout)
			} else {
				mc.RecordLatency(model, latency)
				mc.RecordTokens(model, int64(50+i*10))
			}
		}
	}

	// Get metrics for all models
	allMetrics := mc.GetAllModelMetrics()
	fmt.Println("=== ALL MODELS PERFORMANCE ===")
	for model, perfMetrics := range allMetrics {
		fmt.Printf("\n%s:\n", model)
		fmt.Printf("  Total: %d, Success: %d, Failed: %d\n",
			perfMetrics.TotalRequests, perfMetrics.SuccessfulReqs, perfMetrics.FailedReqs)
		fmt.Printf("  Latency: min=%v, avg=%v, max=%v\n",
			perfMetrics.MinLatency, perfMetrics.AvgLatency, perfMetrics.MaxLatency)
		fmt.Printf("  Throughput: %.2f req/s\n", perfMetrics.Throughput)
		fmt.Printf("  Error Rate: %.2f%%\n", perfMetrics.ErrorRate)
	}
}

// ExampleMetricsReset demonstrates resetting metrics
func ExampleMetricsReset() {
	mc := NewMetricsCollector()
	defer mc.Close()

	model := "test-model"

	// Record some metrics
	mc.RecordLatency(model, 100*time.Millisecond)
	mc.RecordLatency(model, 150*time.Millisecond)
	mc.RecordTaskCompletion(true, 100)

	fmt.Println("Before reset:")
	fmt.Println(mc.GetSummary())

	// Reset all metrics
	mc.ResetMetrics()

	fmt.Println("\nAfter reset:")
	fmt.Println(mc.GetSummary())
}

// ExampleThreadSafeMetrics demonstrates thread-safe operations
func ExampleThreadSafeMetrics() {
	mc := NewMetricsCollector()
	defer mc.Close()

	numWorkers := 5
	requestsPerWorker := 100
	var wg sync.WaitGroup

	// Start multiple worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < requestsPerWorker; i++ {
				model := fmt.Sprintf("model-%d", workerID%3)
				latency := time.Duration(50+i%100) * time.Millisecond

				if i%10 == 0 {
					mc.RecordError(model, ErrorTimeout)
				} else {
					mc.RecordLatency(model, latency)
					mc.RecordTokens(model, int64(i))
				}

				// Occasionally read metrics to verify thread-safety
				if i%25 == 0 {
					_, _ = mc.GetModelMetrics(model)
					_ = mc.GetTaskStatistics()
				}
			}
		}(w)
	}

	// Also start a reader goroutine
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = mc.GetAllModelMetrics()
				_ = mc.GetTaskStatistics()
				_ = mc.GetResourceMetrics()
			case <-done:
				return
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(done)

	// Print final summary
	fmt.Println(mc.GetSummary())
}
