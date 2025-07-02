package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// HASHMAP TESTS (Key-Value functionality)
// =============================================================================

func TestHashMapBasicOperations(t *testing.T) {
	hashMap := NewHashMap[string, int]()

	// Test Put
	prev, existed := hashMap.Put("apple", 5)
	if existed {
		t.Error("Expected Put to return false for new key")
	}
	if prev != 0 {
		t.Error("Expected zero value for new key")
	}

	// Test updating existing key
	prev, existed = hashMap.Put("apple", 10)
	if !existed {
		t.Error("Expected Put to return true for existing key")
	}
	if prev != 5 {
		t.Errorf("Expected previous value 5, got %d", prev)
	}

	// Test Get
	if value, exists := hashMap.Get("apple"); !exists {
		t.Error("Expected Get to return true for existing key")
	} else if value != 10 {
		t.Errorf("Expected value 10, got %d", value)
	}

	if _, exists := hashMap.Get("banana"); exists {
		t.Error("Expected Get to return false for non-existing key")
	}

	// Test Size
	if hashMap.Size() != 1 {
		t.Errorf("Expected size 1, got %d", hashMap.Size())
	}

	// Test ContainsKey
	if !hashMap.ContainsKey("apple") {
		t.Error("Expected ContainsKey to return true for existing key")
	}
	if hashMap.ContainsKey("banana") {
		t.Error("Expected ContainsKey to return false for non-existing key")
	}

	// Test Remove
	if value, existed := hashMap.Remove("apple"); !existed {
		t.Error("Expected Remove to return true for existing key")
	} else if value != 10 {
		t.Errorf("Expected removed value 10, got %d", value)
	}

	if _, existed := hashMap.Remove("apple"); existed {
		t.Error("Expected Remove to return false for already removed key")
	}

	if hashMap.Size() != 0 {
		t.Errorf("Expected size 0 after removal, got %d", hashMap.Size())
	}
}

func TestHashMapRandomOperations(t *testing.T) {
	hashMap := NewHashMap[string, int]()
	hashMap.Put("a", 1)
	hashMap.Put("b", 2)
	hashMap.Put("c", 3)

	// Test RandomEntry
	key, value, ok := hashMap.RandomEntry()
	if !ok {
		t.Error("Expected RandomEntry to return true for non-empty map")
	}
	if expectedValue, exists := hashMap.Get(key); !exists || expectedValue != value {
		t.Error("Random entry should be consistent with map contents")
	}

	// Test RandomKey
	randomKey, ok := hashMap.RandomKey()
	if !ok {
		t.Error("Expected RandomKey to return true for non-empty map")
	}
	if !hashMap.ContainsKey(randomKey) {
		t.Error("Random key should exist in map")
	}

	// Test RandomValue
	randomValue, ok := hashMap.RandomValue()
	if !ok {
		t.Error("Expected RandomValue to return true for non-empty map")
	}

	// Verify random value exists in the map
	found := false
	for _, value := range hashMap.Values() {
		if value == randomValue {
			found = true
			break
		}
	}
	if !found {
		t.Error("Random value should exist in map")
	}
}

func TestHashMapKeysValuesEntries(t *testing.T) {
	hashMap := NewHashMap[string, int]()
	expected := map[string]int{
		"apple":  1,
		"banana": 2,
		"cherry": 3,
	}

	// Populate map
	for key, value := range expected {
		hashMap.Put(key, value)
	}

	// Test Keys
	keys := hashMap.Keys()
	if len(keys) != len(expected) {
		t.Errorf("Expected %d keys, got %d", len(expected), len(keys))
	}

	keySet := make(map[string]bool)
	for _, key := range keys {
		keySet[key] = true
	}

	for expectedKey := range expected {
		if !keySet[expectedKey] {
			t.Errorf("Expected key %s not found in Keys()", expectedKey)
		}
	}

	// Test Values
	values := hashMap.Values()
	if len(values) != len(expected) {
		t.Errorf("Expected %d values, got %d", len(expected), len(values))
	}

	// Test Entries
	entries := hashMap.Entries()
	if len(entries) != len(expected) {
		t.Errorf("Expected %d entries, got %d", len(expected), len(entries))
	}

	entryMap := make(map[string]int)
	for _, entry := range entries {
		entryMap[entry.Key] = entry.Value
	}

	for expectedKey, expectedValue := range expected {
		if entryMap[expectedKey] != expectedValue {
			t.Errorf("Expected entry %s:%d, got %s:%d", expectedKey, expectedValue, expectedKey, entryMap[expectedKey])
		}
	}
}

func TestHashMapIterator(t *testing.T) {
	hashMap := NewHashMap[string, int]()
	expected := map[string]int{
		"apple":  1,
		"banana": 2,
		"cherry": 3,
	}

	for key, value := range expected {
		hashMap.Put(key, value)
	}

	iterator := hashMap.NewIterator()
	count := 0
	foundEntries := make(map[string]int)

	for iterator.HasNext() {
		entry, ok := iterator.Next()
		if !ok {
			t.Error("Next() should return true when HasNext() is true")
		}
		foundEntries[entry.Key] = entry.Value
		count++
	}

	if count != len(expected) {
		t.Errorf("Expected to iterate over %d entries, got %d", len(expected), count)
	}

	for expectedKey, expectedValue := range expected {
		if foundEntries[expectedKey] != expectedValue {
			t.Errorf("Expected entry %s:%d, found %s:%d", expectedKey, expectedValue, expectedKey, foundEntries[expectedKey])
		}
	}

	// Test iterator exhaustion
	_, ok := iterator.Next()
	if ok {
		t.Error("Next() should return false when iterator is exhausted")
	}
}

func TestHashMapConcurrentOperations(t *testing.T) {
	hashMap := NewHashMap[string, int]()
	numGoroutines := 50
	operationsPerGoroutine := 1000

	var wg sync.WaitGroup

	// Concurrent Put operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				hashMap.Put(key, id*1000+j)
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * operationsPerGoroutine
	if hashMap.Size() != expectedSize {
		t.Errorf("Expected size %d after concurrent puts, got %d", expectedSize, hashMap.Size())
	}

	// Verify some entries
	for i := 0; i < min(10, numGoroutines); i++ {
		for j := 0; j < min(10, operationsPerGoroutine); j++ {
			key := fmt.Sprintf("key-%d-%d", i, j)
			expectedValue := i*1000 + j
			if value, exists := hashMap.Get(key); !exists {
				t.Errorf("Expected key %s to exist", key)
			} else if value != expectedValue {
				t.Errorf("Expected value %d for key %s, got %d", expectedValue, key, value)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// ADDITIONAL HASHMAP TESTS (Real usage scenarios)
// =============================================================================

func TestHashMapClientScenario(t *testing.T) {
	hashMap := NewHashMap[string, string]()

	// Test adding clients (id -> name)
	prev, existed := hashMap.Put("container1", "client-app-1")
	if existed {
		t.Error("Expected Put to return false for new client")
	}
	if prev != "" {
		t.Error("Expected empty string for new client")
	}

	// Test updating client name
	prev, existed = hashMap.Put("container1", "client-app-updated")
	if !existed {
		t.Error("Expected Put to return true for existing client")
	}
	if prev != "client-app-1" {
		t.Errorf("Expected previous name 'client-app-1', got '%s'", prev)
	}

	// Test getting client by ID
	if name, exists := hashMap.Get("container1"); !exists {
		t.Error("Expected Get to return true for existing client")
	} else if name != "client-app-updated" {
		t.Errorf("Expected name 'client-app-updated', got '%s'", name)
	}

	// Test multiple clients
	hashMap.Put("container2", "client-web-1")
	hashMap.Put("container3", "client-db-1")

	if hashMap.Size() != 3 {
		t.Errorf("Expected size 3, got %d", hashMap.Size())
	}

	// Test removing client by ID
	name, existed := hashMap.Remove("container2")
	if !existed {
		t.Error("Expected Remove to return true for existing client")
	}
	if name != "client-web-1" {
		t.Errorf("Expected removed name 'client-web-1', got '%s'", name)
	}

	if hashMap.Size() != 2 {
		t.Errorf("Expected size 2 after removal, got %d", hashMap.Size())
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkHashMapPut(b *testing.B) {
	hashMap := NewHashMap[string, int]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			hashMap.Put(fmt.Sprintf("key-%d", i), i)
			i++
		}
	})
}

func BenchmarkHashMapGet(b *testing.B) {
	hashMap := NewHashMap[string, int]()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		hashMap.Put(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			hashMap.Get(fmt.Sprintf("key-%d", i%10000))
			i++
		}
	})
}

func BenchmarkHashMapRandomEntry(b *testing.B) {
	hashMap := NewHashMap[string, int]()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		hashMap.Put(fmt.Sprintf("key-%d", i), i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hashMap.RandomEntry()
		}
	})
}

func BenchmarkHashMapRemove(b *testing.B) {
	// This benchmark needs to be careful since we can't remove the same element twice
	b.StopTimer()

	numElements := b.N
	hashMap := NewHashMap[string, string]()

	// Pre-populate with enough elements for the benchmark
	for i := 0; i < numElements; i++ {
		hashMap.Put(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	b.StartTimer()

	// Sequential removes since we can't remove the same element twice
	for i := 0; i < numElements; i++ {
		hashMap.Remove(fmt.Sprintf("key-%d", i))
	}
}

// =============================================================================
// PERSISTENCE TESTS
// =============================================================================

func TestHashMapPersistenceBasic(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_hashmap.json")

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         filePath,
		SnapshotInterval: 100 * time.Millisecond,
		MaxRetries:       3,
	}

	// Create hashmap with persistence
	hashMap := NewHashMapWithPersistence[string, int](config)
	defer hashMap.Close()

	// Add some data
	testData := map[string]int{
		"apple":  1,
		"banana": 2,
		"cherry": 3,
	}

	for key, value := range testData {
		hashMap.Put(key, value)
	}

	// Trigger snapshot
	if err := hashMap.TriggerSnapshot(); err != nil {
		t.Fatalf("Failed to trigger snapshot: %v", err)
	}

	// Wait a bit for persistence to complete
	time.Sleep(200 * time.Millisecond)

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Snapshot file should exist")
	}

	// Create new hashmap and load from disk
	newHashMap := NewHashMap[string, int]()
	if err := newHashMap.LoadFromDisk(filePath); err != nil {
		t.Fatalf("Failed to load from disk: %v", err)
	}

	// Verify data
	if newHashMap.Size() != len(testData) {
		t.Errorf("Expected size %d, got %d", len(testData), newHashMap.Size())
	}

	for key, expectedValue := range testData {
		if value, exists := newHashMap.Get(key); !exists {
			t.Errorf("Expected key %s to exist", key)
		} else if value != expectedValue {
			t.Errorf("Expected value %d for key %s, got %d", expectedValue, key, value)
		}
	}
}

func TestHashMapPersistenceStringToString(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_client_map.json")

	config := PersistenceConfig{
		Enabled:          true,
		FilePath:         filePath,
		SnapshotInterval: 100 * time.Millisecond,
		MaxRetries:       3,
	}

	// Create client map with persistence (id -> name)
	clientMap := NewHashMapWithPersistence[string, string](config)
	defer clientMap.Close()

	// Add some client data
	clients := map[string]string{
		"container1": "client-app-1",
		"container2": "client-web-1",
		"container3": "client-db-1",
	}

	for id, name := range clients {
		clientMap.Put(id, name)
	}

	// Trigger snapshot
	if err := clientMap.TriggerSnapshot(); err != nil {
		t.Fatalf("Failed to trigger snapshot: %v", err)
	}

	// Wait for persistence
	time.Sleep(200 * time.Millisecond)

	// Create new map and load from disk
	newClientMap := NewHashMap[string, string]()
	if err := newClientMap.LoadFromDisk(filePath); err != nil {
		t.Fatalf("Failed to load from disk: %v", err)
	}

	// Verify data
	if newClientMap.Size() != len(clients) {
		t.Errorf("Expected size %d, got %d", len(clients), newClientMap.Size())
	}

	for id, expectedName := range clients {
		if name, exists := newClientMap.Get(id); !exists {
			t.Errorf("Expected client ID %s to exist", id)
		} else if name != expectedName {
			t.Errorf("Expected name %s for client ID %s, got %s", expectedName, id, name)
		}
	}
}
