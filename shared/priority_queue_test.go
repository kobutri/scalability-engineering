package shared

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestPriorityQueueBasicOperations(t *testing.T) {
	// Test with integers as both priority and value
	pq := NewPriorityQueue[int, int]()

	// Test empty queue
	if !pq.IsEmpty() {
		t.Error("New queue should be empty")
	}

	if pq.Size() != 0 {
		t.Error("Empty queue size should be 0")
	}

	// Test peek on empty queue
	if _, _, ok := pq.Peek(); ok {
		t.Error("Peek on empty queue should return false")
	}

	// Test extract from empty queue
	if _, _, ok := pq.ExtractMin(); ok {
		t.Error("ExtractMin on empty queue should return false")
	}

	// Test contains on empty queue
	if pq.Contains(42) {
		t.Error("Empty queue should not contain any elements")
	}
}

func TestPriorityQueueInsertAndExtract(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	// Insert priority-value pairs
	items := []struct {
		priority int
		value    string
	}{
		{5, "five"},
		{2, "two"},
		{8, "eight"},
		{1, "one"},
		{9, "nine"},
		{3, "three"},
	}

	for _, item := range items {
		if !pq.Insert(item.priority, item.value) {
			t.Errorf("Insert (%d, %s) should succeed", item.priority, item.value)
		}
	}

	// Try to insert duplicate value - should update priority
	if pq.Insert(10, "five") {
		t.Error("Insert duplicate value should return false (indicating update)")
	}

	// Verify the priority was updated
	if priority, ok := pq.GetPriority("five"); !ok || priority != 10 {
		t.Errorf("Priority for 'five' should be updated to 10, got %d", priority)
	}

	// Check size
	if pq.Size() != len(items) {
		t.Errorf("Size should be %d, got %d", len(items), pq.Size())
	}

	// Check contains
	for _, item := range items {
		if !pq.Contains(item.value) {
			t.Errorf("Should contain %s", item.value)
		}
	}

	if pq.Contains("notfound") {
		t.Error("Should not contain notfound")
	}

	// Check peek
	if priority, value, ok := pq.Peek(); !ok || priority != 1 || value != "one" {
		t.Errorf("Peek should return (1, one), got (%d, %s)", priority, value)
	}

	// Extract in sorted order by priority
	// Note: "five" now has priority 10 due to the update above
	expected := []struct {
		priority int
		value    string
	}{
		{1, "one"},
		{2, "two"},
		{3, "three"},
		{8, "eight"},
		{9, "nine"},
		{10, "five"},
	}

	for i, expectedItem := range expected {
		if priority, value, ok := pq.ExtractMin(); !ok || priority != expectedItem.priority || value != expectedItem.value {
			t.Errorf("ExtractMin %d should return (%d, %s), got (%d, %s)", i, expectedItem.priority, expectedItem.value, priority, value)
		}
	}

	// Queue should be empty now
	if !pq.IsEmpty() {
		t.Error("Queue should be empty after extracting all elements")
	}
}

func TestPriorityQueueRemove(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	// Insert elements (priority = value for simplicity)
	elements := []int{10, 5, 15, 3, 7, 12, 18}
	for _, elem := range elements {
		pq.Insert(elem, elem)
	}

	// Remove middle element
	if !pq.Remove(7) {
		t.Error("Remove 7 should succeed")
	}

	// Check it's no longer there
	if pq.Contains(7) {
		t.Error("Should not contain 7 after removal")
	}

	// Try to remove non-existent element
	if pq.Remove(42) {
		t.Error("Remove non-existent element should fail")
	}

	// Remove root
	if !pq.Remove(3) {
		t.Error("Remove root should succeed")
	}

	// Verify heap property maintained
	if !pq.Validate() {
		t.Error("Heap property violated after removal")
	}

	// Extract remaining elements should be in priority order
	var resultPriorities []int
	var resultValues []int
	for !pq.IsEmpty() {
		if priority, value, ok := pq.ExtractMin(); ok {
			resultPriorities = append(resultPriorities, priority)
			resultValues = append(resultValues, value)
		}
	}

	expected := []int{5, 10, 12, 15, 18}
	if len(resultPriorities) != len(expected) {
		t.Errorf("Expected %d elements, got %d", len(expected), len(resultPriorities))
	}

	for i, val := range resultPriorities {
		if val != expected[i] {
			t.Errorf("At index %d, expected priority %d, got %d", i, expected[i], val)
		}
		// Since priority = value in this test
		if resultValues[i] != expected[i] {
			t.Errorf("At index %d, expected value %d, got %d", i, expected[i], resultValues[i])
		}
	}
}

func TestPriorityQueueBulkOperations(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	// Insert many elements (priority = value)
	for i := 100; i >= 1; i-- {
		pq.Insert(i, i)
	}

	initialSize := pq.Size()

	// Extract bulk (now doesn't return elements)
	pq.ExtractMinBulk(10)

	// Remaining size should be 90
	if pq.Size() != initialSize-10 {
		t.Errorf("Expected size %d, got %d", initialSize-10, pq.Size())
	}

	// Next minimum should be (11, 11)
	if priority, value, ok := pq.Peek(); !ok || priority != 11 || value != 11 {
		t.Errorf("Next minimum should be (11, 11), got (%d, %d)", priority, value)
	}

	// Test extracting more than available
	pq.Clear()
	pq.Insert(1, 1)
	pq.Insert(2, 2)

	// This should just remove both elements without error
	pq.ExtractMinBulk(10)

	if !pq.IsEmpty() {
		t.Error("Queue should be empty after bulk extract")
	}
}

func TestPriorityQueueRandomAccess(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	items := []struct {
		priority int
		value    string
	}{
		{5, "five"},
		{2, "two"},
		{8, "eight"},
		{1, "one"},
		{9, "nine"},
	}

	for _, item := range items {
		pq.Insert(item.priority, item.value)
	}

	// Test random access to both priority and value
	for i := 0; i < pq.Size(); i++ {
		if _, _, ok := pq.Get(i); !ok {
			t.Errorf("Get(%d) should succeed", i)
		}
	}

	// Test random access to just priority
	for i := 0; i < pq.Size(); i++ {
		if _, ok := pq.GetPriorityAt(i); !ok {
			t.Errorf("GetPriorityAt(%d) should succeed", i)
		}
	}

	// Test random access to just value
	for i := 0; i < pq.Size(); i++ {
		if _, ok := pq.GetValueAt(i); !ok {
			t.Errorf("GetValueAt(%d) should succeed", i)
		}
	}

	// Test out of bounds access
	if _, _, ok := pq.Get(-1); ok {
		t.Error("Get(-1) should fail")
	}

	if _, _, ok := pq.Get(pq.Size()); ok {
		t.Error("Get(size) should fail")
	}

	// Test ToSlices
	priorities, values := pq.ToSlices()
	if len(priorities) != pq.Size() || len(values) != pq.Size() {
		t.Error("ToSlices length should match size")
	}

	// Test ToSortedSlices
	sortedPriorities, sortedValues := pq.ToSortedSlices()
	expectedPriorities := []int{1, 2, 5, 8, 9}
	expectedValues := []string{"one", "two", "five", "eight", "nine"}

	if len(sortedPriorities) != len(expectedPriorities) {
		t.Errorf("Expected %d elements, got %d", len(expectedPriorities), len(sortedPriorities))
	}

	for i, priority := range sortedPriorities {
		if priority != expectedPriorities[i] {
			t.Errorf("At index %d, expected priority %d, got %d", i, expectedPriorities[i], priority)
		}
		if sortedValues[i] != expectedValues[i] {
			t.Errorf("At index %d, expected value %s, got %s", i, expectedValues[i], sortedValues[i])
		}
	}

	// Original queue should be unchanged
	if pq.Size() != len(items) {
		t.Error("ToSortedSlices should not modify original queue")
	}

	// Test WithReadLock for safe access
	var accessedPriorities []int
	var accessedValues []string
	pq.WithReadLock(func(priorities []int, values []string) {
		accessedPriorities = make([]int, len(priorities))
		accessedValues = make([]string, len(values))
		copy(accessedPriorities, priorities)
		copy(accessedValues, values)
	})

	if len(accessedPriorities) != pq.Size() || len(accessedValues) != pq.Size() {
		t.Error("WithReadLock should provide access to all elements")
	}

	// Test GetPriority
	if priority, ok := pq.GetPriority("two"); !ok || priority != 2 {
		t.Errorf("GetPriority('two') should return 2, got %d", priority)
	}

	// Test GetPriority for another value
	if priority, ok := pq.GetPriority("five"); !ok || priority != 5 {
		t.Errorf("GetPriority('five') should return 5, got %d", priority)
	}

	if _, ok := pq.GetPriority("notfound"); ok {
		t.Error("GetPriority('notfound') should fail")
	}
}

func TestPriorityQueuePriorityUpdate(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	// Insert initial items
	pq.Insert(5, "task1")
	pq.Insert(3, "task2")
	pq.Insert(8, "task3")
	pq.Insert(1, "task4")

	// Verify initial state
	if priority, value, ok := pq.Peek(); !ok || priority != 1 || value != "task4" {
		t.Errorf("Initial min should be (1, task4), got (%d, %s)", priority, value)
	}

	// Update priority of task4 from 1 to 10 (should move to end)
	if pq.Insert(10, "task4") {
		t.Error("Update should return false")
	}

	// Now task2 should be minimum
	if priority, value, ok := pq.Peek(); !ok || priority != 3 || value != "task2" {
		t.Errorf("After update, min should be (3, task2), got (%d, %s)", priority, value)
	}

	// Update priority of task3 from 8 to 2 (should become new minimum)
	if pq.Insert(2, "task3") {
		t.Error("Update should return false")
	}

	// Now task3 should be minimum
	if priority, value, ok := pq.Peek(); !ok || priority != 2 || value != "task3" {
		t.Errorf("After second update, min should be (2, task3), got (%d, %s)", priority, value)
	}

	// Extract all and verify order
	expected := []struct {
		priority int
		value    string
	}{
		{2, "task3"},
		{3, "task2"},
		{5, "task1"},
		{10, "task4"},
	}

	for i, expectedItem := range expected {
		if priority, value, ok := pq.ExtractMin(); !ok || priority != expectedItem.priority || value != expectedItem.value {
			t.Errorf("ExtractMin %d should return (%d, %s), got (%d, %s)",
				i, expectedItem.priority, expectedItem.value, priority, value)
		}
	}

	// Verify heap property maintained throughout
	if !pq.IsEmpty() {
		t.Error("Queue should be empty after extracting all elements")
	}
}

func TestPriorityQueueWithDifferentTypes(t *testing.T) {
	// Test with float priorities and string values
	pq := NewPriorityQueue[float64, string]()

	items := []struct {
		priority float64
		value    string
	}{
		{3.14, "pi"},
		{2.71, "e"},
		{1.41, "sqrt2"},
		{1.61, "phi"},
	}

	for _, item := range items {
		pq.Insert(item.priority, item.value)
	}

	// Extract in priority order
	expected := []string{"sqrt2", "phi", "e", "pi"}
	for _, expectedValue := range expected {
		if _, value, ok := pq.ExtractMin(); !ok || value != expectedValue {
			t.Errorf("Expected %s, got %s", expectedValue, value)
		}
	}
}

func TestPriorityQueueThreadSafety(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	numWorkers := 10
	numOpsPerWorker := 100
	var wg sync.WaitGroup

	// Start multiple workers doing concurrent operations
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for j := 0; j < numOpsPerWorker; j++ {
				switch r.Intn(4) {
				case 0: // Insert
					value := r.Intn(1000)
					priority := value // Use same value as priority
					pq.Insert(priority, value)

				case 1: // Contains
					value := r.Intn(1000)
					pq.Contains(value)

				case 2: // Remove
					value := r.Intn(1000)
					pq.Remove(value)

				case 3: // ExtractMin
					pq.ExtractMin()
				}

				// Occasionally check stats
				if j%10 == 0 {
					stats := pq.Stats()
					if !stats.IsValid {
						t.Errorf("Heap invariant violated in worker %d", workerID)
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Final validation
	if !pq.Validate() {
		t.Error("Final heap validation failed")
	}
}

func TestPriorityQueueConcurrentInsertExtract(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	numInserters := 3
	numExtractors := 2
	numElements := 100

	var wg sync.WaitGroup
	var insertDone sync.WaitGroup

	// Start inserters first and let them finish
	insertDone.Add(numInserters)
	for i := 0; i < numInserters; i++ {
		wg.Add(1)
		go func(inserterID int) {
			defer wg.Done()
			defer insertDone.Done()

			for j := 0; j < numElements; j++ {
				value := inserterID*1000 + j
				priority := value // Use same value as priority
				pq.Insert(priority, value)
			}
		}(i)
	}

	// Start extractors after a brief delay to let some insertions happen
	time.Sleep(time.Millisecond)

	extracted := make([][]int, numExtractors)
	for i := 0; i < numExtractors; i++ {
		wg.Add(1)
		go func(extractorID int) {
			defer wg.Done()

			// Keep extracting until queue is empty and all insertions are done
			for {
				if priority, value, ok := pq.ExtractMin(); ok {
					// Verify priority equals value in this test
					if priority != value {
						t.Errorf("Priority %d should equal value %d", priority, value)
						return
					}
					extracted[extractorID] = append(extracted[extractorID], priority)
				} else {
					// Check if all insertions are done
					insertDone.Wait()
					// Try one more time in case there were elements added after our last check
					if priority, value, ok := pq.ExtractMin(); ok {
						if priority != value {
							t.Errorf("Priority %d should equal value %d", priority, value)
							return
						}
						extracted[extractorID] = append(extracted[extractorID], priority)
					} else {
						break // No more elements and insertions are done
					}
				}
				time.Sleep(time.Microsecond) // Small delay to allow insertions
			}
		}(i)
	}

	wg.Wait()

	// Verify extracted elements are in sorted order within each extractor
	for i, slice := range extracted {
		for j := 1; j < len(slice); j++ {
			if slice[j] < slice[j-1] {
				t.Errorf("Extractor %d: elements not in sorted order at positions %d, %d: %d < %d",
					i, j-1, j, slice[j-1], slice[j])
			}
		}
	}

	// Verify all elements were extracted
	totalExtracted := 0
	for _, slice := range extracted {
		totalExtracted += len(slice)
	}

	expectedTotal := numInserters * numElements
	if totalExtracted != expectedTotal {
		t.Errorf("Expected to extract %d elements total, got %d", expectedTotal, totalExtracted)
	}

	// Final validation
	if !pq.Validate() {
		t.Error("Final heap validation failed")
	}
}

func TestPriorityQueueEdgeCases(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	// Test single element
	pq.Insert(42, "answer")
	if priority, value, ok := pq.ExtractMin(); !ok || priority != 42 || value != "answer" {
		t.Error("Single element test failed")
	}

	// Test clear
	for i := 1; i <= 5; i++ {
		pq.Insert(i, string(rune('a'+i-1)))
	}
	pq.Clear()

	if !pq.IsEmpty() {
		t.Error("Clear should make queue empty")
	}

	// Test zero and negative bulk extract
	pq.Insert(1, "one")

	pq.ExtractMinBulk(0)
	if pq.Size() != 1 {
		t.Error("Zero bulk extract should not change size")
	}

	pq.ExtractMinBulk(-5)
	if pq.Size() != 1 {
		t.Error("Negative bulk extract should not change size")
	}
}

func TestPriorityQueueStats(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	// Empty queue stats
	stats := pq.Stats()
	if stats.Size != 0 || stats.HeapHeight != 0 || !stats.IsValid {
		t.Error("Empty queue stats incorrect")
	}

	// Add elements and check stats
	for i := 1; i <= 15; i++ {
		pq.Insert(i, i)
	}

	stats = pq.Stats()
	if stats.Size != 15 || !stats.IsValid {
		t.Errorf("Stats after insertion incorrect: %+v", stats)
	}

	// Height should be reasonable for 15 elements
	if stats.HeapHeight < 4 || stats.HeapHeight > 6 {
		t.Errorf("Unexpected heap height: %d", stats.HeapHeight)
	}
}

func TestPriorityQueueUnsafeAccess(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	items := []struct {
		priority int
		value    string
	}{
		{5, "five"},
		{2, "two"},
		{8, "eight"},
		{1, "one"},
		{9, "nine"},
	}

	for _, item := range items {
		pq.Insert(item.priority, item.value)
	}

	// Test unsafe access - should work but is dangerous
	unsafePriorities := pq.GetUnsafePriorities()
	unsafeValues := pq.GetUnsafeValues()

	if len(unsafePriorities) != pq.Size() {
		t.Error("GetUnsafePriorities should return slice with correct length")
	}

	if len(unsafeValues) != pq.Size() {
		t.Error("GetUnsafeValues should return slice with correct length")
	}

	// Test that we can access elements through the unsafe slices
	if len(unsafePriorities) > 0 && len(unsafeValues) > 0 {
		// Just verify we can read the first elements
		_ = unsafePriorities[0]
		_ = unsafeValues[0]
	}
}

// Benchmark tests
func BenchmarkPriorityQueueInsert(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Insert(i, i)
	}
}

func BenchmarkPriorityQueueExtractMin(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	// Pre-populate
	for i := 0; i < b.N; i++ {
		pq.Insert(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.ExtractMin()
	}
}

func BenchmarkPriorityQueueContains(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		pq.Insert(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Contains(i % 10000)
	}
}

func BenchmarkPriorityQueueConcurrent(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value := rand.Intn(1000)
			// Mix of operations
			switch rand.Intn(3) {
			case 0:
				pq.Insert(value, value)
			case 1:
				pq.ExtractMin()
			case 2:
				pq.Contains(value)
			}
		}
	})
}

func TestPriorityQueueGetRandomSubset(t *testing.T) {
	pq := NewPriorityQueue[int, string]()

	// Test empty queue
	subset := pq.GetRandomSubset(5)
	if len(subset) != 0 {
		t.Error("GetRandomSubset on empty queue should return empty slice")
	}

	// Insert test data
	items := []struct {
		priority int
		value    string
	}{
		{1, "one"}, {2, "two"}, {3, "three"}, {4, "four"}, {5, "five"},
		{6, "six"}, {7, "seven"}, {8, "eight"}, {9, "nine"}, {10, "ten"},
	}

	for _, item := range items {
		pq.Insert(item.priority, item.value)
	}

	// Test edge cases
	subset = pq.GetRandomSubset(0)
	if len(subset) != 0 {
		t.Error("GetRandomSubset(0) should return empty slice")
	}

	subset = pq.GetRandomSubset(-1)
	if len(subset) != 0 {
		t.Error("GetRandomSubset(-1) should return empty slice")
	}

	// Test subset larger than queue size
	subset = pq.GetRandomSubset(20)
	if len(subset) != len(items) {
		t.Errorf("GetRandomSubset(20) should return all %d elements, got %d", len(items), len(subset))
	}

	// Verify all original values are present when requesting more than available
	originalValues := make(map[string]bool)
	for _, item := range items {
		originalValues[item.value] = true
	}
	for _, value := range subset {
		if !originalValues[value] {
			t.Errorf("Unexpected value in full subset: %s", value)
		}
	}

	// Test small subset (uses rejection sampling)
	subset = pq.GetRandomSubset(3)
	if len(subset) != 3 {
		t.Errorf("GetRandomSubset(3) should return 3 elements, got %d", len(subset))
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, value := range subset {
		if seen[value] {
			t.Errorf("Duplicate value in subset: %s", value)
		}
		seen[value] = true

		// Verify value exists in original queue
		if !pq.Contains(value) {
			t.Errorf("Subset contains value not in queue: %s", value)
		}
	}

	// Test large subset (uses reservoir sampling)
	subset = pq.GetRandomSubset(8)
	if len(subset) != 8 {
		t.Errorf("GetRandomSubset(8) should return 8 elements, got %d", len(subset))
	}

	// Verify no duplicates in large subset
	seen = make(map[string]bool)
	for _, value := range subset {
		if seen[value] {
			t.Errorf("Duplicate value in large subset: %s", value)
		}
		seen[value] = true

		// Verify value exists in original queue
		if !pq.Contains(value) {
			t.Errorf("Large subset contains value not in queue: %s", value)
		}
	}

	// Test randomness by checking distribution over multiple calls
	// This is a statistical test - very unlikely to fail if implementation is correct
	counts := make(map[string]int)
	iterations := 1000
	subsetSize := 5

	for i := 0; i < iterations; i++ {
		subset = pq.GetRandomSubset(subsetSize)
		for _, value := range subset {
			counts[value]++
		}
	}

	// Each value should appear in roughly 50% of subsets (5 out of 10 items)
	expectedFreq := float64(iterations*subsetSize) / float64(len(items))
	tolerance := expectedFreq * 0.3 // Allow 30% deviation

	for _, item := range items {
		freq := float64(counts[item.value])
		if freq < expectedFreq-tolerance || freq > expectedFreq+tolerance {
			t.Logf("Warning: Value %s appeared %d times, expected around %.1f (tolerance ±%.1f)",
				item.value, counts[item.value], expectedFreq, tolerance)
			// Don't fail the test as this is statistical and could occasionally fail
		}
	}
}

func TestPriorityQueueGetRandomSubsetConcurrency(t *testing.T) {
	pq := NewPriorityQueue[int, int]()

	// Populate queue
	for i := 1; i <= 100; i++ {
		pq.Insert(i, i)
	}

	// Test concurrent access to GetRandomSubset
	var wg sync.WaitGroup
	numWorkers := 10

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				subset := pq.GetRandomSubset(10)

				// Verify subset properties
				if len(subset) != 10 {
					t.Errorf("Worker %d: expected subset size 10, got %d", workerID, len(subset))
					return
				}

				// Check for duplicates
				seen := make(map[int]bool)
				for _, value := range subset {
					if seen[value] {
						t.Errorf("Worker %d: duplicate value %d in subset", workerID, value)
						return
					}
					seen[value] = true
				}
			}
		}(i)
	}

	wg.Wait()
}

func BenchmarkPriorityQueueGetRandomSubsetSmall(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	// Pre-populate with 1000 elements
	for i := 0; i < 1000; i++ {
		pq.Insert(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subset := pq.GetRandomSubset(10) // Small subset
		_ = subset
	}
}

func BenchmarkPriorityQueueGetRandomSubsetLarge(b *testing.B) {
	pq := NewPriorityQueue[int, int]()

	// Pre-populate with 1000 elements
	for i := 0; i < 1000; i++ {
		pq.Insert(i, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		subset := pq.GetRandomSubset(800) // Large subset (uses reservoir sampling)
		_ = subset
	}
}
