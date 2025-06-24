package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	defaultTimeout             = 500 * time.Millisecond
	defaultHealthCheckInterval = 1 * time.Second
	defaultCleanupInterval     = 1 * time.Second
	defaultMaxAge              = 10 * time.Second
	defaultSubsetSize          = 5
)

type Server struct {
	hashSet       *HashSet[string]
	priorityQueue *PriorityQueue[int64, string]
	startTime     time.Time

	// Configuration
	timeout    time.Duration
	maxAge     time.Duration
	subsetSize int

	// Worker control
	workersRunning bool
	stopWorkers    chan struct{}
	workersMutex   sync.RWMutex
}

func NewServer(dataPath string) *Server {
	// Create HashSet with persistence
	persistConfig := PersistenceConfig{
		Enabled:          true,
		FilePath:         dataPath,
		SnapshotInterval: 30 * time.Second,
		MaxRetries:       3,
	}

	hashSet := NewHashSetWithPersistence[string](persistConfig)

	// Try to load existing data
	if _, err := os.Stat(dataPath); err == nil {
		if err := hashSet.LoadFromDisk(dataPath); err != nil {
			log.Printf("Failed to load existing data: %v", err)
		} else {
			log.Printf("Loaded existing data from %s", dataPath)
		}
	}

	return &Server{
		hashSet:        hashSet,
		priorityQueue:  NewPriorityQueue[int64, string](),
		startTime:      time.Now(),
		timeout:        defaultTimeout,
		maxAge:         defaultMaxAge,
		subsetSize:     defaultSubsetSize,
		workersRunning: false,
		stopWorkers:    make(chan struct{}),
	}
}

func (s *Server) getCurrentTimestamp() int64 {
	return time.Since(s.startTime).Microseconds()
}

func (s *Server) healthCheckWorker() {
	ticker := time.NewTicker(defaultHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopWorkers:
			return
		case <-ticker.C:
			// Get a random element from the hashset
			name, ok := s.hashSet.RandomElement()
			if !ok {
				continue
			}

			// Check if it's alive in a separate goroutine to avoid blocking
			go func(name string) {
				if s.isAlive(name) {
					// Add to priority queue with current timestamp
					timestamp := s.getCurrentTimestamp()
					s.priorityQueue.Insert(timestamp, name)
				}
			}(name)
		}
	}
}

func (s *Server) cleanupWorker() {
	ticker := time.NewTicker(defaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopWorkers:
			return
		case <-ticker.C:
			s.workersMutex.RLock()
			maxAgeMicros := s.maxAge.Microseconds()
			s.workersMutex.RUnlock()

			currentTime := s.getCurrentTimestamp()
			cutoffTime := currentTime - maxAgeMicros

			// Remove old entries from priority queue
			for {
				priority, _, ok := s.priorityQueue.Peek()
				if !ok || priority >= cutoffTime {
					break
				}
				s.priorityQueue.ExtractMin()
			}
		}
	}
}

func (s *Server) isAlive(name string) bool {
	url := fmt.Sprintf("http://%s/alive", name)

	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (s *Server) getRandomSubset() []string {
	return s.priorityQueue.GetRandomSubset(s.subsetSize)
}

func (s *Server) connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	name := string(body)
	if name == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	// Insert into hashset
	s.hashSet.Insert(name)

	// Add to priority queue with current timestamp
	timestamp := s.getCurrentTimestamp()
	s.priorityQueue.Insert(timestamp, name)

	// Get random subset to return
	subset := s.getRandomSubset()

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subset)
}

func (s *Server) deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	name := string(body)
	if name == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	// Remove from hashset
	s.hashSet.Remove(name)

	// Remove from priority queue
	s.priorityQueue.Remove(name)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"hashset_size":        s.hashSet.Size(),
		"priority_queue_size": s.priorityQueue.Size(),
		"uptime_seconds":      time.Since(s.startTime).Seconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) formatUptime(duration time.Duration) string {
	if duration.Hours() >= 24 {
		days := int(duration.Hours()) / 24
		hours := int(duration.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	} else if duration.Hours() >= 1 {
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if duration.Minutes() >= 1 {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%.1fs", duration.Seconds())
}

func (s *Server) startWorkersFunc() {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	if !s.workersRunning {
		s.stopWorkers = make(chan struct{})
		s.workersRunning = true
		go s.healthCheckWorker()
		go s.cleanupWorker()
	}
}

func (s *Server) stopWorkersFunc() {
	s.workersMutex.Lock()
	defer s.workersMutex.Unlock()

	if s.workersRunning {
		close(s.stopWorkers)
		s.workersRunning = false
	}
}

func (s *Server) getQueueItems() []QueueItem {
	priorities, values := s.priorityQueue.ToSlices()
	items := make([]QueueItem, len(values))

	for i, value := range values {
		priority := priorities[i]
		timestamp := time.Duration(priority) * time.Microsecond
		actualTime := s.startTime.Add(timestamp)
		age := time.Since(actualTime)

		items[i] = QueueItem{
			Name:      value,
			Priority:  priority,
			Timestamp: actualTime.Format("15:04:05"),
			Age:       s.formatUptime(age),
		}
	}

	return items
}

func (s *Server) getStatusData() StatusData {
	uptime := time.Since(s.startTime)
	names := s.hashSet.ToSlice()
	queueItems := s.getQueueItems()

	s.workersMutex.RLock()
	workersRunning := s.workersRunning
	timeout := s.timeout
	maxAge := s.maxAge
	subsetSize := s.subsetSize
	s.workersMutex.RUnlock()

	return StatusData{
		HashSetSize:       s.hashSet.Size(),
		PriorityQueueSize: s.priorityQueue.Size(),
		UptimeSeconds:     uptime.Seconds(),
		UptimeFormatted:   s.formatUptime(uptime),
		CurrentTime:       time.Now().Format("15:04:05"),
		Names:             names,
		QueueItems:        queueItems,
		WorkersRunning:    workersRunning,
		Config: ServerConfig{
			Timeout:    timeout.String(),
			MaxAge:     maxAge.String(),
			SubsetSize: subsetSize,
		},
	}
}

func (s *Server) statusPageHandler(w http.ResponseWriter, r *http.Request) {
	data := s.getStatusData()
	statusPage(data).Render(r.Context(), w)
}

func (s *Server) statusDataHandler(w http.ResponseWriter, r *http.Request) {
	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) updateConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Update timeout
	if timeoutStr := r.FormValue("timeout"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			s.workersMutex.Lock()
			s.timeout = timeout
			s.workersMutex.Unlock()
		}
	}

	// Update max age
	if maxAgeStr := r.FormValue("maxAge"); maxAgeStr != "" {
		if maxAge, err := time.ParseDuration(maxAgeStr); err == nil {
			s.workersMutex.Lock()
			s.maxAge = maxAge
			s.workersMutex.Unlock()
		}
	}

	// Update subset size
	if subsetSizeStr := r.FormValue("subsetSize"); subsetSizeStr != "" {
		if subsetSize, err := strconv.Atoi(subsetSizeStr); err == nil && subsetSize > 0 {
			s.workersMutex.Lock()
			s.subsetSize = subsetSize
			s.workersMutex.Unlock()
		}
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) startWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.startWorkersFunc()
	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) stopWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.stopWorkersFunc()
	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) addNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name != "" {
		s.hashSet.Insert(name)
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) removeNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var request map[string]string
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if name, ok := request["name"]; ok && name != "" {
		s.hashSet.Remove(name)
		s.priorityQueue.Remove(name)
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) clearQueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.priorityQueue.Clear()
	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) removeFromQueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var request map[string]string
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if name, ok := request["name"]; ok && name != "" {
		s.priorityQueue.Remove(name)
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func getEnvInt(key string, defaultValue int) int {
	if str := os.Getenv(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if str := os.Getenv(key); str != "" {
		if val, err := time.ParseDuration(str); err == nil {
			return val
		}
	}
	return defaultValue
}

func getDataPath() string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "../data"
	}
	return fmt.Sprintf("%s/names.json", dataDir)
}

func main() {
	// Get data path from environment variable
	dataPath := getDataPath()
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "../data"
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	server := NewServer(dataPath)

	// Configure from environment variables
	server.timeout = getEnvDuration("HEALTH_TIMEOUT", defaultTimeout)
	server.maxAge = getEnvDuration("MAX_AGE", defaultMaxAge)
	server.subsetSize = getEnvInt("SUBSET_SIZE", defaultSubsetSize)

	// Start background workers
	server.startWorkersFunc()

	// Setup HTTP routes
	http.HandleFunc("/connect", server.connectHandler)
	http.HandleFunc("/delete", server.deleteHandler)
	http.HandleFunc("/", server.healthHandler)
	http.HandleFunc("/status", server.statusHandler)
	http.HandleFunc("/status-page", server.statusPageHandler)
	http.HandleFunc("/status-data", server.statusDataHandler)
	http.HandleFunc("/update-config", server.updateConfigHandler)
	http.HandleFunc("/start-workers", server.startWorkersHandler)
	http.HandleFunc("/stop-workers", server.stopWorkersHandler)
	http.HandleFunc("/add-name", server.addNameHandler)
	http.HandleFunc("/remove-name", server.removeNameHandler)
	http.HandleFunc("/clear-queue", server.clearQueueHandler)
	http.HandleFunc("/remove-from-queue", server.removeFromQueueHandler)

	log.Printf("Server starting on port 8080...")
	log.Printf("Data persistence: %s", dataPath)
	log.Printf("Health check timeout: %v", server.timeout)
	log.Printf("Max age: %v", server.maxAge)
	log.Printf("Subset size: %d", server.subsetSize)

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
