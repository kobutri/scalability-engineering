package main

import (
	"fmt"
	"sync"
	"testing"
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
