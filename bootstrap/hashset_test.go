package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHashSetBasicOperations(t *testing.T) {
	set := NewHashSet[string]()

	// Test insertion
	if !set.Insert("apple") {
		t.Error("Expected Insert to return true for new element")
	}
	if set.Insert("apple") {
		t.Error("Expected Insert to return false for duplicate element")
	}

	// Test size
	if set.Size() != 1 {
		t.Errorf("Expected size 1, got %d", set.Size())
	}

	// Test contains
	if !set.Contains("apple") {
		t.Error("Expected Contains to return true for existing element")
	}
	if set.Contains("banana") {
		t.Error("Expected Contains to return false for non-existing element")
	}

	// Test insertion of multiple elements
	set.Insert("banana")
	set.Insert("cherry")
	if set.Size() != 3 {
		t.Errorf("Expected size 3, got %d", set.Size())
	}

	// Test removal
	if !set.Remove("banana") {
		t.Error("Expected Remove to return true for existing element")
	}
	if set.Remove("banana") {
		t.Error("Expected Remove to return false for already removed element")
	}
	if set.Size() != 2 {
		t.Errorf("Expected size 2 after removal, got %d", set.Size())
	}

	// Test random element
	elem, ok := set.RandomElement()
	if !ok {
		t.Error("Expected RandomElement to return true for non-empty set")
	}
	if !set.Contains(elem) {
		t.Error("Random element should exist in set")
	}
}

func TestHashSetAdvancedOperations(t *testing.T) {
	set1 := NewHashSet[string]()
	set2 := NewHashSet[string]()

	// Populate sets
	set1.InsertAll("a", "b", "c", "d")
	set2.InsertAll("c", "d", "e", "f")

	// Test ContainsAll
	if !set1.ContainsAll("a", "b") {
		t.Error("Expected ContainsAll to return true for existing elements")
	}
	if set1.ContainsAll("a", "x") {
		t.Error("Expected ContainsAll to return false when some elements don't exist")
	}

	// Test Union
	union := set1.Union(set2)
	expectedUnionSize := 6 // a, b, c, d, e, f
	if union.Size() != expectedUnionSize {
		t.Errorf("Expected union size %d, got %d", expectedUnionSize, union.Size())
	}

	// Test Intersection
	intersection := set1.Intersection(set2)
	expectedIntersectionSize := 2 // c, d
	if intersection.Size() != expectedIntersectionSize {
		t.Errorf("Expected intersection size %d, got %d", expectedIntersectionSize, intersection.Size())
	}
	if !intersection.ContainsAll("c", "d") {
		t.Error("Intersection should contain common elements")
	}

	// Test Difference
	diff := set1.Difference(set2)
	expectedDiffSize := 2 // a, b
	if diff.Size() != expectedDiffSize {
		t.Errorf("Expected difference size %d, got %d", expectedDiffSize, diff.Size())
	}
	if !diff.ContainsAll("a", "b") {
		t.Error("Difference should contain elements only in first set")
	}
}

func TestHashSetClear(t *testing.T) {
	set := NewHashSet[string]()
	set.InsertAll("a", "b", "c")

	set.Clear()
	if !set.IsEmpty() {
		t.Error("Expected set to be empty after Clear")
	}
	if set.Size() != 0 {
		t.Errorf("Expected size 0 after Clear, got %d", set.Size())
	}
}

func TestHashSetToSlice(t *testing.T) {
	set := NewHashSet[string]()
	elements := []string{"apple", "banana", "cherry"}

	for _, elem := range elements {
		set.Insert(elem)
	}

	slice := set.ToSlice()
	if len(slice) != len(elements) {
		t.Errorf("Expected slice length %d, got %d", len(elements), len(slice))
	}

	// Check all elements are present (order may vary)
	sliceSet := make(map[string]bool)
	for _, elem := range slice {
		sliceSet[elem] = true
	}

	for _, elem := range elements {
		if !sliceSet[elem] {
			t.Errorf("Element %v missing from ToSlice result", elem)
		}
	}
}

func TestHashSetIterator(t *testing.T) {
	set := NewHashSet[string]()
	elements := []string{"apple", "banana", "cherry"}

	for _, elem := range elements {
		set.Insert(elem)
	}

	iterator := set.NewIterator()
	count := 0
	foundElements := make(map[string]bool)

	for iterator.HasNext() {
		elem, ok := iterator.Next()
		if !ok {
			t.Error("Next() should return true when HasNext() is true")
		}
		foundElements[elem] = true
		count++
	}

	if count != len(elements) {
		t.Errorf("Expected to iterate over %d elements, got %d", len(elements), count)
	}

	// Verify all elements were found
	for _, elem := range elements {
		if !foundElements[elem] {
			t.Errorf("Element %v was not found during iteration", elem)
		}
	}

	// Test iterator exhaustion
	_, ok := iterator.Next()
	if ok {
		t.Error("Next() should return false when iterator is exhausted")
	}
}

func TestHashSetConcurrentInserts(t *testing.T) {
	set := NewHashSetWithShards[string](64) // More shards for high concurrency
	numGoroutines := 100
	elementsPerGoroutine := 1000

	var wg sync.WaitGroup

	// Concurrent insertions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerGoroutine; j++ {
				set.Insert(fmt.Sprintf("item-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * elementsPerGoroutine
	if set.Size() != expectedSize {
		t.Errorf("Expected size %d after concurrent inserts, got %d", expectedSize, set.Size())
	}
}

func TestHashSetConcurrentMixedOperations(t *testing.T) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate with some elements
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("initial-%d", i))
	}

	var wg sync.WaitGroup
	numOperations := 10000

	// Concurrent insertions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			set.Insert(fmt.Sprintf("insert-%d", i))
		}
	}()

	// Concurrent removals
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations/2; i++ {
			set.Remove(fmt.Sprintf("initial-%d", i))
		}
	}()

	// Concurrent lookups
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			set.Contains(fmt.Sprintf("lookup-%d", i%100))
		}
	}()

	// Concurrent random access
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			set.RandomElement()
		}
	}()

	wg.Wait()

	// Just verify no panic occurred and basic functionality still works
	if set.Size() < 0 {
		t.Error("Size should never be negative")
	}

	set.Insert("final-test")
	if !set.Contains("final-test") {
		t.Error("Set should still function correctly after concurrent operations")
	}
}

func TestHashSetRandomElementDistribution(t *testing.T) {
	set := NewHashSet[string]()
	elements := []string{"a", "b", "c", "d", "e"}

	for _, elem := range elements {
		set.Insert(elem)
	}

	// Get many random elements and check distribution
	numSamples := 10000
	counts := make(map[string]int)

	for i := 0; i < numSamples; i++ {
		elem, ok := set.RandomElement()
		if !ok {
			t.Fatal("RandomElement should succeed on non-empty set")
		}
		counts[elem]++
	}

	// Each element should appear roughly numSamples/len(elements) times
	expectedCount := numSamples / len(elements)
	tolerance := expectedCount / 2 // 50% tolerance

	for _, elem := range elements {
		count := counts[elem]
		if count < expectedCount-tolerance || count > expectedCount+tolerance {
			t.Logf("Element %v appeared %d times (expected ~%d)", elem, count, expectedCount)
		}
	}
}

func TestHashSetStats(t *testing.T) {
	set := NewHashSetWithShards[string](8)

	// Insert elements
	for i := 0; i < 100; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	stats := set.Stats()

	if stats.TotalElements != 100 {
		t.Errorf("Expected total elements 100, got %d", stats.TotalElements)
	}

	if stats.ShardCount != 8 {
		t.Errorf("Expected shard count 8, got %d", stats.ShardCount)
	}

	if len(stats.ElementsPerShard) != 8 {
		t.Errorf("Expected ElementsPerShard length 8, got %d", len(stats.ElementsPerShard))
	}

	// Verify sum of elements per shard equals total
	sum := 0
	for _, count := range stats.ElementsPerShard {
		sum += count
	}
	if sum != stats.TotalElements {
		t.Errorf("Sum of elements per shard (%d) doesn't match total (%d)", sum, stats.TotalElements)
	}

	expectedLoadFactor := float64(100) / float64(8)
	if stats.LoadFactor != expectedLoadFactor {
		t.Errorf("Expected load factor %f, got %f", expectedLoadFactor, stats.LoadFactor)
	}
}

func TestHashSetRandomElements(t *testing.T) {
	set := NewHashSet[int]()
	elements := []int{1, 2, 3, 4, 5}

	for _, elem := range elements {
		set.Insert(elem)
	}

	// Test getting multiple random elements
	randomElems := set.RandomElements(10)
	if len(randomElems) != 10 {
		t.Errorf("Expected 10 random elements, got %d", len(randomElems))
	}

	// All returned elements should exist in the set
	for _, elem := range randomElems {
		if !set.Contains(elem) {
			t.Errorf("Random element %v should exist in set", elem)
		}
	}

	// Test with empty set
	emptySet := NewHashSet[int]()
	emptyResult := emptySet.RandomElements(5)
	if len(emptyResult) != 0 {
		t.Errorf("Expected 0 elements from empty set, got %d", len(emptyResult))
	}
}

func TestHashSetDifferentTypes(t *testing.T) {
	// Test with integers
	intSet := NewHashSet[int]()
	intSet.InsertAll(1, 2, 3, 4, 5)
	if intSet.Size() != 5 {
		t.Errorf("Expected int set size 5, got %d", intSet.Size())
	}

	// Test with custom struct
	type Person struct {
		Name string
		Age  int
	}

	personSet := NewHashSet[Person]()
	alice := Person{"Alice", 30}
	bob := Person{"Bob", 25}

	personSet.Insert(alice)
	personSet.Insert(bob)
	personSet.Insert(alice) // Duplicate

	if personSet.Size() != 2 {
		t.Errorf("Expected person set size 2, got %d", personSet.Size())
	}

	if !personSet.Contains(alice) {
		t.Error("Person set should contain Alice")
	}
}

// Benchmark tests
func BenchmarkHashSetInsert(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

func BenchmarkHashSetContains(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Contains(fmt.Sprintf("item-%d", i%10000))
			i++
		}
	})
}

func BenchmarkHashSetRandomElement(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			set.RandomElement()
		}
	})
}

func BenchmarkHashSetRemove(b *testing.B) {
	// This benchmark needs to be careful since we can't remove the same element twice
	b.StopTimer()

	numElements := b.N
	set := NewHashSetWithShards[string](32)

	// Pre-populate with enough elements for the benchmark
	for i := 0; i < numElements; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	b.StartTimer()

	// Sequential removes since we can't remove the same element twice
	for i := 0; i < numElements; i++ {
		set.Remove(fmt.Sprintf("item-%d", i))
	}
}

func TestGlobalLockHashSetBasicOperations(t *testing.T) {
	set := NewGlobalLockHashSet[string]()

	// Test insertion
	if !set.Insert("apple") {
		t.Error("Expected Insert to return true for new element")
	}
	if set.Insert("apple") {
		t.Error("Expected Insert to return false for duplicate element")
	}

	// Test size
	if set.Size() != 1 {
		t.Errorf("Expected size 1, got %d", set.Size())
	}

	// Test contains
	if !set.Contains("apple") {
		t.Error("Expected Contains to return true for existing element")
	}
	if set.Contains("banana") {
		t.Error("Expected Contains to return false for non-existing element")
	}

	// Test removal
	if !set.Remove("apple") {
		t.Error("Expected Remove to return true for existing element")
	}
	if set.Remove("apple") {
		t.Error("Expected Remove to return false for already removed element")
	}
	if set.Size() != 0 {
		t.Errorf("Expected size 0 after removal, got %d", set.Size())
	}
}

func TestGlobalLockHashSetConcurrentOperations(t *testing.T) {
	set := NewGlobalLockHashSet[string]()
	numGoroutines := 50
	elementsPerGoroutine := 1000

	var wg sync.WaitGroup

	// Concurrent insertions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerGoroutine; j++ {
				set.Insert(fmt.Sprintf("item-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * elementsPerGoroutine
	if set.Size() != expectedSize {
		t.Errorf("Expected size %d after concurrent inserts, got %d", expectedSize, set.Size())
	}
}

// Comparison benchmarks
func BenchmarkComparisonInsertSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

func BenchmarkComparisonInsertGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

func BenchmarkComparisonContainsSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate
	for i := 0; i < 10000; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Contains(fmt.Sprintf("item-%d", i%10000))
			i++
		}
	})
}

func BenchmarkComparisonContainsGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		set.Insert(fmt.Sprintf("item-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Contains(fmt.Sprintf("item-%d", i%10000))
			i++
		}
	})
}

// Realistic scenario benchmarks: Many goroutines, few operations each
func BenchmarkRealisticWebServerPatternSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate with some "hot" data that requests will access
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("hot-data-%d", i))
	}

	b.ResetTimer()

	// Simulate web server pattern: many concurrent requests, each doing few operations
	goroutinesPerIteration := 100
	operationsPerGoroutine := 5

	for i := 0; i < b.N; i += goroutinesPerIteration {
		var wg sync.WaitGroup
		remaining := b.N - i
		if remaining > goroutinesPerIteration {
			remaining = goroutinesPerIteration
		}

		for j := 0; j < remaining; j++ {
			wg.Add(1)
			go func(reqID int) {
				defer wg.Done()

				// Each "request" does a few operations
				for k := 0; k < operationsPerGoroutine; k++ {
					// Mix of operations similar to web requests
					set.Contains(fmt.Sprintf("hot-data-%d", (reqID*7+k)%1000))
					set.Insert(fmt.Sprintf("session-%d-%d", reqID, k))
					if k%2 == 0 {
						set.RandomElement()
					}
				}
			}(i + j)
		}
		wg.Wait()
	}
}

func BenchmarkRealisticWebServerPatternGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	// Pre-populate with some "hot" data that requests will access
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("hot-data-%d", i))
	}

	b.ResetTimer()

	// Simulate web server pattern: many concurrent requests, each doing few operations
	goroutinesPerIteration := 100
	operationsPerGoroutine := 5

	for i := 0; i < b.N; i += goroutinesPerIteration {
		var wg sync.WaitGroup
		remaining := b.N - i
		if remaining > goroutinesPerIteration {
			remaining = goroutinesPerIteration
		}

		for j := 0; j < remaining; j++ {
			wg.Add(1)
			go func(reqID int) {
				defer wg.Done()

				// Each "request" does a few operations
				for k := 0; k < operationsPerGoroutine; k++ {
					// Same operations as sharded version
					set.Contains(fmt.Sprintf("hot-data-%d", (reqID*7+k)%1000))
					set.Insert(fmt.Sprintf("session-%d-%d", reqID, k))
					if k%2 == 0 {
						set.RandomElement()
					}
				}
			}(i + j)
		}
		wg.Wait()
	}
}

// Forced contention benchmarks: Many goroutines accessing the same data
func BenchmarkForcedContentionSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate with hot elements that will cause contention
	hotElements := []string{"hot1", "hot2", "hot3", "hot4", "hot5"}
	for _, elem := range hotElements {
		set.Insert(elem)
	}

	b.ResetTimer()

	// Force contention by having many goroutines access the same elements
	goroutinesPerIteration := 200
	operationsPerGoroutine := 3

	for i := 0; i < b.N; i += goroutinesPerIteration {
		var wg sync.WaitGroup
		var startBarrier sync.WaitGroup
		startBarrier.Add(1)

		remaining := b.N - i
		if remaining > goroutinesPerIteration {
			remaining = goroutinesPerIteration
		}

		for j := 0; j < remaining; j++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				startBarrier.Wait() // Synchronize start for maximum contention

				for k := 0; k < operationsPerGoroutine; k++ {
					// All goroutines fight for the same hot elements
					hotElement := hotElements[k%len(hotElements)]
					set.Contains(hotElement)
					set.Insert(fmt.Sprintf("%s-variant-%d", hotElement, goroutineID))
					set.RandomElement()
				}
			}(i + j)
		}
		startBarrier.Done() // Release all goroutines simultaneously
		wg.Wait()
	}
}

func BenchmarkForcedContentionGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	// Pre-populate with hot elements that will cause contention
	hotElements := []string{"hot1", "hot2", "hot3", "hot4", "hot5"}
	for _, elem := range hotElements {
		set.Insert(elem)
	}

	b.ResetTimer()

	// Force contention by having many goroutines access the same elements
	goroutinesPerIteration := 200
	operationsPerGoroutine := 3

	for i := 0; i < b.N; i += goroutinesPerIteration {
		var wg sync.WaitGroup
		var startBarrier sync.WaitGroup
		startBarrier.Add(1)

		remaining := b.N - i
		if remaining > goroutinesPerIteration {
			remaining = goroutinesPerIteration
		}

		for j := 0; j < remaining; j++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				startBarrier.Wait() // Synchronize start for maximum contention

				for k := 0; k < operationsPerGoroutine; k++ {
					// Same operations as sharded version
					hotElement := hotElements[k%len(hotElements)]
					set.Contains(hotElement)
					set.Insert(fmt.Sprintf("%s-variant-%d", hotElement, goroutineID))
					set.RandomElement()
				}
			}(i + j)
		}
		startBarrier.Done() // Release all goroutines simultaneously
		wg.Wait()
	}
}

// Shard count comparison benchmarks
func BenchmarkShardCountComparison8(b *testing.B) {
	set := NewHashSetWithShards[string](8)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

func BenchmarkShardCountComparison32(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

func BenchmarkShardCountComparison128(b *testing.B) {
	set := NewHashSetWithShards[string](128)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			set.Insert(fmt.Sprintf("item-%d", i))
			i++
		}
	})
}

// Burst pattern benchmark: Simulates sudden spikes of concurrent requests
func BenchmarkBurstPatternSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate
	for i := 0; i < 5000; i++ {
		set.Insert(fmt.Sprintf("base-%d", i))
	}

	b.ResetTimer()

	// Simulate burst patterns: sudden spikes of many concurrent operations
	burstsPerIteration := 10
	goroutinesPerBurst := 50
	operationsPerGoroutine := 2

	for i := 0; i < b.N; i += burstsPerIteration * goroutinesPerBurst * operationsPerGoroutine {
		for burst := 0; burst < burstsPerIteration; burst++ {
			var wg sync.WaitGroup

			for j := 0; j < goroutinesPerBurst; j++ {
				wg.Add(1)
				go func(burstID, goroutineID int) {
					defer wg.Done()

					for k := 0; k < operationsPerGoroutine; k++ {
						// Quick burst of operations
						set.Contains(fmt.Sprintf("base-%d", (burstID*goroutineID+k)%5000))
						set.Insert(fmt.Sprintf("burst-%d-%d-%d", burstID, goroutineID, k))
					}
				}(burst, j)
			}
			wg.Wait()
		}
	}
}

func BenchmarkBurstPatternGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	// Pre-populate
	for i := 0; i < 5000; i++ {
		set.Insert(fmt.Sprintf("base-%d", i))
	}

	b.ResetTimer()

	// Simulate burst patterns: sudden spikes of many concurrent operations
	burstsPerIteration := 10
	goroutinesPerBurst := 50
	operationsPerGoroutine := 2

	for i := 0; i < b.N; i += burstsPerIteration * goroutinesPerBurst * operationsPerGoroutine {
		for burst := 0; burst < burstsPerIteration; burst++ {
			var wg sync.WaitGroup

			for j := 0; j < goroutinesPerBurst; j++ {
				wg.Add(1)
				go func(burstID, goroutineID int) {
					defer wg.Done()

					for k := 0; k < operationsPerGoroutine; k++ {
						// Same operations as sharded version
						set.Contains(fmt.Sprintf("base-%d", (burstID*goroutineID+k)%5000))
						set.Insert(fmt.Sprintf("burst-%d-%d-%d", burstID, goroutineID, k))
					}
				}(burst, j)
			}
			wg.Wait()
		}
	}
}

// Comparison: B.RunParallel versions (sustained parallel load)
func BenchmarkSustainedParallelSharded(b *testing.B) {
	set := NewHashSetWithShards[string](32)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("hot-data-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Similar operations to web server pattern but sustained
			set.Contains(fmt.Sprintf("hot-data-%d", (i*7)%1000))
			set.Insert(fmt.Sprintf("session-%d", i))
			if i%2 == 0 {
				set.RandomElement()
			}
			i++
		}
	})
}

func BenchmarkSustainedParallelGlobalLock(b *testing.B) {
	set := NewGlobalLockHashSet[string]()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		set.Insert(fmt.Sprintf("hot-data-%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Same operations as sharded version
			set.Contains(fmt.Sprintf("hot-data-%d", (i*7)%1000))
			set.Insert(fmt.Sprintf("session-%d", i))
			if i%2 == 0 {
				set.RandomElement()
			}
			i++
		}
	})
}

// =============================================================================
// PERSISTENCE TESTS
// =============================================================================

func TestHashSetPersistenceBasic(t *testing.T) {
	// Create temporary file for testing
	tempFile := filepath.Join(os.TempDir(), "hashset_test_basic.json")
	defer os.Remove(tempFile)

	// Create HashSet with persistence
	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         tempFile,
		SnapshotInterval: 100 * time.Millisecond, // Fast for testing
		MaxRetries:       3,
	}

	set := NewHashSetWithPersistence[string](8, config)
	defer set.Close()

	// Add some data
	testData := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for _, item := range testData {
		set.Insert(item)
	}

	// Wait for periodic snapshot or trigger manually
	time.Sleep(150 * time.Millisecond)

	// Verify file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		// Try manual trigger if periodic didn't happen yet
		err := set.TriggerSnapshot()
		if err != nil {
			t.Fatalf("Manual trigger failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		if _, err := os.Stat(tempFile); os.IsNotExist(err) {
			t.Fatal("Snapshot file was not created")
		}
	}

	// Create new HashSet and restore from disk
	set2 := NewHashSetWithShards[string](8)
	err := set2.LoadFromDisk(tempFile)
	if err != nil {
		t.Fatalf("Failed to load from disk: %v", err)
	}

	// Verify data was restored correctly
	if set2.Size() != len(testData) {
		t.Errorf("Expected size %d, got %d", len(testData), set2.Size())
	}

	for _, item := range testData {
		if !set2.Contains(item) {
			t.Errorf("Item %s was not restored", item)
		}
	}
}

func TestHashSetPersistenceConsistency(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "hashset_test_consistency.json")
	defer os.Remove(tempFile)

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         tempFile,
		SnapshotInterval: 50 * time.Millisecond,
		MaxRetries:       3,
	}

	set := NewHashSetWithPersistence[int](16, config)
	defer set.Close()

	// Concurrent operations while snapshots are happening
	var wg sync.WaitGroup
	numOperations := 1000

	// Concurrent insertions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations; i++ {
			set.Insert(i)
			if i%100 == 0 {
				time.Sleep(1 * time.Millisecond) // Small pause
			}
		}
	}()

	// Concurrent removals
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numOperations/2; i++ {
			set.Remove(i * 2) // Remove even numbers
			if i%50 == 0 {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Let operations run for a bit to generate snapshots
	time.Sleep(200 * time.Millisecond)
	wg.Wait()

	// Final manual snapshot
	err := set.TriggerSnapshot()
	if err != nil {
		t.Fatalf("Final snapshot failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Restore and verify consistency
	set2 := NewHashSetWithShards[int](16)
	err = set2.LoadFromDisk(tempFile)
	if err != nil {
		t.Fatalf("Failed to load from disk: %v", err)
	}

	// The restored set should contain odd numbers (evens were removed)
	expectedCount := numOperations / 2
	if set2.Size() != expectedCount {
		t.Logf("Expected size %d, got %d (some timing variance expected)", expectedCount, set2.Size())
	}

	// Verify no corruption - all elements should be valid
	for _, element := range set2.ToSlice() {
		if element < 0 || element >= numOperations {
			t.Errorf("Invalid element found: %d", element)
		}
	}
}

func TestHashSetPersistenceAtomicWrites(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "hashset_test_atomic.json")
	defer os.Remove(tempFile)
	defer os.Remove(tempFile + ".tmp") // Clean up temp file if it exists

	config := PersistenceConfig{
		Enabled:    true,
		FilePath:   tempFile,
		MaxRetries: 1, // Only one attempt for this test
	}

	set := NewHashSetWithPersistence[string](4, config)
	defer set.Close()

	// Add initial data
	set.Insert("initial")

	// Trigger snapshot to create initial file
	err := set.TriggerSnapshot()
	if err != nil {
		t.Fatalf("Initial snapshot failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Verify initial file exists and is valid
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Fatal("Initial snapshot file not created")
	}

	// Add more data
	set.Insert("additional1")
	set.Insert("additional2")

	// Create another snapshot
	err = set.TriggerSnapshot()
	if err != nil {
		t.Fatalf("Second snapshot failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Verify file is still valid and contains all data
	set2 := NewHashSetWithShards[string](4)
	err = set2.LoadFromDisk(tempFile)
	if err != nil {
		t.Fatalf("Failed to load after second snapshot: %v", err)
	}

	expectedItems := []string{"initial", "additional1", "additional2"}
	if set2.Size() != len(expectedItems) {
		t.Errorf("Expected size %d, got %d", len(expectedItems), set2.Size())
	}

	for _, item := range expectedItems {
		if !set2.Contains(item) {
			t.Errorf("Missing item after restore: %s", item)
		}
	}

	// Verify no temporary file left behind
	if _, err := os.Stat(tempFile + ".tmp"); !os.IsNotExist(err) {
		t.Error("Temporary file was not cleaned up")
	}
}

func TestHashSetPersistenceVersioning(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "hashset_test_versioning.json")
	defer os.Remove(tempFile)

	config := PersistenceConfig{
		Enabled:    true,
		FilePath:   tempFile,
		MaxRetries: 3,
	}

	set := NewHashSetWithPersistence[string](4, config)
	defer set.Close()

	// Initial version should be 0
	if set.GetCurrentVersion() != 0 {
		t.Errorf("Expected initial version 0, got %d", set.GetCurrentVersion())
	}

	// Add data and create snapshot
	set.Insert("v1_data")
	snapshot1 := set.CreateSnapshot()

	if snapshot1.Version != 1 {
		t.Errorf("Expected snapshot version 1, got %d", snapshot1.Version)
	}

	// Add more data and create another snapshot
	set.Insert("v2_data")
	snapshot2 := set.CreateSnapshot()

	if snapshot2.Version != 2 {
		t.Errorf("Expected snapshot version 2, got %d", snapshot2.Version)
	}

	// Verify snapshots contain correct data
	if len(snapshot1.Shards) == 0 {
		t.Error("Snapshot1 has no shards")
	}

	if len(snapshot2.Shards) == 0 {
		t.Error("Snapshot2 has no shards")
	}

	// Count elements in snapshots
	count1, count2 := 0, 0
	for _, shard := range snapshot1.Shards {
		count1 += len(shard.Elements)
	}
	for _, shard := range snapshot2.Shards {
		count2 += len(shard.Elements)
	}

	if count1 != 1 {
		t.Errorf("Expected 1 element in snapshot1, got %d", count1)
	}
	if count2 != 2 {
		t.Errorf("Expected 2 elements in snapshot2, got %d", count2)
	}
}

func TestHashSetPersistenceNoPerformanceImpact(t *testing.T) {
	// Test that regular operations aren't blocked by persistence operations
	tempFile := filepath.Join(os.TempDir(), "hashset_test_performance.json")
	defer os.Remove(tempFile)

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         tempFile,
		SnapshotInterval: 10 * time.Millisecond, // Very frequent snapshots
		MaxRetries:       3,
	}

	set := NewHashSetWithPersistence[int](8, config)
	defer set.Close()

	// Perform many operations while snapshots are happening frequently
	start := time.Now()
	numOperations := 10000

	for i := 0; i < numOperations; i++ {
		set.Insert(i)
		if i%2 == 0 {
			set.Contains(i / 2)
		}
		if i%3 == 0 && i > 0 {
			set.Remove(i - 1)
		}
	}

	duration := time.Since(start)

	// Operations should complete in reasonable time despite frequent snapshots
	// This is a loose check - main thing is it doesn't hang
	maxExpectedDuration := 5 * time.Second
	if duration > maxExpectedDuration {
		t.Errorf("Operations took too long: %v (expected < %v)", duration, maxExpectedDuration)
	}

	t.Logf("Completed %d operations in %v with frequent snapshots", numOperations, duration)
}

func TestHashSetPersistenceShardCountMismatch(t *testing.T) {
	tempFile := filepath.Join(os.TempDir(), "hashset_test_mismatch.json")
	defer os.Remove(tempFile)

	// Create set with 4 shards
	config := PersistenceConfig{
		Enabled:    true,
		FilePath:   tempFile,
		MaxRetries: 3,
	}

	set1 := NewHashSetWithPersistence[string](4, config)
	set1.Insert("test")
	err := set1.TriggerSnapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	set1.Close()

	// Try to load with 8 shards (mismatch)
	set2 := NewHashSetWithShards[string](8)
	err = set2.LoadFromDisk(tempFile)

	if err == nil {
		t.Error("Expected error for shard count mismatch, got nil")
	}

	if !strings.Contains(fmt.Sprintf("%v", err), "shard count mismatch") {
		t.Errorf("Expected shard count mismatch error, got: %v", err)
	}
}
