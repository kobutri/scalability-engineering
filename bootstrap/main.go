package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Global HashSet instance for the demo
var demoSet *HashSet[string]
var mu sync.RWMutex

func init() {
	// Initialize with some demo data
	demoSet = NewHashSet[string]()
	demoSet.InsertAll("apple", "banana", "cherry", "date", "elderberry", "fig", "grape")
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
            <p>Type: <code>HashSet[string]</code> thread-safe implementation</p>
        </div>
        
        <div class="stats">
            <div class="stat-item">
                <div class="stat-number">%d</div>
                <div class="stat-label">Total Elements</div>
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
                <span class="symbol">&harr;</span> Performance Analysis
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
            <p><strong>Features:</strong> O(1) operations, thread-safe, zero-boxing generics</p>
        </div>
    </div>
</body>
</html>`, size, stats.LoadFactor)
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
        <h1>Detailed Statistics (HashSet[string])</h1>
        
        <h3>Overall Stats</h3>
        <p><strong>Total Elements:</strong> %d</p>
        <p><strong>Load Factor:</strong> %.2f</p>
        
        <h3>Implementation Details</h3>
        <p>This HashSet provides thread-safe operations with O(1) performance characteristics.</p>`,
		stats.TotalElements, stats.LoadFactor)

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
	testSet := NewHashSet[string]()
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
    <title>Performance Analysis</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .container { max-width: 1000px; margin: 0 auto; }
        pre { background: #f8f8f8; padding: 10px; border-radius: 4px; overflow-x: auto; }
        .back { background: #007cba; color: white; padding: 8px 16px; text-decoration: none; border-radius: 4px; }
        .result { background: #f0f0f0; padding: 15px; border-radius: 5px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Performance Analysis: HashSet</h1>
        <pre>`)

	fmt.Fprintf(w, "Running performance analysis of HashSet...\n\n")

	// Test 1: Single-threaded operations
	fmt.Fprintf(w, "=== Test 1: Single-threaded Operations ===\n")

	// Create fresh instance for testing
	testSet := NewHashSet[string]()

	// Test insertions
	numOps := 10000
	fmt.Fprintf(w, "Inserting %d elements:\n", numOps)

	start := time.Now()
	for i := 0; i < numOps; i++ {
		testSet.Insert(fmt.Sprintf("item-%d", i))
	}
	insertTime := time.Since(start)

	fmt.Fprintf(w, "  Insert time: %v (%.0f ops/sec)\n", insertTime, float64(numOps)/insertTime.Seconds())

	// Test lookups
	fmt.Fprintf(w, "Performing %d lookups:\n", numOps)

	start = time.Now()
	for i := 0; i < numOps; i++ {
		testSet.Contains(fmt.Sprintf("item-%d", i))
	}
	lookupTime := time.Since(start)

	fmt.Fprintf(w, "  Lookup time: %v (%.0f ops/sec)\n\n", lookupTime, float64(numOps)/lookupTime.Seconds())

	// Test 2: Concurrent operations
	fmt.Fprintf(w, "=== Test 2: Concurrent Operations ===\n")
	fmt.Fprintf(w, "Testing with multiple goroutines for concurrent access.\n\n")

	numGoroutines := 1000
	opsPerGoroutine := 10
	fmt.Fprintf(w, "Concurrent insertions (%d goroutines, %d ops each = %d total):\n",
		numGoroutines, opsPerGoroutine, numGoroutines*opsPerGoroutine)

	// Fresh instance for concurrent test
	testSet = NewHashSet[string]()

	var wg sync.WaitGroup

	// Test concurrent operations
	start = time.Now()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				testSet.Insert(fmt.Sprintf("concurrent-%d-%d", id, j))
			}
		}(i)
	}
	wg.Wait()
	concurrentTime := time.Since(start)

	totalOps := numGoroutines * opsPerGoroutine
	fmt.Fprintf(w, "  Concurrent time: %v (%.0f ops/sec)\n\n", concurrentTime, float64(totalOps)/concurrentTime.Seconds())

	// Test 3: Mixed operations
	fmt.Fprintf(w, "=== Test 3: Mixed Operations ===\n")
	fmt.Fprintf(w, "Testing mixed read/write operations under load.\n\n")

	// Pre-populate set with base data
	testSet = NewHashSet[string]()
	for i := 0; i < 1000; i++ {
		testSet.Insert(fmt.Sprintf("base-%d", i))
	}

	numMixedGoroutines := 500
	fmt.Fprintf(w, "Running %d goroutines, each doing mixed operations:\n", numMixedGoroutines)

	start = time.Now()
	for i := 0; i < numMixedGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine does mixed operations
			// 1-2 writes
			testSet.Insert(fmt.Sprintf("write-%d-1", id))
			if id%2 == 0 {
				testSet.Insert(fmt.Sprintf("write-%d-2", id))
			}

			// 3-5 reads
			testSet.Contains(fmt.Sprintf("base-%d", id%1000))
			testSet.Contains(fmt.Sprintf("base-%d", (id*7)%1000))
			testSet.Contains(fmt.Sprintf("base-%d", (id*13)%1000))
			if id%3 == 0 {
				testSet.Contains(fmt.Sprintf("base-%d", (id*17)%1000))
				testSet.Contains(fmt.Sprintf("base-%d", (id*19)%1000))
			}

			// 1-2 random accesses
			testSet.RandomElement()
			if id%4 == 0 {
				testSet.RandomElement()
			}
		}(i)
	}
	wg.Wait()
	mixedTime := time.Since(start)

	// Estimate total operations (approximate)
	avgOpsPerGoroutine := 1.5 + 3.7 + 1.25 // writes + reads + random
	totalMixedOps := int(float64(numMixedGoroutines) * avgOpsPerGoroutine)

	fmt.Fprintf(w, "  Mixed operations time: %v (%.0f ops/sec)\n\n", mixedTime, float64(totalMixedOps)/mixedTime.Seconds())

	// Summary
	fmt.Fprintf(w, "=== Summary ===\n")
	fmt.Fprintf(w, "Single-threaded Insert:  %.0f ops/sec\n", float64(numOps)/insertTime.Seconds())
	fmt.Fprintf(w, "Single-threaded Lookup:  %.0f ops/sec\n", float64(numOps)/lookupTime.Seconds())
	fmt.Fprintf(w, "Concurrent Insert:       %.0f ops/sec\n", float64(totalOps)/concurrentTime.Seconds())
	fmt.Fprintf(w, "Mixed Operations:        %.0f ops/sec\n\n", float64(totalMixedOps)/mixedTime.Seconds())

	fmt.Fprintf(w, "=== Analysis ===\n")
	fmt.Fprintf(w, "This HashSet provides thread-safe operations with consistent performance.\n")
	fmt.Fprintf(w, "Characteristics:\n\n")
	fmt.Fprintf(w, "• **Thread Safety**: Concurrent access protection\n")
	fmt.Fprintf(w, "• **Memory Efficiency**: Optimized memory usage\n")
	fmt.Fprintf(w, "• **Performance**: O(1) average case operations\n")
	fmt.Fprintf(w, "• **Concurrent Reads**: Multiple readers can operate simultaneously\n")
	fmt.Fprintf(w, "• **Consistency**: Thread-safe operations maintain data integrity\n\n")
	fmt.Fprintf(w, "Best suited for:\n")
	fmt.Fprintf(w, "• General purpose concurrent set operations\n")
	fmt.Fprintf(w, "• Multi-threaded applications\n")
	fmt.Fprintf(w, "• Applications requiring thread-safe data structures\n")
	fmt.Fprintf(w, "• Systems needing consistent performance")

	fmt.Fprintf(w, `
        </pre>
        <p><a href="/" class="back">← Back to Home</a></p>
    </div>
</body>
</html>`)
}

func main() {
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
	fmt.Println("- Thread-safe concurrent operations")
	fmt.Println("- Swap-and-pop removal for efficiency")
	fmt.Println("- Zero boxing overhead with direct type storage")
	fmt.Println("")
	fmt.Println("Server starting on http://localhost:8080...")
	fmt.Println("Visit the URL to interact with the HashSet[string]!")

	log.Fatal(http.ListenAndServe(":8080", nil))
}
