package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Global HashSet instance for the demo
var demoSet *HashSet[string]
var globalLockSet *GlobalLockHashSet[string]
var mu sync.RWMutex

func init() {
	// Initialize with some demo data
	demoSet = NewHashSetWithShards[string](32)
	globalLockSet = NewGlobalLockHashSet[string]()
	demoSet.InsertAll("apple", "banana", "cherry", "date", "elderberry", "fig", "grape")
	globalLockSet.InsertAll("apple", "banana", "cherry", "date", "elderberry", "fig", "grape")
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	stats := demoSet.Stats()
	elements := demoSet.ToSlice()
	mu.RUnlock()

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Thread-Safe HashSet Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 800px; margin: 0 auto; }
        .stats { background: #f0f0f0; padding: 15px; margin: 10px 0; border-radius: 5px; }
        .operations { margin: 20px 0; }
        .button { background: #007cba; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; margin: 5px; display: inline-block; }
        .button:hover { background: #005a8b; }
        .elements { background: #e8f4f8; padding: 15px; margin: 10px 0; border-radius: 5px; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 4px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Thread-Safe HashSet Demo (Generic)</h1>
        
        <div class="stats">
            <h3>Current Statistics</h3>
            <p><strong>Total Elements:</strong> %d</p>
            <p><strong>Shard Count:</strong> %d</p>
            <p><strong>Load Factor:</strong> %.2f elements per shard</p>
            <p><strong>Elements per Shard:</strong> %v</p>
        </div>

        <div class="elements">
            <h3>Current Elements (string type)</h3>
            <pre>%v</pre>
        </div>

        <div class="operations">
            <h3>Operations</h3>
            <a href="/insert" class="button">Add Random Element</a>
            <a href="/remove" class="button">Remove Random Element</a>
            <a href="/random" class="button">Get Random Element</a>
            <a href="/clear" class="button">Clear All</a>
            <a href="/populate" class="button">Add 100 Random Items</a>
            <a href="/stats" class="button">Detailed Stats</a>
            <a href="/test" class="button">Run Concurrency Test</a>
            <a href="/compare" class="button" style="background: #e74c3c;">Compare vs Global Lock</a>
        </div>

        <div>
            <h3>Manual Operations</h3>
            <form action="/insert" method="get" style="display: inline;">
                <input type="text" name="element" placeholder="Element to insert">
                <input type="submit" value="Insert" class="button">
            </form>
            
            <form action="/remove" method="get" style="display: inline;">
                <input type="text" name="element" placeholder="Element to remove">
                <input type="submit" value="Remove" class="button">
            </form>
        </div>

        <div style="margin-top: 30px;">
            <h3>About This HashSet (Generic Version)</h3>
            <ul>
                <li><strong>Type-Safe:</strong> Uses Go generics with HashSet[string] type</li>
                <li><strong>Thread-Safe:</strong> Uses fine-grained locking with sharding</li>
                <li><strong>O(1) Operations:</strong> Insert, Remove, Contains, and Random access</li>
                <li><strong>Efficient Random Access:</strong> Using contiguous arrays within shards</li>
                <li><strong>Swap-and-Pop Removal:</strong> Maintains O(1) removal complexity</li>
                <li><strong>Configurable Sharding:</strong> Optimized for concurrent workloads</li>
                <li><strong>Zero Boxing Overhead:</strong> Direct type storage without interface{}</li>
            </ul>
        </div>
    </div>
</body>
</html>
`, stats.TotalElements, stats.ShardCount, stats.LoadFactor, stats.ElementsPerShard, elements)
}

func insertHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	element := r.URL.Query().Get("element")
	if element == "" {
		element = fmt.Sprintf("item-%d", time.Now().UnixNano())
	}

	mu.Lock()
	added := demoSet.Insert(element)
	mu.Unlock()
	elapsed := time.Since(start)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Refresh", "2; url=/")
	if added {
		fmt.Fprintf(w, "Successfully inserted: %s\nOperation took: %v\nRedirecting...", element, elapsed)
	} else {
		fmt.Fprintf(w, "Element already exists: %s\nOperation took: %v\nRedirecting...", element, elapsed)
	}
}

func removeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	element := r.URL.Query().Get("element")

	mu.Lock()
	if element == "" {
		// Remove random element
		if randomElem, ok := demoSet.RandomElement(); ok {
			element = randomElem
			demoSet.Remove(randomElem)
		}
	} else {
		demoSet.Remove(element)
	}
	mu.Unlock()
	elapsed := time.Since(start)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Refresh", "2; url=/")
	if element != "" {
		fmt.Fprintf(w, "Removed: %s\nOperation took: %v\nRedirecting...", element, elapsed)
	} else {
		fmt.Fprintf(w, "No elements to remove\nOperation took: %v\nRedirecting...", elapsed)
	}
}

func randomHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	mu.RLock()
	element, ok := demoSet.RandomElement()
	mu.RUnlock()
	elapsed := time.Since(start)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Refresh", "2; url=/")
	if ok {
		fmt.Fprintf(w, "Random element: %v\nOperation took: %v\nRedirecting...", element, elapsed)
	} else {
		fmt.Fprintf(w, "Set is empty\nOperation took: %v\nRedirecting...", elapsed)
	}
}

func clearHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	mu.Lock()
	oldSize := demoSet.Size()
	demoSet.Clear()
	mu.Unlock()
	elapsed := time.Since(start)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Refresh", "2; url=/")
	fmt.Fprintf(w, "Set cleared (%d elements removed)\nOperation took: %v\nRedirecting...", oldSize, elapsed)
}

func populateHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	mu.Lock()
	added := 0
	for i := 0; i < 100; i++ {
		element := fmt.Sprintf("random-%d-%d", time.Now().UnixNano(), i)
		if demoSet.Insert(element) {
			added++
		}
	}
	mu.Unlock()
	elapsed := time.Since(start)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Refresh", "2; url=/")
	fmt.Fprintf(w, "Added %d new elements\nOperation took: %v\nThroughput: %.0f ops/sec\nRedirecting...",
		added, elapsed, float64(added)/elapsed.Seconds())
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	stats := demoSet.Stats()
	mu.RUnlock()

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>HashSet Detailed Statistics</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 800px; margin: 0 auto; }
        table { border-collapse: collapse; width: 100%%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .back { background: #007cba; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Detailed Statistics (Generic HashSet[string])</h1>
        
        <h3>Overall Stats</h3>
        <p><strong>Total Elements:</strong> %d</p>
        <p><strong>Shard Count:</strong> %d</p>
        <p><strong>Load Factor:</strong> %.2f</p>
        
        <h3>Per-Shard Distribution</h3>
        <table>
            <tr><th>Shard Index</th><th>Element Count</th><th>Percentage</th></tr>`,
		stats.TotalElements, stats.ShardCount, stats.LoadFactor)

	for i, count := range stats.ElementsPerShard {
		percentage := 0.0
		if stats.TotalElements > 0 {
			percentage = float64(count) / float64(stats.TotalElements) * 100
		}
		fmt.Fprintf(w, "<tr><td>%d</td><td>%d</td><td>%.1f%%</td></tr>", i, count, percentage)
	}

	fmt.Fprintf(w, `
        </table>
        
        <p><a href="/" class="back">← Back to Home</a></p>
    </div>
</body>
</html>`)
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Concurrency Test</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 800px; margin: 0 auto; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 4px; overflow-x: auto; }
        .back { background: #007cba; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Concurrency Test Results (Generic HashSet[string])</h1>
        <pre>`)

	// Run concurrency test
	testSet := NewHashSetWithShards[string](64)
	numGoroutines := 100
	elementsPerGoroutine := 10000

	fmt.Fprintf(w, "Running concurrency test...\n")
	fmt.Fprintf(w, "Goroutines: %d\n", numGoroutines)
	fmt.Fprintf(w, "Elements per goroutine: %d\n", elementsPerGoroutine)
	fmt.Fprintf(w, "Expected total elements: %d\n\n", numGoroutines*elementsPerGoroutine)

	start := time.Now()
	var wg sync.WaitGroup

	// Concurrent insertions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerGoroutine; j++ {
				testSet.Insert(fmt.Sprintf("test-%d-%d", id, j))
			}
		}(i)
	}

	// Concurrent random access
	randomCount := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500000; i++ {
			if _, ok := testSet.RandomElement(); ok {
				randomCount++
			}
		}
	}()

	wg.Wait()
	elapsed := time.Since(start)

	finalStats := testSet.Stats()

	fmt.Fprintf(w, "Test completed in: %v\n", elapsed)
	fmt.Fprintf(w, "Final size: %d\n", finalStats.TotalElements)
	fmt.Fprintf(w, "Success rate: %.2f%%\n", float64(finalStats.TotalElements)/float64(numGoroutines*elementsPerGoroutine)*100)
	fmt.Fprintf(w, "Random accesses completed: %d\n", randomCount)
	fmt.Fprintf(w, "Load factor: %.2f\n", finalStats.LoadFactor)
	fmt.Fprintf(w, "Elements per shard: %v\n", finalStats.ElementsPerShard)

	fmt.Fprintf(w, `
        </pre>
        <p><a href="/" class="back">← Back to Home</a></p>
    </div>
</body>
</html>`)
}

func compareHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Performance Comparison</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1000px; margin: 0 auto; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 4px; overflow-x: auto; }
        .back { background: #007cba; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; }
        .comparison { display: flex; gap: 20px; margin: 20px 0; }
        .result { flex: 1; background: #f0f0f0; padding: 15px; border-radius: 5px; }
        .winner { background: #d4edda; border: 2px solid #c3e6cb; }
        .loser { background: #f8d7da; border: 2px solid #f5c6cb; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Performance Comparison: Sharded vs Global Lock</h1>
        <pre>`)

	fmt.Fprintf(w, "Running comprehensive performance comparison...\n\n")

	// Test 1: Single-threaded operations
	fmt.Fprintf(w, "=== Test 1: Single-threaded Operations ===\n")

	// Create fresh instances for testing
	shardedSet := NewHashSetWithShards[string](32)
	globalSet := NewGlobalLockHashSet[string]()

	// Test insertions
	numOps := 10000
	fmt.Fprintf(w, "Inserting %d elements:\n", numOps)

	start := time.Now()
	for i := 0; i < numOps; i++ {
		shardedSet.Insert(fmt.Sprintf("item-%d", i))
	}
	shardedTime := time.Since(start)

	start = time.Now()
	for i := 0; i < numOps; i++ {
		globalSet.Insert(fmt.Sprintf("item-%d", i))
	}
	globalTime := time.Since(start)

	fmt.Fprintf(w, "  Sharded:     %v (%.0f ops/sec)\n", shardedTime, float64(numOps)/shardedTime.Seconds())
	fmt.Fprintf(w, "  Global Lock: %v (%.0f ops/sec)\n", globalTime, float64(numOps)/globalTime.Seconds())
	fmt.Fprintf(w, "  Speedup:     %.2fx\n\n", float64(globalTime)/float64(shardedTime))

	// Test lookups
	fmt.Fprintf(w, "Performing %d lookups:\n", numOps)

	start = time.Now()
	for i := 0; i < numOps; i++ {
		shardedSet.Contains(fmt.Sprintf("item-%d", i))
	}
	shardedLookupTime := time.Since(start)

	start = time.Now()
	for i := 0; i < numOps; i++ {
		globalSet.Contains(fmt.Sprintf("item-%d", i))
	}
	globalLookupTime := time.Since(start)

	fmt.Fprintf(w, "  Sharded:     %v (%.0f ops/sec)\n", shardedLookupTime, float64(numOps)/shardedLookupTime.Seconds())
	fmt.Fprintf(w, "  Global Lock: %v (%.0f ops/sec)\n", globalLookupTime, float64(numOps)/globalLookupTime.Seconds())
	fmt.Fprintf(w, "  Speedup:     %.2fx\n\n", float64(globalLookupTime)/float64(shardedLookupTime))

	// Test 2: Concurrent operations
	fmt.Fprintf(w, "=== Test 2: High Concurrency, Low Ops Per Goroutine ===\n")
	fmt.Fprintf(w, "This simulates a realistic scenario like a web server with many\n")
	fmt.Fprintf(w, "concurrent requests, each doing just a few operations.\n\n")

	numGoroutines := 5000
	opsPerGoroutine := 5
	fmt.Fprintf(w, "Concurrent insertions (%d goroutines, %d ops each = %d total):\n",
		numGoroutines, opsPerGoroutine, numGoroutines*opsPerGoroutine)

	// Fresh instances for concurrent test
	shardedSet = NewHashSetWithShards[string](32)
	globalSet = NewGlobalLockHashSet[string]()

	var wg sync.WaitGroup

	// Test sharded version
	start = time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				shardedSet.Insert(fmt.Sprintf("concurrent-%d-%d", id, j))
			}
		}(i)
	}
	wg.Wait()
	shardedConcurrentTime := time.Since(start)

	// Test global lock version
	start = time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				globalSet.Insert(fmt.Sprintf("concurrent-%d-%d", id, j))
			}
		}(i)
	}
	wg.Wait()
	globalConcurrentTime := time.Since(start)

	totalOps := numGoroutines * opsPerGoroutine
	fmt.Fprintf(w, "  Sharded:     %v (%.0f ops/sec)\n", shardedConcurrentTime, float64(totalOps)/shardedConcurrentTime.Seconds())
	fmt.Fprintf(w, "  Global Lock: %v (%.0f ops/sec)\n", globalConcurrentTime, float64(totalOps)/globalConcurrentTime.Seconds())
	fmt.Fprintf(w, "  Speedup:     %.2fx\n\n", float64(globalConcurrentTime)/float64(shardedConcurrentTime))

	// Test 2.5: Force actual lock contention by accessing same elements
	fmt.Fprintf(w, "=== Test 2.5: Forced Lock Contention (Same Data Access) ===\n")
	fmt.Fprintf(w, "Force contention by having many goroutines access the same elements\n")
	fmt.Fprintf(w, "and use a start barrier to ensure simultaneous execution.\n\n")

	numContentionGoroutines := 1000
	contentionOps := 20
	fmt.Fprintf(w, "Contention test (%d goroutines, %d ops each, accessing same elements):\n",
		numContentionGoroutines, contentionOps)

	// Fresh instances
	shardedSet = NewHashSetWithShards[string](32)
	globalSet = NewGlobalLockHashSet[string]()

	// Pre-populate with a small set of "hot" elements that everyone will access
	hotElements := []string{"hot1", "hot2", "hot3", "hot4", "hot5"}
	for _, elem := range hotElements {
		shardedSet.Insert(elem)
		globalSet.Insert(elem)
	}

	// Test sharded version with forced contention
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	start = time.Now()
	for i := 0; i < numContentionGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			startBarrier.Wait() // Wait for all goroutines to be ready

			for j := 0; j < contentionOps; j++ {
				// Access the same "hot" elements to force contention
				hotElement := hotElements[j%len(hotElements)]
				shardedSet.Contains(hotElement)
				shardedSet.Insert(fmt.Sprintf("%s-variant-%d", hotElement, id))
				shardedSet.RandomElement()
			}
		}(i)
	}
	startBarrier.Done() // Release all goroutines simultaneously
	wg.Wait()
	shardedContentionTime := time.Since(start)

	// Test global lock version with forced contention
	startBarrier.Add(1)

	start = time.Now()
	for i := 0; i < numContentionGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			startBarrier.Wait() // Wait for all goroutines to be ready

			for j := 0; j < contentionOps; j++ {
				// Same operations as sharded version
				hotElement := hotElements[j%len(hotElements)]
				globalSet.Contains(hotElement)
				globalSet.Insert(fmt.Sprintf("%s-variant-%d", hotElement, id))
				globalSet.RandomElement()
			}
		}(i)
	}
	startBarrier.Done() // Release all goroutines simultaneously
	wg.Wait()
	globalContentionTime := time.Since(start)

	totalContentionOps := numContentionGoroutines * contentionOps * 3 // 3 ops per iteration
	fmt.Fprintf(w, "  Sharded:     %v (%.0f ops/sec)\n", shardedContentionTime, float64(totalContentionOps)/shardedContentionTime.Seconds())
	fmt.Fprintf(w, "  Global Lock: %v (%.0f ops/sec)\n", globalContentionTime, float64(totalContentionOps)/globalContentionTime.Seconds())
	fmt.Fprintf(w, "  Speedup:     %.2fx\n\n", float64(globalContentionTime)/float64(shardedContentionTime))

	// Test 3: Mixed concurrent operations with many short-lived goroutines
	fmt.Fprintf(w, "=== Test 3: Mixed Operations (Many Short-Lived Goroutines) ===\n")
	fmt.Fprintf(w, "Simulating many concurrent workers each doing mixed operations.\n\n")

	// Pre-populate sets with reasonable base data
	for i := 0; i < 10000; i++ {
		element := fmt.Sprintf("base-%d", i)
		shardedSet.Insert(element)
		globalSet.Insert(element)
	}

	numMixedGoroutines := 2000
	fmt.Fprintf(w, "Running %d goroutines, each doing 1-2 writes, 3-5 reads, 1-2 random access:\n", numMixedGoroutines)

	// Test sharded version
	start = time.Now()
	for i := 0; i < numMixedGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine does a few mixed operations (realistic for web requests)
			// 1-2 writes
			shardedSet.Insert(fmt.Sprintf("write-%d-1", id))
			if id%2 == 0 {
				shardedSet.Insert(fmt.Sprintf("write-%d-2", id))
			}

			// 3-5 reads
			shardedSet.Contains(fmt.Sprintf("base-%d", id%10000))
			shardedSet.Contains(fmt.Sprintf("base-%d", (id*7)%10000))
			shardedSet.Contains(fmt.Sprintf("base-%d", (id*13)%10000))
			if id%3 == 0 {
				shardedSet.Contains(fmt.Sprintf("base-%d", (id*17)%10000))
				shardedSet.Contains(fmt.Sprintf("base-%d", (id*19)%10000))
			}

			// 1-2 random accesses
			shardedSet.RandomElement()
			if id%4 == 0 {
				shardedSet.RandomElement()
			}
		}(i)
	}
	wg.Wait()
	shardedMixedTime := time.Since(start)

	// Test global lock version
	start = time.Now()
	for i := 0; i < numMixedGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Same operations as sharded version
			globalSet.Insert(fmt.Sprintf("write-%d-1", id))
			if id%2 == 0 {
				globalSet.Insert(fmt.Sprintf("write-%d-2", id))
			}

			globalSet.Contains(fmt.Sprintf("base-%d", id%10000))
			globalSet.Contains(fmt.Sprintf("base-%d", (id*7)%10000))
			globalSet.Contains(fmt.Sprintf("base-%d", (id*13)%10000))
			if id%3 == 0 {
				globalSet.Contains(fmt.Sprintf("base-%d", (id*17)%10000))
				globalSet.Contains(fmt.Sprintf("base-%d", (id*19)%10000))
			}

			globalSet.RandomElement()
			if id%4 == 0 {
				globalSet.RandomElement()
			}
		}(i)
	}
	wg.Wait()
	globalMixedTime := time.Since(start)

	// Estimate total operations (approximate)
	avgOpsPerGoroutine := 1.5 + 3.7 + 1.25 // writes + reads + random
	totalMixedOps := int(float64(numMixedGoroutines) * avgOpsPerGoroutine)

	fmt.Fprintf(w, "  Sharded:     %v (%.0f ops/sec)\n", shardedMixedTime, float64(totalMixedOps)/shardedMixedTime.Seconds())
	fmt.Fprintf(w, "  Global Lock: %v (%.0f ops/sec)\n", globalMixedTime, float64(totalMixedOps)/globalMixedTime.Seconds())
	fmt.Fprintf(w, "  Speedup:     %.2fx\n\n", float64(globalMixedTime)/float64(shardedMixedTime))

	// Summary
	fmt.Fprintf(w, "=== Summary ===\n")
	fmt.Fprintf(w, "Single-threaded Insert Speedup:  %.2fx\n", float64(globalTime)/float64(shardedTime))
	fmt.Fprintf(w, "Single-threaded Lookup Speedup:  %.2fx\n", float64(globalLookupTime)/float64(shardedLookupTime))
	fmt.Fprintf(w, "High Concurrency Insert Speedup: %.2fx\n", float64(globalConcurrentTime)/float64(shardedConcurrentTime))
	fmt.Fprintf(w, "Forced Contention Speedup:       %.2fx\n", float64(globalContentionTime)/float64(shardedContentionTime))
	fmt.Fprintf(w, "Mixed Operations Speedup:        %.2fx\n\n", float64(globalMixedTime)/float64(shardedMixedTime))

	overallSpeedup := (float64(globalTime)/float64(shardedTime) +
		float64(globalLookupTime)/float64(shardedLookupTime) +
		float64(globalConcurrentTime)/float64(shardedConcurrentTime) +
		float64(globalContentionTime)/float64(shardedContentionTime) +
		float64(globalMixedTime)/float64(shardedMixedTime)) / 5

	fmt.Fprintf(w, "Average Speedup: %.2fx\n\n", overallSpeedup)

	fmt.Fprintf(w, "=== Why This Matters ===\n")
	fmt.Fprintf(w, "The 'many goroutines, few operations each' pattern is extremely\n")
	fmt.Fprintf(w, "common in real-world applications:\n\n")
	fmt.Fprintf(w, "• Web servers: thousands of concurrent requests\n")
	fmt.Fprintf(w, "• Each request: only a few cache/database operations\n")
	fmt.Fprintf(w, "• Microservices: high concurrency, low per-request work\n")
	fmt.Fprintf(w, "• Event processing: many workers, small tasks\n\n")
	fmt.Fprintf(w, "In these scenarios, lock contention becomes the dominant bottleneck.\n")
	fmt.Fprintf(w, "Sharding dramatically reduces contention by allowing concurrent\n")
	fmt.Fprintf(w, "operations on different shards to proceed in parallel.\n\n")
	fmt.Fprintf(w, "The global lock approach forces ALL operations to serialize,\n")
	fmt.Fprintf(w, "regardless of which data they're accessing.\n\n")

	fmt.Fprintf(w, "=== Key Insights ===\n")
	fmt.Fprintf(w, "1. **Sharding Overhead**: Single-threaded operations show the 'tax'\n")
	fmt.Fprintf(w, "   of sharding (hash computation, shard selection). This is the\n")
	fmt.Fprintf(w, "   price paid for concurrent benefits.\n\n")
	fmt.Fprintf(w, "2. **Insufficient Contention**: Simple concurrent insertions may not\n")
	fmt.Fprintf(w, "   create enough actual simultaneous access to justify sharding\n")
	fmt.Fprintf(w, "   overhead, especially with Go's scheduler.\n\n")
	fmt.Fprintf(w, "3. **Real Benefits Emerge**: With actual lock contention (forced\n")
	fmt.Fprintf(w, "   contention test) and mixed read/write patterns, sharding shows\n")
	fmt.Fprintf(w, "   significant improvements.\n\n")
	fmt.Fprintf(w, "4. **Architecture Trade-offs**: Sharding is most beneficial when:\n")
	fmt.Fprintf(w, "   • High concurrent access to shared data\n")
	fmt.Fprintf(w, "   • Mixed read/write operations (RWMutex benefits)\n")
	fmt.Fprintf(w, "   • Lock hold times are non-trivial\n")
	fmt.Fprintf(w, "   • Data access patterns allow for good shard distribution")

	fmt.Fprintf(w, `
        </pre>
        <p><a href="/" class="back">← Back to Home</a></p>
    </div>
</body>
</html>`)
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/insert", insertHandler)
	http.HandleFunc("/remove", removeHandler)
	http.HandleFunc("/random", randomHandler)
	http.HandleFunc("/clear", clearHandler)
	http.HandleFunc("/populate", populateHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/test", testHandler)
	http.HandleFunc("/compare", compareHandler)

	fmt.Println("=== Thread-Safe Generic HashSet Demo Server ===")
	fmt.Println("Features:")
	fmt.Println("- Type-safe with Go generics (HashSet[T comparable])")
	fmt.Println("- O(1) Insert, Remove, Contains, and Random access")
	fmt.Println("- Fine-grained locking with configurable sharding")
	fmt.Println("- Thread-safe concurrent operations")
	fmt.Println("- Swap-and-pop removal for efficiency")
	fmt.Println("- Zero boxing overhead with direct type storage")
	fmt.Println("")
	fmt.Println("Server starting on http://localhost:8080...")
	fmt.Println("Visit the URL to interact with the HashSet[string]!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
