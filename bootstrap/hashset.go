package main

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Hasher interface for custom hash functions
type Hasher[T any] interface {
	Hash(T) uint32
}

// Default hasher using FNV-1a
type defaultHasher[T any] struct{}

func (d defaultHasher[T]) Hash(key T) uint32 {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%v", key)))
	return h.Sum32()
}

// HashSet is the main thread-safe set structure
type HashSet[T comparable] struct {
	shards     []shard[T]
	shardCount uint32
	hasher     Hasher[T]
	size       int64 // atomic counter for total size
}

// shard represents a single shard with its own lock
type shard[T comparable] struct {
	mu       sync.RWMutex
	indexMap map[T]int
	elements []T
}

const (
	defaultShardCount      = 32
	defaultInitialCapacity = 16
)

// New creates a new HashSet with default settings
func NewHashSet[T comparable]() *HashSet[T] {
	return NewHashSetWithShards[T](defaultShardCount)
}

// NewHashSetWithShards creates a HashSet with specified shard count
func NewHashSetWithShards[T comparable](shardCount int) *HashSet[T] {
	// Ensure shard count is power of 2
	sc := uint32(1)
	for sc < uint32(shardCount) {
		sc <<= 1
	}

	h := &HashSet[T]{
		shards:     make([]shard[T], sc),
		shardCount: sc,
		hasher:     defaultHasher[T]{},
	}

	// Initialize each shard
	for i := range h.shards {
		h.shards[i].indexMap = make(map[T]int)
		h.shards[i].elements = make([]T, 0, defaultInitialCapacity)
	}

	return h
}

// NewHashSetWithHasher creates a HashSet with custom hasher
func NewHashSetWithHasher[T comparable](shardCount int, hasher Hasher[T]) *HashSet[T] {
	h := NewHashSetWithShards[T](shardCount)
	h.hasher = hasher
	return h
}

// getShard returns the shard for a given element
func (h *HashSet[T]) getShard(element T) *shard[T] {
	hash := h.hasher.Hash(element)
	// Use bitwise AND for fast modulo when shardCount is power of 2
	index := hash & (h.shardCount - 1)
	return &h.shards[index]
}

// getShardByIndex returns shard by direct index
func (h *HashSet[T]) getShardByIndex(index uint32) *shard[T] {
	return &h.shards[index]
}

// Insert adds an element to the set, returns true if element was added
func (h *HashSet[T]) Insert(element T) bool {
	shard := h.getShard(element)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check if element already exists
	if _, exists := shard.indexMap[element]; exists {
		return false
	}

	// Add element to slice and update index map
	index := len(shard.elements)
	shard.elements = append(shard.elements, element)
	shard.indexMap[element] = index

	// Update global size counter
	atomic.AddInt64(&h.size, 1)

	return true
}

// InsertAll adds multiple elements
func (h *HashSet[T]) InsertAll(elements ...T) int {
	added := 0
	for _, elem := range elements {
		if h.Insert(elem) {
			added++
		}
	}
	return added
}

// Remove deletes an element from the set, returns true if element was removed
func (h *HashSet[T]) Remove(element T) bool {
	shard := h.getShard(element)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	index, exists := shard.indexMap[element]
	if !exists {
		return false
	}

	// Get the last element
	lastIndex := len(shard.elements) - 1

	// If removing last element, just truncate
	if index == lastIndex {
		shard.elements = shard.elements[:lastIndex]
	} else {
		// Swap with last element
		lastElement := shard.elements[lastIndex]
		shard.elements[index] = lastElement
		shard.indexMap[lastElement] = index

		// Truncate slice
		shard.elements = shard.elements[:lastIndex]
	}

	// Remove from index map
	delete(shard.indexMap, element)

	// Update global size counter
	atomic.AddInt64(&h.size, -1)

	return true
}

// Contains checks if element exists in the set
func (h *HashSet[T]) Contains(element T) bool {
	shard := h.getShard(element)

	shard.mu.RLock()
	defer shard.mu.RUnlock()

	_, exists := shard.indexMap[element]
	return exists
}

// ContainsAll checks if all elements exist in the set
func (h *HashSet[T]) ContainsAll(elements ...T) bool {
	for _, elem := range elements {
		if !h.Contains(elem) {
			return false
		}
	}
	return true
}

// RandomElement returns a random element from the set
func (h *HashSet[T]) RandomElement() (T, bool) {
	var zero T
	totalSize := atomic.LoadInt64(&h.size)
	if totalSize == 0 {
		return zero, false
	}

	// Use thread-local random to avoid contention
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	targetIndex := r.Int63n(totalSize)

	// Find which shard contains the target index
	currentTotal := int64(0)
	for i := uint32(0); i < h.shardCount; i++ {
		shard := h.getShardByIndex(i)

		shard.mu.RLock()
		shardSize := int64(len(shard.elements))

		if currentTotal+shardSize > targetIndex {
			// Element is in this shard
			localIndex := targetIndex - currentTotal
			element := shard.elements[localIndex]
			shard.mu.RUnlock()
			return element, true
		}

		shard.mu.RUnlock()
		currentTotal += shardSize
	}

	return zero, false
}

// RandomElements returns n random elements (with possible duplicates)
func (h *HashSet[T]) RandomElements(n int) []T {
	results := make([]T, 0, n)
	for i := 0; i < n; i++ {
		if elem, ok := h.RandomElement(); ok {
			results = append(results, elem)
		}
	}
	return results
}

// Size returns the total number of elements
func (h *HashSet[T]) Size() int {
	return int(atomic.LoadInt64(&h.size))
}

// IsEmpty checks if the set is empty
func (h *HashSet[T]) IsEmpty() bool {
	return h.Size() == 0
}

// Clear removes all elements from the set
func (h *HashSet[T]) Clear() {
	// Lock all shards to ensure consistency
	for i := range h.shards {
		h.shards[i].mu.Lock()
		defer h.shards[i].mu.Unlock()
	}

	// Clear each shard
	for i := range h.shards {
		h.shards[i].indexMap = make(map[T]int)
		h.shards[i].elements = h.shards[i].elements[:0]
	}

	atomic.StoreInt64(&h.size, 0)
}

// ToSlice returns all elements as a slice
func (h *HashSet[T]) ToSlice() []T {
	totalSize := atomic.LoadInt64(&h.size)
	result := make([]T, 0, totalSize)

	for i := range h.shards {
		shard := &h.shards[i]
		shard.mu.RLock()
		result = append(result, shard.elements...)
		shard.mu.RUnlock()
	}

	return result
}

// Union creates a new set containing elements from both sets
func (h *HashSet[T]) Union(other *HashSet[T]) *HashSet[T] {
	result := NewHashSetWithShards[T](int(h.shardCount))

	// Add all elements from current set
	for _, elem := range h.ToSlice() {
		result.Insert(elem)
	}

	// Add all elements from other set
	for _, elem := range other.ToSlice() {
		result.Insert(elem)
	}

	return result
}

// Intersection creates a new set with common elements
func (h *HashSet[T]) Intersection(other *HashSet[T]) *HashSet[T] {
	result := NewHashSetWithShards[T](int(h.shardCount))

	// Iterate through smaller set for efficiency
	var smaller, larger *HashSet[T]
	if h.Size() <= other.Size() {
		smaller, larger = h, other
	} else {
		smaller, larger = other, h
	}

	for _, elem := range smaller.ToSlice() {
		if larger.Contains(elem) {
			result.Insert(elem)
		}
	}

	return result
}

// Difference returns elements in h but not in other
func (h *HashSet[T]) Difference(other *HashSet[T]) *HashSet[T] {
	result := NewHashSetWithShards[T](int(h.shardCount))

	for _, elem := range h.ToSlice() {
		if !other.Contains(elem) {
			result.Insert(elem)
		}
	}

	return result
}

// Iterator provides safe iteration over set elements
type Iterator[T comparable] struct {
	set      *HashSet[T]
	elements []T
	index    int
}

// NewIterator creates a snapshot-based iterator
func (h *HashSet[T]) NewIterator() *Iterator[T] {
	return &Iterator[T]{
		set:      h,
		elements: h.ToSlice(),
		index:    0,
	}
}

// HasNext checks if more elements exist
func (it *Iterator[T]) HasNext() bool {
	return it.index < len(it.elements)
}

// Next returns the next element
func (it *Iterator[T]) Next() (T, bool) {
	var zero T
	if !it.HasNext() {
		return zero, false
	}
	elem := it.elements[it.index]
	it.index++
	return elem, true
}

// Stats returns statistics about the set distribution
type Stats struct {
	TotalElements    int
	ShardCount       int
	ElementsPerShard []int
	LoadFactor       float64
}

func (h *HashSet[T]) Stats() Stats {
	stats := Stats{
		TotalElements:    h.Size(),
		ShardCount:       int(h.shardCount),
		ElementsPerShard: make([]int, h.shardCount),
	}

	for i := range h.shards {
		h.shards[i].mu.RLock()
		stats.ElementsPerShard[i] = len(h.shards[i].elements)
		h.shards[i].mu.RUnlock()
	}

	if stats.ShardCount > 0 {
		stats.LoadFactor = float64(stats.TotalElements) / float64(stats.ShardCount)
	}

	return stats
}

// GlobalLockHashSet - Alternative implementation using a single global lock
// This is for comparison purposes to demonstrate the benefits of sharding
type GlobalLockHashSet[T comparable] struct {
	mu       sync.RWMutex
	indexMap map[T]int
	elements []T
	hasher   Hasher[T]
}

// NewGlobalLockHashSet creates a new HashSet with a single global lock
func NewGlobalLockHashSet[T comparable]() *GlobalLockHashSet[T] {
	return &GlobalLockHashSet[T]{
		indexMap: make(map[T]int),
		elements: make([]T, 0, defaultInitialCapacity),
		hasher:   defaultHasher[T]{},
	}
}

// Insert adds an element to the set, returns true if element was added
func (h *GlobalLockHashSet[T]) Insert(element T) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if element already exists
	if _, exists := h.indexMap[element]; exists {
		return false
	}

	// Add element to slice and update index map
	index := len(h.elements)
	h.elements = append(h.elements, element)
	h.indexMap[element] = index

	return true
}

// Remove deletes an element from the set, returns true if element was removed
func (h *GlobalLockHashSet[T]) Remove(element T) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	index, exists := h.indexMap[element]
	if !exists {
		return false
	}

	// Get the last element
	lastIndex := len(h.elements) - 1

	// If removing last element, just truncate
	if index == lastIndex {
		h.elements = h.elements[:lastIndex]
	} else {
		// Swap with last element
		lastElement := h.elements[lastIndex]
		h.elements[index] = lastElement
		h.indexMap[lastElement] = index

		// Truncate slice
		h.elements = h.elements[:lastIndex]
	}

	// Remove from index map
	delete(h.indexMap, element)

	return true
}

// Contains checks if element exists in the set
func (h *GlobalLockHashSet[T]) Contains(element T) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, exists := h.indexMap[element]
	return exists
}

// RandomElement returns a random element from the set
func (h *GlobalLockHashSet[T]) RandomElement() (T, bool) {
	var zero T
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.elements) == 0 {
		return zero, false
	}

	// Use thread-local random to avoid contention
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(h.elements))
	return h.elements[index], true
}

// Size returns the total number of elements
func (h *GlobalLockHashSet[T]) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.elements)
}

// Clear removes all elements from the set
func (h *GlobalLockHashSet[T]) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.indexMap = make(map[T]int)
	h.elements = h.elements[:0]
}

// InsertAll adds multiple elements
func (h *GlobalLockHashSet[T]) InsertAll(elements ...T) int {
	added := 0
	for _, elem := range elements {
		if h.Insert(elem) {
			added++
		}
	}
	return added
}
