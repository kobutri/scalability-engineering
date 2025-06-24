package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func persistenceDemo() {
	fmt.Println("🎯 HashSet Persistence Demo")
	fmt.Println(strings.Repeat("=", 50))

	// Setup
	dataFile := "demo_hashset.json"
	defer os.Remove(dataFile) // Clean up at end

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         dataFile,
		SnapshotInterval: 2 * time.Second, // Snapshot every 2 seconds
		MaxRetries:       3,
	}

	// Create HashSet with persistence
	fmt.Println("📦 Creating HashSet with persistence enabled...")
	set := NewHashSetWithPersistence[string](8, config)
	defer set.Close()

	// Add some initial data
	fmt.Println("➕ Adding initial data...")
	fruits := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for _, fruit := range fruits {
		set.Insert(fruit)
		fmt.Printf("   Added: %s (Version: %d, Size: %d)\n",
			fruit, set.GetCurrentVersion(), set.Size())
	}

	// Manually trigger a snapshot
	fmt.Println("\n📸 Triggering manual snapshot...")
	err := set.TriggerSnapshot()
	if err != nil {
		log.Printf("Snapshot error: %v", err)
	} else {
		fmt.Println("   ✅ Snapshot triggered successfully")
	}

	// Wait for snapshot to complete
	time.Sleep(500 * time.Millisecond)

	// Check if file exists
	if _, err := os.Stat(dataFile); err == nil {
		fmt.Printf("   💾 Snapshot file created: %s\n", dataFile)

		// Show file info
		if info, err := os.Stat(dataFile); err == nil {
			fmt.Printf("   📊 File size: %d bytes, Modified: %s\n",
				info.Size(), info.ModTime().Format("15:04:05"))
		}
	}

	// Simulate some ongoing operations
	fmt.Println("\n🔄 Simulating ongoing operations...")
	colors := []string{"red", "blue", "green", "yellow", "purple"}

	go func() {
		for i, color := range colors {
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
			set.Insert(color)
			fmt.Printf("   🎨 Added color: %s (Version: %d, Size: %d)\n",
				color, set.GetCurrentVersion(), set.Size())
		}
	}()

	// Let periodic snapshots happen
	fmt.Println("   ⏰ Waiting for periodic snapshots (every 2 seconds)...")
	time.Sleep(7 * time.Second)

	fmt.Printf("\n📊 Final state - Size: %d, Version: %d\n",
		set.Size(), set.GetCurrentVersion())

	// Test restoration
	fmt.Println("\n🔄 Testing restoration from disk...")

	// Create a new HashSet and restore from the saved file
	newSet := NewHashSetWithShards[string](8)
	err = newSet.LoadFromDisk(dataFile)
	if err != nil {
		log.Printf("Restoration error: %v", err)
		return
	}

	fmt.Printf("   ✅ Restored from disk - Size: %d, Version: %d\n",
		newSet.Size(), newSet.GetCurrentVersion())

	// Verify data integrity
	fmt.Println("   🔍 Verifying data integrity...")
	allData := append(fruits, colors...)
	missing := 0
	for _, item := range allData {
		if !newSet.Contains(item) {
			fmt.Printf("   ❌ Missing: %s\n", item)
			missing++
		}
	}

	if missing == 0 {
		fmt.Println("   ✅ All data restored successfully!")
	} else {
		fmt.Printf("   ⚠️  %d items missing (may be due to timing)\n", missing)
	}

	// Show some random elements
	fmt.Println("   🎲 Random elements from restored set:")
	for i := 0; i < 3; i++ {
		if elem, ok := newSet.RandomElement(); ok {
			fmt.Printf("      - %s\n", elem)
		}
	}

	fmt.Println("\n🎉 Demo completed successfully!")
}

func persistenceBenchmarkDemo() {
	fmt.Println("\n⚡ Performance Impact Demo")
	fmt.Println(strings.Repeat("=", 30))

	// Test without persistence
	fmt.Println("🏃 Testing WITHOUT persistence...")
	set1 := NewHashSetWithShards[string](32)

	start := time.Now()
	for i := 0; i < 50000; i++ {
		set1.Insert(fmt.Sprintf("item-%d", i))
		if i%2 == 0 {
			set1.Contains(fmt.Sprintf("item-%d", i/2))
		}
	}
	durationWithout := time.Since(start)
	fmt.Printf("   ⏱️  50,000 operations completed in: %v\n", durationWithout)

	// Test with persistence
	fmt.Println("🏃 Testing WITH persistence...")
	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         "benchmark_test.json",
		SnapshotInterval: 100 * time.Millisecond,
		MaxRetries:       3,
	}
	defer os.Remove("benchmark_test.json")

	set2 := NewHashSetWithPersistence[string](32, config)
	defer set2.Close()

	start = time.Now()
	for i := 0; i < 50000; i++ {
		set2.Insert(fmt.Sprintf("item-%d", i))
		if i%2 == 0 {
			set2.Contains(fmt.Sprintf("item-%d", i/2))
		}
	}
	durationWith := time.Since(start)
	fmt.Printf("   ⏱️  50,000 operations completed in: %v\n", durationWith)

	// Calculate overhead
	overhead := float64(durationWith-durationWithout) / float64(durationWithout) * 100
	fmt.Printf("   📈 Persistence overhead: %.2f%%\n", overhead)

	if overhead < 5 {
		fmt.Println("   ✅ Excellent! Very low overhead")
	} else if overhead < 20 {
		fmt.Println("   👍 Good! Acceptable overhead")
	} else {
		fmt.Println("   ⚠️  Higher overhead than expected")
	}
}

func gracefulShutdownDemo() {
	fmt.Println("\n🛑 Graceful Shutdown Demo")
	fmt.Println(strings.Repeat("=", 30))

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         "shutdown_test.json",
		SnapshotInterval: 5 * time.Second, // Long interval
		MaxRetries:       3,
	}
	defer os.Remove("shutdown_test.json")

	set := NewHashSetWithPersistence[string](8, config)

	// Add some data
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("data-%d", i))
	}

	fmt.Printf("   📊 Added 1000 items, Version: %d\n", set.GetCurrentVersion())

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("   🔧 Press Ctrl+C for graceful shutdown demo...")

	// Simulate some ongoing work
	go func() {
		for {
			select {
			case <-sigChan:
				return
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Wait a bit, then simulate shutdown
	time.Sleep(2 * time.Second)
	fmt.Println("   🛑 Simulating shutdown...")

	// Close gracefully - this will trigger final snapshot
	err := set.Close()
	if err != nil {
		log.Printf("Shutdown error: %v", err)
	} else {
		fmt.Println("   ✅ Graceful shutdown completed")
	}

	// Verify final snapshot was created
	if _, err := os.Stat("shutdown_test.json"); err == nil {
		fmt.Println("   💾 Final snapshot saved successfully")
	}
}

func runPersistenceDemo() {
	persistenceDemo()
	persistenceBenchmarkDemo()
	gracefulShutdownDemo()
}

// This file contains only demo functions, main() is in main.go
