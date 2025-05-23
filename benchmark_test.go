package main

import (
	"chronos/session"
	"chronos/session/tmux"
	"chronos/ui"
	"runtime"
	"testing"
	"time"
)

// BenchmarkInstanceCreation measures instance creation performance
func BenchmarkInstanceCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instance := session.NewInstance("test", "Test Instance", "echo test", false)
		_ = instance
	}
}

// BenchmarkStringBuilder measures string building performance
func BenchmarkStringBuilder(b *testing.B) {
	menu := ui.NewMenu()
	menu.SetSize(80, 24)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = menu.String()
	}
}

// BenchmarkTmuxSessionCreation measures tmux session creation
func BenchmarkTmuxSessionCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session := tmux.NewTmuxSession("test", "echo test")
		_ = session
	}
}

// BenchmarkMemoryUsage measures memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	var m1, m2 runtime.MemStats
	
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	instances := make([]*session.Instance, b.N)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instances[i] = session.NewInstance("test", "Test Instance", "echo test", false)
		instances[i].SetMetadata("test", "value")
	}
	
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/instance")
}

// BenchmarkCacheOperations measures cache performance
func BenchmarkCacheOperations(b *testing.B) {
	cache := session.NewCache(5 * time.Second)
	
	b.ResetTimer()
	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.Set("key", "value")
		}
	})
	
	b.Run("Get", func(b *testing.B) {
		cache.Set("key", "value")
		for i := 0; i < b.N; i++ {
			_, _ = cache.Get("key")
		}
	})
}

// BenchmarkWorkerPool measures worker pool performance
func BenchmarkWorkerPool(b *testing.B) {
	wp := session.NewWorkerPool(4)
	wp.Start()
	defer wp.Shutdown()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wp.Submit(func() error {
			time.Sleep(1 * time.Millisecond)
			return nil
		})
	}
	wp.Wait()
}