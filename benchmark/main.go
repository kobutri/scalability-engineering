package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"shared"
)

var globalHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        600,
		MaxIdleConnsPerHost: 600,
		MaxConnsPerHost:     600,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 30 * time.Second,
}

// BenchmarkConfig holds the configuration for the benchmark
type BenchmarkConfig struct {
	MaxClients        int           `json:"max_clients"`
	ThrottleOPS       int           `json:"throttle_ops"`
	BootstrapURL      string        `json:"bootstrap_url"`
	WorkerCount       int           `json:"worker_count"`
	MetricsInterval   time.Duration `json:"metrics_interval"`
	LatencyBufferSize int           `json:"latency_buffer_size"`
}

// OperationResult represents the result of a connect/disconnect operation
type OperationResult struct {
	Success   bool
	Duration  time.Duration
	Error     error
	Timestamp time.Time
	Ignored   bool // If true, this operation should be ignored in metrics
}

// DataPoint represents a timestamped data point for charts
type DataPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// RingBuffer implements a fixed-size circular buffer for latency measurements
type RingBuffer struct {
	data     []time.Duration
	head     int
	size     int
	capacity int
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		data:     make([]time.Duration, capacity),
		head:     0,
		size:     0,
		capacity: capacity,
	}
}

// Add inserts a new value into the ring buffer (O(1))
func (rb *RingBuffer) Add(value time.Duration) {
	rb.data[rb.head] = value
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetAll returns all current values in the ring buffer for analysis
func (rb *RingBuffer) GetAll() []time.Duration {
	if rb.size == 0 {
		return nil
	}

	result := make([]time.Duration, rb.size)
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - rb.size + i + rb.capacity) % rb.capacity
		result[i] = rb.data[idx]
	}
	return result
}

// Metrics holds the benchmark metrics
type Metrics struct {
	mu                sync.RWMutex
	TotalOps          int64
	SuccessfulOps     int64
	ErrorOps          int64
	CurrentOPS        float64
	ExpectedOPS       float64
	LatencyBuffer     *RingBuffer
	LatencyP99        time.Duration
	StartTime         time.Time
	LastMetricsUpdate time.Time
	ErrorRate         float64

	// Historical data for charts (last 60 data points)
	ThroughputHistory []DataPoint
	LatencyHistory    []DataPoint
	ErrorHistory      []DataPoint

	// Per-second tracking
	opsThisSecond   int64
	lastSecondStart time.Time
}

// MetricsData holds metrics data without mutex for safe copying
type MetricsData struct {
	TotalOps          int64
	SuccessfulOps     int64
	ErrorOps          int64
	CurrentOPS        float64
	ExpectedOPS       float64
	LatencyP99        time.Duration
	StartTime         time.Time
	LastMetricsUpdate time.Time
	ErrorRate         float64

	// Historical data for charts
	ThroughputHistory []DataPoint
	LatencyHistory    []DataPoint
	ErrorHistory      []DataPoint
}

// BenchmarkTool is the main benchmark tool structure
type BenchmarkTool struct {
	config      BenchmarkConfig
	clients     *shared.HashMap[string, shared.ClientIdentity] // containerID -> client identity
	metrics     *Metrics
	workCounter int64      // Atomic counter for available work
	workCond    *sync.Cond // Condition variable to wake workers
	workMutex   sync.Mutex // Mutex for the condition variable
	stopChan    chan struct{}
	workerWG    sync.WaitGroup
	metricsWG   sync.WaitGroup
	running     bool
	runningMu   sync.RWMutex
	mu          sync.RWMutex
}

// NewBenchmarkTool creates a new benchmark tool
func NewBenchmarkTool(config BenchmarkConfig) *BenchmarkTool {
	clients := shared.NewHashMap[string, shared.ClientIdentity]()

	metrics := &Metrics{
		StartTime:         time.Now(),
		LastMetricsUpdate: time.Now(),
		lastSecondStart:   time.Now(),
		LatencyBuffer:     NewRingBuffer(config.LatencyBufferSize),
	}

	bt := &BenchmarkTool{
		config:      config,
		clients:     clients,
		metrics:     metrics,
		workCounter: 0,
		stopChan:    make(chan struct{}),
	}
	bt.workCond = sync.NewCond(&bt.workMutex)

	return bt
}

// generateClientIdentity generates a random client identity with fixed hostname
func (bt *BenchmarkTool) generateClientIdentity() shared.ClientIdentity {
	containerID := fmt.Sprintf("benchmark-client-%d", rand.Intn(1000000))
	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to get hostname: %v", err)
		hostname = "benchmark-host" // Fallback
	}
	return shared.ClientIdentity{
		Name:        fmt.Sprintf("benchmark-%s", containerID),
		ContainerID: containerID,
		Hostname:    hostname,
	}
}

// processOperationResult processes the result of an operation and updates metrics
func (bt *BenchmarkTool) processOperationResult(result OperationResult) {
	// If operation should be ignored, return without processing
	if result.Ignored {
		log.Printf("Operation ignored (pre-processing failed): %v", result.Error)
		return
	}

	bt.metrics.mu.Lock()
	defer bt.metrics.mu.Unlock()

	bt.metrics.TotalOps++
	bt.metrics.opsThisSecond++

	if result.Success {
		bt.metrics.SuccessfulOps++
	} else {
		bt.metrics.ErrorOps++
	}

	// Add latency to buffer
	bt.metrics.LatencyBuffer.Add(result.Duration)
}

// isRunning safely checks if the benchmark is running
func (bt *BenchmarkTool) isRunning() bool {
	bt.runningMu.RLock()
	defer bt.runningMu.RUnlock()
	return bt.running
}

// setRunning safely sets the running state
func (bt *BenchmarkTool) setRunning(running bool) {
	bt.runningMu.Lock()
	defer bt.runningMu.Unlock()
	bt.running = running
}

// worker runs the benchmark worker loop with atomic counter
func (bt *BenchmarkTool) worker(ctx context.Context) {
	defer bt.workerWG.Done()

	client := globalHTTPClient

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Try to get work using CAS
			current := atomic.LoadInt64(&bt.workCounter)
			if current > 0 && atomic.CompareAndSwapInt64(&bt.workCounter, current, current-1) {
				// Successfully claimed work, perform it
				bt.performWork(client)
			} else {
				// No work available, wait for signal
				bt.workMutex.Lock()
				bt.workCond.Wait()
				bt.workMutex.Unlock()
			}
		}
	}
}

// performWork executes one benchmark operation (connect/disconnect logic)
func (bt *BenchmarkTool) performWork(client *http.Client) {
	bt.mu.RLock()
	maxClients := bt.config.MaxClients
	bootstrapURL := bt.config.BootstrapURL
	bt.mu.RUnlock()

	currentSize := bt.clients.Size()

	if currentSize >= maxClients {
		// Remove a random client when at max capacity
		entries := bt.clients.Entries()
		if len(entries) > 0 {
			randomEntry := entries[rand.Intn(len(entries))]
			bt.clients.Remove(randomEntry.Key)

			// Disconnect from bootstrap
			result := bt.disconnectFromBootstrap(client, bootstrapURL, randomEntry.Key)
			bt.processOperationResult(result)
		}
	}

	// Always add a new client
	identity := bt.generateClientIdentity()
	bt.clients.Put(identity.ContainerID, identity)

	// Connect to bootstrap
	result := bt.connectToBootstrap(client, bootstrapURL, identity)
	bt.processOperationResult(result)
}

// throttleController manages the throttling by incrementing work counter
func (bt *BenchmarkTool) throttleController(ctx context.Context) {
	defer bt.workerWG.Done()

	// Start with current config
	bt.mu.RLock()
	currentThrottleOPS := bt.config.ThrottleOPS
	bt.mu.RUnlock()

	ticker := time.NewTicker(time.Second / time.Duration(currentThrottleOPS))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if throttle config has changed
			bt.mu.RLock()
			newThrottleOPS := bt.config.ThrottleOPS
			bt.mu.RUnlock()

			if newThrottleOPS != currentThrottleOPS {
				// Restart ticker with new interval
				ticker.Stop()
				currentThrottleOPS = newThrottleOPS
				ticker = time.NewTicker(time.Second / time.Duration(currentThrottleOPS))
				continue
			}

			if !bt.isRunning() {
				continue
			}

			// Increment work counter and wake one worker
			atomic.AddInt64(&bt.workCounter, 1)
			bt.workCond.Signal()
		}
	}
}

// metricsUpdater updates the metrics at regular intervals
func (bt *BenchmarkTool) metricsUpdater(ctx context.Context) {
	defer bt.metricsWG.Done()

	ticker := time.NewTicker(bt.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bt.updateMetrics()
		}
	}
}

// updateMetrics calculates and updates current metrics
func (bt *BenchmarkTool) updateMetrics() {
	bt.metrics.mu.Lock()
	defer bt.metrics.mu.Unlock()

	now := time.Now()

	// Calculate current OPS
	if now.Sub(bt.metrics.lastSecondStart) >= time.Second {
		elapsedSeconds := now.Sub(bt.metrics.lastSecondStart).Seconds()
		bt.metrics.CurrentOPS = float64(bt.metrics.opsThisSecond) / elapsedSeconds
		bt.metrics.opsThisSecond = 0
		bt.metrics.lastSecondStart = now
	}

	// Calculate expected OPS
	bt.mu.RLock()
	bt.metrics.ExpectedOPS = float64(bt.config.ThrottleOPS)
	bt.mu.RUnlock()

	// Calculate error rate
	if bt.metrics.TotalOps > 0 {
		bt.metrics.ErrorRate = float64(bt.metrics.ErrorOps) / float64(bt.metrics.TotalOps) * 100
	}

	// Calculate P99 latency
	latencies := bt.metrics.LatencyBuffer.GetAll()
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		p99Index := int(float64(len(latencies)) * 0.99)
		if p99Index >= len(latencies) {
			p99Index = len(latencies) - 1
		}
		bt.metrics.LatencyP99 = latencies[p99Index]
	}

	// Store historical data points for charts (keep last 60 points)
	const maxHistoryPoints = 60

	// Add throughput data point
	bt.metrics.ThroughputHistory = append(bt.metrics.ThroughputHistory, DataPoint{
		Value:     bt.metrics.CurrentOPS,
		Timestamp: now,
	})
	if len(bt.metrics.ThroughputHistory) > maxHistoryPoints {
		bt.metrics.ThroughputHistory = bt.metrics.ThroughputHistory[1:]
	}

	// Add latency data point
	bt.metrics.LatencyHistory = append(bt.metrics.LatencyHistory, DataPoint{
		Value:     float64(bt.metrics.LatencyP99.Milliseconds()),
		Timestamp: now,
	})
	if len(bt.metrics.LatencyHistory) > maxHistoryPoints {
		bt.metrics.LatencyHistory = bt.metrics.LatencyHistory[1:]
	}

	// Add error rate data point
	bt.metrics.ErrorHistory = append(bt.metrics.ErrorHistory, DataPoint{
		Value:     bt.metrics.ErrorRate,
		Timestamp: now,
	})
	if len(bt.metrics.ErrorHistory) > maxHistoryPoints {
		bt.metrics.ErrorHistory = bt.metrics.ErrorHistory[1:]
	}

	bt.metrics.LastMetricsUpdate = now
}

// connectToBootstrap performs a connect operation to the bootstrap server
func (bt *BenchmarkTool) connectToBootstrap(client *http.Client, bootstrapURL string, identity shared.ClientIdentity) OperationResult {
	// Pre-request processing - if this fails, ignore the operation
	identityJSON, err := json.Marshal(identity)
	if err != nil {
		return OperationResult{
			Success:   false,
			Duration:  0,
			Error:     fmt.Errorf("failed to marshal identity: %w", err),
			Timestamp: time.Now(),
			Ignored:   true, // Ignore this operation in metrics
		}
	}

	// Start timing only for the HTTP request
	start := time.Now()
	url := fmt.Sprintf("%s/connect", bootstrapURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(identityJSON))
	if err != nil {
		return OperationResult{
			Success:   false,
			Duration:  time.Since(start),
			Error:     fmt.Errorf("failed to create request: %w", err),
			Timestamp: start,
			Ignored:   true,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		requestDuration := time.Since(start)
		return OperationResult{
			Success:   false,
			Duration:  requestDuration,
			Error:     fmt.Errorf("connect request failed: %w", err),
			Timestamp: start,
			Ignored:   false,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		requestDuration := time.Since(start)
		return OperationResult{
			Success:   false,
			Duration:  requestDuration,
			Error:     fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body)),
			Timestamp: start,
			Ignored:   false,
		}
	}

	// Fully read the response body to ensure connection reuse
	io.Copy(io.Discard, resp.Body)

	// Calculate total duration including body reading
	requestDuration := time.Since(start)

	return OperationResult{
		Success:   true,
		Duration:  requestDuration,
		Timestamp: start,
		Ignored:   false,
	}
}

// disconnectFromBootstrap performs a disconnect operation to the bootstrap server
func (bt *BenchmarkTool) disconnectFromBootstrap(client *http.Client, bootstrapURL string, containerID string) OperationResult {
	// Start timing only for the HTTP request
	start := time.Now()
	url := fmt.Sprintf("%s/delete", bootstrapURL)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(containerID))
	if err != nil {
		return OperationResult{
			Success:   false,
			Duration:  time.Since(start),
			Error:     fmt.Errorf("failed to create request: %w", err),
			Timestamp: start,
			Ignored:   true,
		}
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		requestDuration := time.Since(start)
		return OperationResult{
			Success:   false,
			Duration:  requestDuration,
			Error:     fmt.Errorf("disconnect request failed: %w", err),
			Timestamp: start,
			Ignored:   false,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		requestDuration := time.Since(start)
		return OperationResult{
			Success:   false,
			Duration:  requestDuration,
			Error:     fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body)),
			Timestamp: start,
			Ignored:   false,
		}
	}

	// Fully read the response body to ensure connection reuse
	io.Copy(io.Discard, resp.Body)

	// Calculate total duration including body reading
	requestDuration := time.Since(start)

	return OperationResult{
		Success:   true,
		Duration:  requestDuration,
		Timestamp: start,
		Ignored:   false,
	}
}

// Start starts the benchmark tool
func (bt *BenchmarkTool) Start() error {
	if bt.isRunning() {
		return fmt.Errorf("benchmark tool is already running")
	}

	bt.setRunning(true)

	ctx, cancel := context.WithCancel(context.Background())

	// Use configured worker count (no auto-scaling needed with this simpler architecture)
	workerCount := bt.config.WorkerCount

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		bt.workerWG.Add(1)
		go bt.worker(ctx)
	}

	// Start throttle controller
	bt.workerWG.Add(1)
	go bt.throttleController(ctx)

	// Start metrics updater
	bt.metricsWG.Add(1)
	go bt.metricsUpdater(ctx)

	log.Printf("Benchmark tool started with %d workers, throttle: %d ops/s, max clients: %d",
		workerCount, bt.config.ThrottleOPS, bt.config.MaxClients)

	// Wait for stop signal
	<-bt.stopChan

	// Cancel context to stop all goroutines
	cancel()

	// Wait for all workers to finish
	bt.workerWG.Wait()
	bt.metricsWG.Wait()

	bt.setRunning(false)
	log.Println("Benchmark tool stopped")

	return nil
}

// Stop stops the benchmark tool
func (bt *BenchmarkTool) Stop() {
	if !bt.isRunning() {
		return
	}

	close(bt.stopChan)

	// Wake up all workers that might be waiting
	bt.workCond.Broadcast()
}

// UpdateConfig updates the benchmark configuration
func (bt *BenchmarkTool) UpdateConfig(newConfig BenchmarkConfig) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	bt.config = newConfig

	// Update metrics expected OPS
	bt.metrics.mu.Lock()
	bt.metrics.ExpectedOPS = float64(newConfig.ThrottleOPS)
	bt.metrics.mu.Unlock()
}

// GetMetrics returns current metrics as a safely copied structure without mutex
func (bt *BenchmarkTool) GetMetrics() MetricsData {
	bt.metrics.mu.RLock()
	defer bt.metrics.mu.RUnlock()

	// Copy historical data arrays
	throughputHistory := make([]DataPoint, len(bt.metrics.ThroughputHistory))
	copy(throughputHistory, bt.metrics.ThroughputHistory)

	latencyHistory := make([]DataPoint, len(bt.metrics.LatencyHistory))
	copy(latencyHistory, bt.metrics.LatencyHistory)

	errorHistory := make([]DataPoint, len(bt.metrics.ErrorHistory))
	copy(errorHistory, bt.metrics.ErrorHistory)

	// Return a copy of metrics data without the mutex
	return MetricsData{
		TotalOps:          bt.metrics.TotalOps,
		SuccessfulOps:     bt.metrics.SuccessfulOps,
		ErrorOps:          bt.metrics.ErrorOps,
		CurrentOPS:        bt.metrics.CurrentOPS,
		ExpectedOPS:       bt.metrics.ExpectedOPS,
		LatencyP99:        bt.metrics.LatencyP99,
		StartTime:         bt.metrics.StartTime,
		LastMetricsUpdate: bt.metrics.LastMetricsUpdate,
		ErrorRate:         bt.metrics.ErrorRate,
		ThroughputHistory: throughputHistory,
		LatencyHistory:    latencyHistory,
		ErrorHistory:      errorHistory,
	}
}

// GetConfig returns current configuration
func (bt *BenchmarkTool) GetConfig() BenchmarkConfig {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	return bt.config
}

// GetClientData returns client data for display
func (bt *BenchmarkTool) GetClientData() ClientData {
	return ClientData{
		ActiveClients: bt.clients.Size(),
	}
}

// ClientData represents client data for display
type ClientData struct {
	ActiveClients int `json:"active_clients"`
}

// main function
func main() {
	// Parse configuration from environment variables
	config := BenchmarkConfig{
		MaxClients:        getEnvInt("MAX_CLIENTS", 100),
		ThrottleOPS:       getEnvInt("THROTTLE_OPS", 10),
		BootstrapURL:      getEnvString("BOOTSTRAP_URL", "http://localhost:8080"),
		WorkerCount:       getEnvInt("WORKER_COUNT", 100),
		MetricsInterval:   getEnvDuration("METRICS_INTERVAL", 1*time.Second),
		LatencyBufferSize: getEnvInt("LATENCY_BUFFER_SIZE", 1000),
	}

	// Create benchmark tool
	benchmarkTool := NewBenchmarkTool(config)

	// Set up HTTP handlers for the dashboard
	setupHTTPHandlers(benchmarkTool)

	// Start HTTP server
	port := getEnvString("PORT", "8090")
	log.Printf("Starting benchmark dashboard on port %s", port)
	log.Printf("Dashboard available at http://localhost:%s", port)
	log.Printf("Benchmark tool is in stopped state - use the dashboard to start it")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Utility functions
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
