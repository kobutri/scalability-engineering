package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	size := demoSet.Size()
	stats := demoSet.Stats()
	mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Thread-Safe Generic HashSet Demo</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
            margin: 20px; 
            background-color: #f8f9fa;
        }
        .container { 
            max-width: 1000px; 
            margin: 0 auto; 
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            padding-bottom: 20px;
            border-bottom: 2px solid #e9ecef;
        }
        .stats {
            display: flex;
            justify-content: space-around;
            margin: 20px 0;
            padding: 20px;
            background: #f8f9fa;
            border-radius: 6px;
        }
        .stat-item {
            text-align: center;
        }
        .stat-number {
            font-size: 2em;
            font-weight: bold;
            color: #007cba;
        }
        .stat-label {
            color: #6c757d;
            font-size: 0.9em;
        }
        .actions {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin: 30px 0;
        }
        .action-btn {
            display: block;
            padding: 12px 20px;
            background: #007cba;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            text-align: center;
            transition: background-color 0.2s;
            border: none;
            cursor: pointer;
            font-size: 14px;
        }
        .action-btn:hover { background: #005a8b; }
        .action-btn.secondary { background: #6c757d; }
        .action-btn.secondary:hover { background: #545b62; }
        .action-btn.danger { background: #dc3545; }
        .action-btn.danger:hover { background: #c82333; }
        .form-section {
            margin: 20px 0;
            padding: 20px;
            border: 1px solid #dee2e6;
            border-radius: 6px;
        }
        .form-section h3 {
            margin-top: 0;
            color: #495057;
        }
        input[type="text"], input[type="number"] {
            padding: 8px 12px;
            border: 1px solid #ced4da;
            border-radius: 4px;
            font-size: 14px;
        }
        .symbol {
            font-weight: bold;
            color: #007cba;
        }
        .symbol.success { color: #28a745; }
        .symbol.warning { color: #ffc107; }
        .symbol.danger { color: #dc3545; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><span class="symbol">&gt;</span> Thread-Safe Generic HashSet Demo</h1>
            <p>Type: <code>HashSet[string]</code> with fine-grained locking</p>
        </div>
        
        <div class="stats">
            <div class="stat-item">
                <div class="stat-number">%d</div>
                <div class="stat-label">Total Elements</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">%d</div>
                <div class="stat-label">Shards</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">%.2f</div>
                <div class="stat-label">Load Factor</div>
            </div>
        </div>

        <div class="actions">
            <a href="/random" class="action-btn">
                <span class="symbol">&clubs;</span> Get Random Element
            </a>
            <a href="/stats" class="action-btn secondary">
                <span class="symbol">&equiv;</span> Detailed Statistics
            </a>
            <a href="/test" class="action-btn secondary">
                <span class="symbol">&rArr;</span> Concurrency Test
            </a>
            <a href="/compare" class="action-btn secondary">
                <span class="symbol">&harr;</span> Performance Comparison
            </a>
            <a href="/clear" class="action-btn danger" onclick="return confirm('Clear all elements?')">
                <span class="symbol">&times;</span> Clear All
            </a>
        </div>

        <div class="form-section">
            <h3><span class="symbol success">+</span> Insert Elements</h3>
            <form action="/insert" method="post" style="display: flex; gap: 10px; align-items: center;">
                <input type="text" name="elements" placeholder="apple,banana,cherry" style="flex: 1;" required>
                <button type="submit" class="action-btn" style="margin: 0;">Add Elements</button>
            </form>
        </div>

        <div class="form-section">
            <h3><span class="symbol danger">-</span> Remove Elements</h3>
            <form action="/remove" method="post" style="display: flex; gap: 10px; align-items: center;">
                <input type="text" name="elements" placeholder="apple,banana" style="flex: 1;" required>
                <button type="submit" class="action-btn danger" style="margin: 0;">Remove Elements</button>
            </form>
        </div>

        <div class="form-section">
            <h3><span class="symbol">&infin;</span> Bulk Operations</h3>
            <form action="/populate" method="post" style="display: flex; gap: 10px; align-items: center;">
                <input type="number" name="count" placeholder="1000" min="1" max="100000" value="1000">
                <button type="submit" class="action-btn secondary" style="margin: 0;">Populate with Random Data</button>
            </form>
        </div>

        <div style="margin-top: 40px; padding-top: 20px; border-top: 1px solid #dee2e6; text-align: center; color: #6c757d;">
            <p><strong>Features:</strong> O(1) operations, thread-safe, sharded locking, zero-boxing generics</p>
        </div>
    </div>
</body>
</html>`, size, stats.ShardCount, stats.LoadFactor)
}

func insertHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var added int
	if r.Method == "POST" {
		// Handle form submission
		r.ParseForm()
		elementsStr := r.FormValue("elements")
		if elementsStr != "" {
			elements := strings.Split(elementsStr, ",")
			mu.Lock()
			for _, elem := range elements {
				elem = strings.TrimSpace(elem)
				if elem != "" && demoSet.Insert(elem) {
					added++
				}
			}
			mu.Unlock()
		}
	} else {
		// Handle single element from URL parameter (legacy)
		element := r.URL.Query().Get("element")
		if element == "" {
			element = fmt.Sprintf("item-%d", time.Now().UnixNano()%10000)
		}

		mu.Lock()
		if demoSet.Insert(element) {
			added = 1
		}
		mu.Unlock()
	}

	elapsed := time.Since(start)

	// Set redirect header before writing response
	w.Header().Set("Refresh", "2; url=/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Insert Result</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #d4edda; color: #155724; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>&#10003; Insert Complete</h2>
        <p>Added %d new elements</p>
        <div class="performance">
            Operation took: %v<br>
            Throughput: %.0f ops/sec<br>
            Redirecting in 2 seconds...
        </div>
    </div>
</body>
</html>`, added, elapsed, float64(added)/elapsed.Seconds())
}

func removeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var removed int
	if r.Method == "POST" {
		// Handle form submission
		r.ParseForm()
		elementsStr := r.FormValue("elements")
		if elementsStr != "" {
			elements := strings.Split(elementsStr, ",")
			mu.Lock()
			for _, elem := range elements {
				elem = strings.TrimSpace(elem)
				if elem != "" && demoSet.Remove(elem) {
					removed++
				}
			}
			mu.Unlock()
		}
	} else {
		// Handle single element from URL parameter or random removal
		element := r.URL.Query().Get("element")
		if element == "" {
			mu.RLock()
			if randomElem, ok := demoSet.RandomElement(); ok {
				element = randomElem
			}
			mu.RUnlock()
		}

		if element != "" {
			mu.Lock()
			if demoSet.Remove(element) {
				removed = 1
			}
			mu.Unlock()
		}
	}

	elapsed := time.Since(start)

	// Set redirect header before writing response
	w.Header().Set("Refresh", "2; url=/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Remove Result</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #f8d7da; color: #721c24; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>&#10003; Remove Complete</h2>
        <p>Removed %d elements</p>
        <div class="performance">
            Operation took: %v<br>
            Throughput: %.0f ops/sec<br>
            Redirecting in 2 seconds...
        </div>
    </div>
</body>
</html>`, removed, elapsed, float64(removed)/elapsed.Seconds())
}

func randomHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	mu.RLock()
	element, ok := demoSet.RandomElement()
	size := demoSet.Size()
	mu.RUnlock()

	elapsed := time.Since(start)

	w.Header().Set("Refresh", "3; url=/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if ok {
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Random Element</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #cce5ff; color: #004080; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .element { font-size: 1.5em; font-weight: bold; margin: 15px 0; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>&clubs; Random Element</h2>
        <div class="element">"%s"</div>
        <p>Selected from %d total elements</p>
        <div class="performance">
            Operation took: %v<br>
            Redirecting in 3 seconds...
        </div>
    </div>
</body>
</html>`, element, size, elapsed)
	} else {
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Random Element</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #f8d7da; color: #721c24; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>! No Elements</h2>
        <p>Set is empty - no random element available</p>
        <div class="performance">
            Operation took: %v<br>
            Redirecting in 3 seconds...
        </div>
    </div>
</body>
</html>`, elapsed)
	}
}

func clearHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	mu.Lock()
	oldSize := demoSet.Size()
	demoSet.Clear()
	mu.Unlock()

	elapsed := time.Since(start)

	w.Header().Set("Refresh", "2; url=/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Clear Complete</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #fff3cd; color: #856404; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>&#10003; Clear Complete</h2>
        <p>Removed all %d elements</p>
        <div class="performance">
            Operation took: %v<br>
            Throughput: %.0f ops/sec<br>
            Redirecting in 2 seconds...
        </div>
    </div>
</body>
</html>`, oldSize, elapsed, float64(oldSize)/elapsed.Seconds())
}

func populateHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	count := 1000 // default
	if r.Method == "POST" {
		r.ParseForm()
		if countStr := r.FormValue("count"); countStr != "" {
			if parsed, err := strconv.Atoi(countStr); err == nil && parsed > 0 && parsed <= 100000 {
				count = parsed
			}
		}
	}

	mu.Lock()
	added := 0
	for i := 0; i < count; i++ {
		element := fmt.Sprintf("random-%d-%d", time.Now().UnixNano(), i)
		if demoSet.Insert(element) {
			added++
		}
	}
	mu.Unlock()

	elapsed := time.Since(start)

	w.Header().Set("Refresh", "2; url=/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Populate Complete</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 20px; text-align: center; }
        .result { background: #d1ecf1; color: #0c5460; padding: 20px; border-radius: 8px; margin: 20px auto; max-width: 500px; }
        .performance { color: #6c757d; font-size: 0.9em; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="result">
        <h2>&infin; Populate Complete</h2>
        <p>Added %d new elements</p>
        <div class="performance">
            Operation took: %v<br>
            Throughput: %.0f ops/sec<br>
            Redirecting in 2 seconds...
        </div>
    </div>
</body>
</html>`, added, elapsed, float64(added)/elapsed.Seconds())
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	stats := demoSet.Stats()
	mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "persistence-demo":
			runPersistenceDemo()
			return
		case "show-json":
			showJSONFormat()
			return
		}
	}

	webServerDemo()
}

func webServerDemo() {
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
