package main

import (
	"math/rand"
	"sync"
	"time"
)

// Ordered constraint for types that can be compared
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 | ~string
}

// PriorityQueue represents a thread-safe hybrid data structure combining
// a min-heap with a hash map for optimal performance:
// - Insert: O(log n)
// - Bulk removal of m lowest: O(m log n)
// - Membership lookup: O(1)
// - Arbitrary removal: O(log n)
// - Random access: O(1)
type PriorityQueue[P Ordered, V comparable] struct {
	mu         sync.RWMutex
	priorities []P       // min-heap array ordered by priority
	values     []V       // corresponding values at same indices (not heap-ordered)
	indexMap   map[V]int // value -> heap index mapping
}

// NewPriorityQueue creates a new thread-safe priority queue
func NewPriorityQueue[P Ordered, V comparable]() *PriorityQueue[P, V] {
	return &PriorityQueue[P, V]{
		priorities: make([]P, 0),
		values:     make([]V, 0),
		indexMap:   make(map[V]int),
	}
}

// Insert adds a priority-value pair to the priority queue
// If value already exists, updates its priority
// Returns true if value was added, false if priority was updated
// Time complexity: O(log n)
func (pq *PriorityQueue[P, V]) Insert(priority P, value V) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Check if value already exists
	if index, exists := pq.indexMap[value]; exists {
		// Update existing priority
		oldPriority := pq.priorities[index]
		pq.priorities[index] = priority

		// Restore heap property
		if priority < oldPriority {
			// Priority decreased, may need to bubble up
			pq.heapifyUp(index)
		} else if priority > oldPriority {
			// Priority increased, may need to bubble down
			pq.heapifyDown(index)
		}
		// If priority is the same, no heap operations needed

		return false // Indicates priority was updated, not inserted
	}

	// Add to end of both slices
	index := len(pq.priorities)
	pq.priorities = append(pq.priorities, priority)
	pq.values = append(pq.values, value)
	pq.indexMap[value] = index

	// Restore heap property by bubbling up
	pq.heapifyUp(index)

	return true // Indicates new value was inserted
}

// ExtractMin removes and returns the minimum priority and its corresponding value
// Returns priority, value and true if successful, zero values and false if empty
// Time complexity: O(log n)
func (pq *PriorityQueue[P, V]) ExtractMin() (P, V, bool) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	var zeroP P
	var zeroV V
	if len(pq.priorities) == 0 {
		return zeroP, zeroV, false
	}

	// Get minimum priority and corresponding value (root)
	minPriority := pq.priorities[0]
	minValue := pq.values[0]
	delete(pq.indexMap, minValue)

	// Move last elements to root
	lastIndex := len(pq.priorities) - 1
	if lastIndex == 0 {
		// Only one element
		pq.priorities = pq.priorities[:0]
		pq.values = pq.values[:0]
		return minPriority, minValue, true
	}

	lastPriority := pq.priorities[lastIndex]
	lastValue := pq.values[lastIndex]
	pq.priorities[0] = lastPriority
	pq.values[0] = lastValue
	pq.indexMap[lastValue] = 0
	pq.priorities = pq.priorities[:lastIndex]
	pq.values = pq.values[:lastIndex]

	// Restore heap property by bubbling down
	pq.heapifyDown(0)

	return minPriority, minValue, true
}

// ExtractMinBulk removes the m smallest elements without returning them (avoids allocation)
// Time complexity: O(m log n)
func (pq *PriorityQueue[P, V]) ExtractMinBulk(m int) {
	if m <= 0 {
		return
	}

	for i := 0; i < m; i++ {
		if _, _, ok := pq.ExtractMin(); !ok {
			break // No more elements
		}
	}
}

// Contains checks if a value exists in the priority queue
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) Contains(value V) bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	_, exists := pq.indexMap[value]
	return exists
}

// Remove removes a value from the priority queue
// Returns true if value was removed, false if not found
// Time complexity: O(log n)
func (pq *PriorityQueue[P, V]) Remove(value V) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	index, exists := pq.indexMap[value]
	if !exists {
		return false
	}

	delete(pq.indexMap, value)
	lastIndex := len(pq.priorities) - 1

	if index == lastIndex {
		// Removing last element
		pq.priorities = pq.priorities[:lastIndex]
		pq.values = pq.values[:lastIndex]
		return true
	}

	// Move last elements to removed position
	lastPriority := pq.priorities[lastIndex]
	lastValue := pq.values[lastIndex]
	pq.priorities[index] = lastPriority
	pq.values[index] = lastValue
	pq.indexMap[lastValue] = index
	pq.priorities = pq.priorities[:lastIndex]
	pq.values = pq.values[:lastIndex]

	// Restore heap property - try both directions
	parent := (index - 1) / 2
	if index > 0 && pq.priorities[index] < pq.priorities[parent] {
		pq.heapifyUp(index)
	} else {
		pq.heapifyDown(index)
	}

	return true
}

// Peek returns the minimum priority and its corresponding value without removing them
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) Peek() (P, V, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var zeroP P
	var zeroV V
	if len(pq.priorities) == 0 {
		return zeroP, zeroV, false
	}

	return pq.priorities[0], pq.values[0], true
}

// GetPriority returns the priority of a given value
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) GetPriority(value V) (P, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var zeroP P
	index, exists := pq.indexMap[value]
	if !exists {
		return zeroP, false
	}

	return pq.priorities[index], true
}

// Get provides random access to both priority and value at the given index
// Note: This returns elements in heap order, not sorted order
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) Get(index int) (P, V, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var zeroP P
	var zeroV V
	if index < 0 || index >= len(pq.priorities) {
		return zeroP, zeroV, false
	}

	return pq.priorities[index], pq.values[index], true
}

// GetPriorityAt provides random access to priority at the given index
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) GetPriorityAt(index int) (P, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var zeroP P
	if index < 0 || index >= len(pq.priorities) {
		return zeroP, false
	}

	return pq.priorities[index], true
}

// GetValueAt provides random access to value at the given index
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) GetValueAt(index int) (V, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	var zeroV V
	if index < 0 || index >= len(pq.values) {
		return zeroV, false
	}

	return pq.values[index], true
}

// GetUnsafePriorities provides direct access to the underlying priorities slice for random access
// WARNING: This returns a reference to the internal data. Do not modify!
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) GetUnsafePriorities() []P {
	return pq.priorities
}

// GetUnsafeValues provides direct access to the underlying values slice for random access
// WARNING: This returns a reference to the internal data. Do not modify!
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) GetUnsafeValues() []V {
	return pq.values
}

// WithReadLock executes a function with read lock held, providing safe access to underlying data
// Time complexity: O(1) + O(f)
func (pq *PriorityQueue[P, V]) WithReadLock(f func(priorities []P, values []V)) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	f(pq.priorities, pq.values)
}

// GetRandomSubset returns a random subset of n values from the priority queue
// Uses only a single allocation for the result slice
// Time complexity: O(n) average case, O(n²) worst case for small n relative to size
func (pq *PriorityQueue[P, V]) GetRandomSubset(n int) []V {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	size := len(pq.values)

	// Handle edge cases
	if n <= 0 || size == 0 {
		return []V{}
	}
	if n >= size {
		// Return copy of all values
		result := make([]V, size)
		copy(result, pq.values)
		return result
	}

	// Single allocation for result - this is the ONLY allocation
	result := make([]V, n)

	// Use rejection sampling with efficient duplicate tracking
	// For better performance when n is large relative to size, we track selected indices
	// but we'll pack this info into the result slice itself to avoid extra allocations

	if n <= size/2 {
		// For smaller subsets: simple rejection sampling with linear duplicate check
		for i := 0; i < n; i++ {
			for {
				randIndex := pq.fastRand() % size

				// Check if this value was already selected
				duplicate := false
				for j := 0; j < i; j++ {
					if pq.values[randIndex] == result[j] {
						duplicate = true
						break
					}
				}

				if !duplicate {
					result[i] = pq.values[randIndex]
					break
				}
			}
		}
	} else {
		// For larger subsets: use reservoir sampling variant
		// This is more efficient when n is close to size
		copy(result, pq.values[:n]) // Start with first n elements

		// Replace elements with decreasing probability
		for i := n; i < size; i++ {
			// Random index from 0 to i (inclusive)
			j := pq.fastRand() % (i + 1)
			if j < n {
				// Replace element at position j with element at position i
				result[j] = pq.values[i]
			}
		}
	}

	return result
}

// fastRand provides a fast random number using the current time and queue state
// This avoids the overhead of global rand state for performance-critical path
func (pq *PriorityQueue[P, V]) fastRand() int {
	// Use current time and queue size for randomness
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(len(pq.values))))
	return r.Int()
}

// Size returns the number of elements in the priority queue
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	return len(pq.priorities)
}

// IsEmpty checks if the priority queue is empty
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) IsEmpty() bool {
	return pq.Size() == 0
}

// Clear removes all elements from the priority queue
// Time complexity: O(1)
func (pq *PriorityQueue[P, V]) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.priorities = pq.priorities[:0]
	pq.values = pq.values[:0]
	pq.indexMap = make(map[V]int)
}

// ToSlices returns all priorities and values as separate slices in heap order (not sorted)
// Time complexity: O(n)
func (pq *PriorityQueue[P, V]) ToSlices() ([]P, []V) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	priorities := make([]P, len(pq.priorities))
	values := make([]V, len(pq.values))
	copy(priorities, pq.priorities)
	copy(values, pq.values)
	return priorities, values
}

// ToSortedSlices returns all elements in sorted order by priority (smallest to largest)
// Time complexity: O(n log n)
func (pq *PriorityQueue[P, V]) ToSortedSlices() ([]P, []V) {
	// Create a copy to avoid modifying original
	tempPQ := NewPriorityQueue[P, V]()

	pq.mu.RLock()
	tempPQ.priorities = make([]P, len(pq.priorities))
	tempPQ.values = make([]V, len(pq.values))
	copy(tempPQ.priorities, pq.priorities)
	copy(tempPQ.values, pq.values)
	tempPQ.indexMap = make(map[V]int)
	for value, idx := range pq.indexMap {
		tempPQ.indexMap[value] = idx
	}
	pq.mu.RUnlock()

	priorities := make([]P, 0, len(tempPQ.priorities))
	values := make([]V, 0, len(tempPQ.values))
	for !tempPQ.IsEmpty() {
		if priority, value, ok := tempPQ.ExtractMin(); ok {
			priorities = append(priorities, priority)
			values = append(values, value)
		}
	}

	return priorities, values
}

// =============================================================================
// INTERNAL HEAP OPERATIONS
// =============================================================================

// heapifyUp restores heap property by moving element up the tree
func (pq *PriorityQueue[P, V]) heapifyUp(index int) {
	for index > 0 {
		parentIndex := (index - 1) / 2

		// If heap property is satisfied, stop
		if pq.priorities[index] >= pq.priorities[parentIndex] {
			break
		}

		// Swap with parent and update indices
		pq.swap(index, parentIndex)
		index = parentIndex
	}
}

// heapifyDown restores heap property by moving element down the tree
func (pq *PriorityQueue[P, V]) heapifyDown(index int) {
	for {
		leftChild := 2*index + 1
		rightChild := 2*index + 2
		smallest := index

		// Find smallest among node and its children
		if leftChild < len(pq.priorities) && pq.priorities[leftChild] < pq.priorities[smallest] {
			smallest = leftChild
		}

		if rightChild < len(pq.priorities) && pq.priorities[rightChild] < pq.priorities[smallest] {
			smallest = rightChild
		}

		// If heap property is satisfied, stop
		if smallest == index {
			break
		}

		// Swap with smallest child and continue
		pq.swap(index, smallest)
		index = smallest
	}
}

// swap exchanges two elements in both slices and updates their indices
func (pq *PriorityQueue[P, V]) swap(i, j int) {
	// Swap elements in both slices
	pq.priorities[i], pq.priorities[j] = pq.priorities[j], pq.priorities[i]
	pq.values[i], pq.values[j] = pq.values[j], pq.values[i]

	// Update index mappings
	pq.indexMap[pq.values[i]] = i
	pq.indexMap[pq.values[j]] = j
}

// =============================================================================
// VALIDATION AND DEBUGGING
// =============================================================================

// Validate checks if the data structure maintains its invariants
// Returns true if valid, false otherwise
func (pq *PriorityQueue[P, V]) Validate() bool {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	return pq.validateUnsafe()
}

// validateUnsafe checks invariants without acquiring locks (assumes caller holds appropriate lock)
func (pq *PriorityQueue[P, V]) validateUnsafe() bool {
	// Check if slices have same length
	if len(pq.priorities) != len(pq.values) {
		return false
	}

	// Check if index map size matches slice lengths
	if len(pq.indexMap) != len(pq.priorities) {
		return false
	}

	// Check if all values are in index map with correct indices
	for i, value := range pq.values {
		if index, exists := pq.indexMap[value]; !exists || index != i {
			return false
		}
	}

	// Check heap property
	for i := 0; i < len(pq.priorities); i++ {
		leftChild := 2*i + 1
		rightChild := 2*i + 2

		if leftChild < len(pq.priorities) && pq.priorities[leftChild] < pq.priorities[i] {
			return false
		}

		if rightChild < len(pq.priorities) && pq.priorities[rightChild] < pq.priorities[i] {
			return false
		}
	}

	return true
}

// Stats returns statistics about the priority queue
type PriorityQueueStats struct {
	Size       int
	HeapHeight int
	IsValid    bool
}

func (pq *PriorityQueue[P, V]) Stats() PriorityQueueStats {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	height := 0
	if len(pq.priorities) > 0 {
		// Height of complete binary tree
		n := len(pq.priorities)
		height = 1
		for (1 << height) <= n {
			height++
		}
	}

	return PriorityQueueStats{
		Size:       len(pq.priorities),
		HeapHeight: height,
		IsValid:    pq.validateUnsafe(),
	}
}
