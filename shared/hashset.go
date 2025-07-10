package shared

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
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

// Entry represents a key-value pair
type Entry[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

// Snapshot represents a consistent point-in-time view of the HashMap
type Snapshot[K comparable, V any] struct {
	Version   int64         `json:"version"`
	Timestamp time.Time     `json:"timestamp"`
	Entries   []Entry[K, V] `json:"entries"`
}

// PersistenceConfig controls durability behavior
type PersistenceConfig struct {
	Enabled          bool          `json:"enabled"`
	FilePath         string        `json:"file_path"`
	SnapshotInterval time.Duration `json:"snapshot_interval"`
	MaxRetries       int           `json:"max_retries"`
}

// HashMap represents a thread-safe map with O(1) random access and durability
type HashMap[K comparable, V any] struct {
	mu       sync.RWMutex
	indexMap map[K]int
	keys     []K
	values   []V
	hasher   Hasher[K]

	// Persistence fields
	persistence    PersistenceConfig
	persistChan    chan struct{}
	stopChan       chan struct{}
	persistWG      sync.WaitGroup
	currentVersion int64
	versionMutex   sync.RWMutex
}

const (
	defaultInitialCapacity = 16
)

// NewHashMap creates a new HashMap
func NewHashMap[K comparable, V any]() *HashMap[K, V] {
	return &HashMap[K, V]{
		indexMap: make(map[K]int),
		keys:     make([]K, 0, defaultInitialCapacity),
		values:   make([]V, 0, defaultInitialCapacity),
		hasher:   defaultHasher[K]{},
	}
}

// NewHashMapWithPersistence creates a HashMap with persistence enabled
func NewHashMapWithPersistence[K comparable, V any](config PersistenceConfig) *HashMap[K, V] {
	h := NewHashMap[K, V]()
	if config.Enabled {
		h.EnablePersistence(config)
	}
	return h
}

// Put adds or updates a key-value pair, returns previous value if key existed
func (h *HashMap[K, V]) Put(key K, value V) (V, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var zero V

	// Check if key already exists
	if index, exists := h.indexMap[key]; exists {
		// Update existing value
		oldValue := h.values[index]
		h.values[index] = value
		return oldValue, true
	}

	// Add new key-value pair
	index := len(h.keys)
	h.keys = append(h.keys, key)
	h.values = append(h.values, value)
	h.indexMap[key] = index

	return zero, false
}

// Get retrieves the value for a key
func (h *HashMap[K, V]) Get(key K) (V, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var zero V
	index, exists := h.indexMap[key]
	if !exists {
		return zero, false
	}

	return h.values[index], true
}

// Remove deletes a key-value pair, returns the removed value
func (h *HashMap[K, V]) Remove(key K) (V, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var zero V
	index, exists := h.indexMap[key]
	if !exists {
		return zero, false
	}

	// Get the value being removed
	removedValue := h.values[index]

	// Get the last elements
	lastIndex := len(h.keys) - 1

	// If removing last element, just truncate
	if index == lastIndex {
		h.keys = h.keys[:lastIndex]
		h.values = h.values[:lastIndex]
	} else {
		// Swap with last elements
		lastKey := h.keys[lastIndex]
		lastValue := h.values[lastIndex]

		h.keys[index] = lastKey
		h.values[index] = lastValue
		h.indexMap[lastKey] = index

		// Truncate slices
		h.keys = h.keys[:lastIndex]
		h.values = h.values[:lastIndex]
	}

	// Remove from index map
	delete(h.indexMap, key)

	return removedValue, true
}

// ContainsKey checks if key exists in the map
func (h *HashMap[K, V]) ContainsKey(key K) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, exists := h.indexMap[key]
	return exists
}

// RandomEntry returns a random key-value pair from the map
func (h *HashMap[K, V]) RandomEntry() (K, V, bool) {
	var zeroK K
	var zeroV V
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keys) == 0 {
		return zeroK, zeroV, false
	}

	// Use thread-local random to avoid contention
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(h.keys))
	return h.keys[index], h.values[index], true
}

// RandomKey returns a random key from the map
func (h *HashMap[K, V]) RandomKey() (K, bool) {
	var zero K
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keys) == 0 {
		return zero, false
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(h.keys))
	return h.keys[index], true
}

// RandomValue returns a random value from the map
func (h *HashMap[K, V]) RandomValue() (V, bool) {
	var zero V
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.values) == 0 {
		return zero, false
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(h.values))
	return h.values[index], true
}

// Size returns the total number of key-value pairs
func (h *HashMap[K, V]) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.keys)
}

// Clear removes all key-value pairs from the map
func (h *HashMap[K, V]) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.indexMap = make(map[K]int)
	h.keys = h.keys[:0]
	h.values = h.values[:0]
}

// PutAll adds multiple key-value pairs
func (h *HashMap[K, V]) PutAll(entries ...Entry[K, V]) int {
	updated := 0
	for _, entry := range entries {
		if _, existed := h.Put(entry.Key, entry.Value); existed {
			updated++
		}
	}
	return updated
}

// ContainsAllKeys checks if all keys exist in the map
func (h *HashMap[K, V]) ContainsAllKeys(keys ...K) bool {
	for _, key := range keys {
		if !h.ContainsKey(key) {
			return false
		}
	}
	return true
}

// RandomEntries returns n random key-value pairs (with possible duplicates)
func (h *HashMap[K, V]) RandomEntries(n int) []Entry[K, V] {
	results := make([]Entry[K, V], 0, n)
	for i := 0; i < n; i++ {
		if key, value, ok := h.RandomEntry(); ok {
			results = append(results, Entry[K, V]{Key: key, Value: value})
		}
	}
	return results
}

// IsEmpty checks if the map is empty
func (h *HashMap[K, V]) IsEmpty() bool {
	return h.Size() == 0
}

// Keys returns all keys as a slice
func (h *HashMap[K, V]) Keys() []K {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]K, len(h.keys))
	copy(result, h.keys)
	return result
}

// Values returns all values as a slice
func (h *HashMap[K, V]) Values() []V {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]V, len(h.values))
	copy(result, h.values)
	return result
}

// Entries returns all key-value pairs as a slice
func (h *HashMap[K, V]) Entries() []Entry[K, V] {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]Entry[K, V], len(h.keys))
	for i := 0; i < len(h.keys); i++ {
		result[i] = Entry[K, V]{Key: h.keys[i], Value: h.values[i]}
	}
	return result
}

// Iterator provides safe iteration over map entries
type Iterator[K comparable, V any] struct {
	hashMap *HashMap[K, V]
	entries []Entry[K, V]
	index   int
}

// NewIterator creates a snapshot-based iterator
func (h *HashMap[K, V]) NewIterator() *Iterator[K, V] {
	return &Iterator[K, V]{
		hashMap: h,
		entries: h.Entries(),
		index:   0,
	}
}

// HasNext checks if more entries exist
func (it *Iterator[K, V]) HasNext() bool {
	return it.index < len(it.entries)
}

// Next returns the next key-value pair
func (it *Iterator[K, V]) Next() (Entry[K, V], bool) {
	var zero Entry[K, V]
	if !it.HasNext() {
		return zero, false
	}
	entry := it.entries[it.index]
	it.index++
	return entry, true
}

// Stats returns statistics about the map
type Stats struct {
	TotalEntries int
	LoadFactor   float64
}

func (h *HashMap[K, V]) Stats() Stats {
	size := h.Size()
	return Stats{
		TotalEntries: size,
		LoadFactor:   float64(size),
	}
}

// =============================================================================
// PERSISTENCE METHODS
// =============================================================================

// EnablePersistence activates durability for the HashMap
func (h *HashMap[K, V]) EnablePersistence(config PersistenceConfig) error {
	if h.persistence.Enabled {
		return fmt.Errorf("persistence already enabled")
	}

	// Set default values
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.SnapshotInterval == 0 {
		config.SnapshotInterval = 30 * time.Second
	}

	h.persistence = config
	h.persistChan = make(chan struct{}, 1)
	h.stopChan = make(chan struct{})

	// Start background persistence worker
	h.persistWG.Add(1)
	go h.persistenceWorker()

	return nil
}

// CreateSnapshot creates a consistent point-in-time snapshot
func (h *HashMap[K, V]) CreateSnapshot() *Snapshot[K, V] {
	// Increment version atomically
	h.versionMutex.Lock()
	h.currentVersion++
	version := h.currentVersion
	h.versionMutex.Unlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot := &Snapshot[K, V]{
		Version:   version,
		Timestamp: time.Now(),
		Entries:   make([]Entry[K, V], len(h.keys)),
	}

	// Copy all key-value pairs
	for i := 0; i < len(h.keys); i++ {
		snapshot.Entries[i] = Entry[K, V]{
			Key:   h.keys[i],
			Value: h.values[i],
		}
	}

	return snapshot
}

// TriggerSnapshot manually triggers a snapshot to be persisted
func (h *HashMap[K, V]) TriggerSnapshot() error {
	if !h.persistence.Enabled {
		return fmt.Errorf("persistence not enabled")
	}

	// Non-blocking trigger
	select {
	case h.persistChan <- struct{}{}:
		return nil
	default:
		return fmt.Errorf("persistence already in progress")
	}
}

// persistenceWorker runs in background handling periodic snapshots
func (h *HashMap[K, V]) persistenceWorker() {
	defer h.persistWG.Done()

	ticker := time.NewTicker(h.persistence.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			// Final snapshot before shutdown
			h.persistSnapshot()
			return

		case <-ticker.C:
			// Periodic snapshot
			h.persistSnapshot()

		case <-h.persistChan:
			// Manual trigger
			h.persistSnapshot()
		}
	}
}

// persistSnapshot performs the actual persistence operation
func (h *HashMap[K, V]) persistSnapshot() {
	snapshot := h.CreateSnapshot()

	// Retry logic for failed writes
	var err error
	for attempt := 0; attempt < h.persistence.MaxRetries; attempt++ {
		err = h.writeSnapshotToDisk(snapshot)
		if err == nil {
			break
		}

		// Exponential backoff on retry
		if attempt < h.persistence.MaxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * time.Second)
		}
	}

	if err != nil {
		fmt.Printf("Failed to persist snapshot after %d attempts: %v\n",
			h.persistence.MaxRetries, err)
	}
}

// writeSnapshotToDisk atomically writes snapshot to disk
func (h *HashMap[K, V]) writeSnapshotToDisk(snapshot *Snapshot[K, V]) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(h.persistence.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file first (atomic write pattern)
	tempFile := h.persistence.FilePath + ".tmp"

	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	// Encode to JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty printing
	if err := encoder.Encode(snapshot); err != nil {
		os.Remove(tempFile) // Clean up on failure
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	// Ensure data is written to disk
	if err := file.Sync(); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to sync file: %w", err)
	}

	file.Close()

	// Atomic rename (this is the atomic operation)
	if err := os.Rename(tempFile, h.persistence.FilePath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// LoadFromDisk restores the HashMap from a persisted snapshot
func (h *HashMap[K, V]) LoadFromDisk(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %w", err)
	}
	defer file.Close()

	var snapshot Snapshot[K, V]
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Clear current data
	h.indexMap = make(map[K]int)
	h.keys = make([]K, len(snapshot.Entries))
	h.values = make([]V, len(snapshot.Entries))

	// Restore data
	for idx, entry := range snapshot.Entries {
		h.keys[idx] = entry.Key
		h.values[idx] = entry.Value
		h.indexMap[entry.Key] = idx
	}

	// Update version
	h.versionMutex.Lock()
	h.currentVersion = snapshot.Version
	h.versionMutex.Unlock()

	return nil
}

// GetCurrentVersion returns the current version number
func (h *HashMap[K, V]) GetCurrentVersion() int64 {
	h.versionMutex.RLock()
	defer h.versionMutex.RUnlock()
	return h.currentVersion
}

// Close properly shuts down persistence and ensures final snapshot
func (h *HashMap[K, V]) Close() error {
	if !h.persistence.Enabled {
		return nil
	}

	// Signal shutdown
	close(h.stopChan)

	// Wait for persistence worker to finish
	h.persistWG.Wait()

	return nil
}
