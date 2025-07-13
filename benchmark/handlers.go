package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"
)

// setupHTTPHandlers sets up all HTTP handlers for the benchmark tool
func setupHTTPHandlers(bt *BenchmarkTool) {
	// Dashboard endpoints
	http.HandleFunc("/", dashboardHandler(bt))
	http.HandleFunc("/dashboard", dashboardHandler(bt))
	http.HandleFunc("/metrics", metricsHandler(bt))
	http.HandleFunc("/config", configHandler(bt))
	http.HandleFunc("/clients", clientsHandler(bt))
	http.HandleFunc("/alive", aliveHandler())

	// Control endpoints
	http.HandleFunc("/start", startHandler(bt))
	http.HandleFunc("/stop", stopHandler(bt))
	http.HandleFunc("/update-config", updateConfigHandler(bt))

	// JSON data endpoints for charts
	http.HandleFunc("/api/chart-data", chartDataHandler(bt))

	// Component refresh endpoints
	http.HandleFunc("/refresh/metrics", refreshMetricsHandler(bt))
	http.HandleFunc("/refresh/config", refreshConfigHandler(bt))
	http.HandleFunc("/refresh/clients", refreshClientsHandler(bt))
	http.HandleFunc("/refresh/control", refreshControlHandler(bt))
}

// dashboardHandler serves the main dashboard page
func dashboardHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBenchmarkData(bt)
		BenchmarkDashboard(&data).Render(r.Context(), w)
	}
}

// metricsHandler returns current metrics as JSON
func metricsHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := bt.GetMetrics()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	}
}

// configHandler returns current configuration as JSON
func configHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config := bt.GetConfig()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)
	}
}

// clientsHandler returns client data as JSON
func clientsHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := bt.GetClientData()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

// aliveHandler returns a simple health check response
func aliveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// startHandler starts the benchmark tool
func startHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Start the benchmark tool in a goroutine
		go func() {
			if err := bt.Start(); err != nil {
				fmt.Printf("Error starting benchmark tool: %v\n", err)
			}
		}()

		// Return updated control panel
		data := getBenchmarkData(bt)
		BenchmarkControlPanel(&data).Render(r.Context(), w)
	}
}

// stopHandler stops the benchmark tool
func stopHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		bt.Stop()

		// Return updated control panel
		data := getBenchmarkData(bt)
		BenchmarkControlPanel(&data).Render(r.Context(), w)
	}
}

// updateConfigHandler updates the benchmark configuration
func updateConfigHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		currentConfig := bt.GetConfig()

		// Parse max clients
		if maxClients := r.FormValue("max_clients"); maxClients != "" {
			if val, err := strconv.Atoi(maxClients); err == nil && val > 0 {
				currentConfig.MaxClients = val
			}
		}

		// Parse throttle OPS
		if throttleOPS := r.FormValue("throttle_ops"); throttleOPS != "" {
			if val, err := strconv.Atoi(throttleOPS); err == nil && val > 0 {
				currentConfig.ThrottleOPS = val
			}
		}

		// Parse worker count
		if workerCount := r.FormValue("worker_count"); workerCount != "" {
			if val, err := strconv.Atoi(workerCount); err == nil && val > 0 {
				currentConfig.WorkerCount = val
			}
		}

		// Parse metrics interval
		if metricsInterval := r.FormValue("metrics_interval"); metricsInterval != "" {
			if val, err := time.ParseDuration(metricsInterval); err == nil {
				currentConfig.MetricsInterval = val
			}
		}

		// Parse latency buffer size
		if latencyBufferSize := r.FormValue("latency_buffer_size"); latencyBufferSize != "" {
			if val, err := strconv.Atoi(latencyBufferSize); err == nil && val > 0 {
				currentConfig.LatencyBufferSize = val
			}
		}

		// Update configuration
		bt.UpdateConfig(currentConfig)

		// Return updated configuration panel
		data := getBenchmarkData(bt)
		BenchmarkConfigPanel(&data).Render(r.Context(), w)
	}
}

// ChartDataResponse represents all chart data in a single JSON response
type ChartDataResponse struct {
	Throughput ThroughputGraphData `json:"throughput"`
	Latency    LatencyGraphData    `json:"latency"`
	Error      ErrorGraphData      `json:"error"`
	Timestamp  string              `json:"timestamp"`
}

// chartDataHandler returns all chart data as JSON
func chartDataHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := bt.GetMetrics()
		timestamp := time.Now().Format("15:04:05")

		response := ChartDataResponse{
			Throughput: ThroughputGraphData{
				History:       metrics.ThroughputHistory,
				ExpectedValue: metrics.ExpectedOPS,
				Timestamp:     timestamp,
			},
			Latency: LatencyGraphData{
				History:   metrics.LatencyHistory,
				Timestamp: timestamp,
			},
			Error: ErrorGraphData{
				History:   metrics.ErrorHistory,
				Timestamp: timestamp,
			},
			Timestamp: timestamp,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// refreshMetricsHandler refreshes the metrics panel
func refreshMetricsHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBenchmarkData(bt)
		BenchmarkMetricsPanel(&data).Render(r.Context(), w)
	}
}

// refreshConfigHandler refreshes the configuration panel
func refreshConfigHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBenchmarkData(bt)
		BenchmarkConfigPanel(&data).Render(r.Context(), w)
	}
}

// refreshClientsHandler refreshes the clients panel
func refreshClientsHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBenchmarkData(bt)
		BenchmarkClientsPanel(&data).Render(r.Context(), w)
	}
}

// refreshControlHandler refreshes the control panel
func refreshControlHandler(bt *BenchmarkTool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := getBenchmarkData(bt)
		BenchmarkControlPanel(&data).Render(r.Context(), w)
	}
}

// BenchmarkData represents all data needed for the benchmark dashboard
type BenchmarkData struct {
	ServiceName     string
	UptimeFormatted string
	CurrentTime     string
	Running         bool
	Config          BenchmarkConfig
	Metrics         MetricsData
	ClientData      ClientData
}

// ThroughputGraphData represents data for the throughput graph
type ThroughputGraphData struct {
	History       []DataPoint `json:"history"`
	ExpectedValue float64     `json:"expected_value"`
	Timestamp     string      `json:"timestamp"`
}

// LatencyGraphData represents data for the latency graph
type LatencyGraphData struct {
	History   []DataPoint `json:"history"`
	Timestamp string      `json:"timestamp"`
}

// ErrorGraphData represents data for the error rate graph
type ErrorGraphData struct {
	History   []DataPoint `json:"history"`
	Timestamp string      `json:"timestamp"`
}

// getBenchmarkData creates a BenchmarkData structure with current state
func getBenchmarkData(bt *BenchmarkTool) BenchmarkData {
	metrics := bt.GetMetrics()
	config := bt.GetConfig()
	clientData := bt.GetClientData()

	uptime := time.Since(metrics.StartTime)

	return BenchmarkData{
		ServiceName:     "Bootstrap Benchmark Tool",
		UptimeFormatted: formatDuration(uptime),
		CurrentTime:     time.Now().Format("15:04:05"),
		Running:         bt.isRunning(),
		Config:          config,
		Metrics:         metrics,
		ClientData:      clientData,
	}
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
