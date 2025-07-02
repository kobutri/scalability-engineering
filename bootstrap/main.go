package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type ClientIdentity struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
}

const (
	defaultTimeout             = 5 * time.Second
	defaultHealthCheckInterval = 100 * time.Millisecond
	defaultCleanupInterval     = 1 * time.Second
	defaultMaxAge              = 10 * time.Second
	defaultMinAge              = 2 * time.Second
	defaultSubsetSize          = 5
)

type Server struct {
	hashMap       *HashMap[string, string] // id -> client name
	priorityQueue *PriorityQueue[int64, string]
	startTime     time.Time

	// Configuration
	timeout             time.Duration
	maxAge              time.Duration
	minAge              time.Duration
	healthCheckInterval time.Duration
	cleanupInterval     time.Duration
	subsetSize          int

	// Worker control
	workersRunning bool
	stopWorkers    chan struct{}
	workersMutex   sync.RWMutex

	// Client management
	clientManager *ClientManager
}

func NewServer(dataPath string) *Server {
	// Create HashMap with persistence
	persistConfig := PersistenceConfig{
		Enabled:          true,
		FilePath:         dataPath,
		SnapshotInterval: 30 * time.Second,
		MaxRetries:       3,
	}

	hashMap := NewHashMapWithPersistence[string, string](persistConfig)

	// Try to load existing data
	if _, err := os.Stat(dataPath); err == nil {
		if err := hashMap.LoadFromDisk(dataPath); err != nil {
			log.Printf("Failed to load existing data: %v", err)
		}
	}

	// Initialize client manager
	workingDir, _ := os.Getwd()
	composeFile := filepath.Join(workingDir, "..", "docker-compose.yml")
	projectName := "scalability-engineering" // This should match your project name

	clientManager, err := NewClientManager(projectName, composeFile, workingDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize client manager: %v", err)
		// Continue without client manager
	}

	return &Server{
		hashMap:             hashMap,
		priorityQueue:       NewPriorityQueue[int64, string](),
		startTime:           time.Now(),
		timeout:             defaultTimeout,
		maxAge:              defaultMaxAge,
		minAge:              defaultMinAge,
		healthCheckInterval: defaultHealthCheckInterval,
		cleanupInterval:     defaultCleanupInterval,
		subsetSize:          defaultSubsetSize,
		workersRunning:      false,
		stopWorkers:         make(chan struct{}),
		clientManager:       clientManager,
	}
}

func (s *Server) getCurrentTimestamp() int64 {
	return time.Since(s.startTime).Microseconds()
}

func (s *Server) healthCheckWorker() {
	s.workersMutex.RLock()
	interval := s.healthCheckInterval
	s.workersMutex.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopWorkers:
			return
		case <-ticker.C:
			// Get a random container ID from the hashmap
			containerID, _, ok := s.hashMap.RandomEntry()
			if !ok {
				continue
			}

			// Check if it's alive in a separate goroutine to avoid blocking
			go func(containerID string) {
				currentTime := s.getCurrentTimestamp()

				// Check if already in priority queue (using container ID)
				if s.priorityQueue.Contains(containerID) {
					if priority, ok := s.priorityQueue.GetPriority(containerID); ok {
						age := currentTime - priority

						s.workersMutex.RLock()
						maxAgeMicros := s.maxAge.Microseconds()
						minAgeMicros := s.minAge.Microseconds()
						s.workersMutex.RUnlock()

						// If older than maxAge, remove it
						if age > maxAgeMicros {
							s.priorityQueue.Remove(containerID)
							return
						}

						// If newer than minAge, skip alive check
						if age < minAgeMicros {
							return
						}

						// Age is between minAge and maxAge, proceed with alive check
					}
				}

				// Proceed with alive check (using container ID for URL)
				if s.isAlive(containerID) {
					// Add to priority queue with current timestamp (using container ID)
					timestamp := s.getCurrentTimestamp()
					s.priorityQueue.Insert(timestamp, containerID)
				} else {
					s.priorityQueue.Remove(containerID)
					s.hashMap.Remove(containerID)
				}
			}(containerID)
		}
	}
}

func (s *Server) cleanupWorker() {
	s.workersMutex.RLock()
	interval := s.cleanupInterval
	s.workersMutex.RUnlock()

	ticker := time.NewTicker(interval)
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

func (s *Server) isAlive(id string) bool {
	url := fmt.Sprintf("http://%s:9090/alive", id)

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

func (s *Server) getRandomSubset() []ClientIdentity {
	containerIDs := s.priorityQueue.GetRandomSubset(s.subsetSize)
	identities := make([]ClientIdentity, 0, len(containerIDs))

	for _, containerID := range containerIDs {
		if name, exists := s.hashMap.Get(containerID); exists {
			identities = append(identities, ClientIdentity{
				Name:        name,
				ContainerID: containerID,
			})
		}
	}

	return identities
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

	var identity ClientIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if identity.ContainerID == "" || identity.Name == "" {
		http.Error(w, "ContainerID and Name cannot be empty", http.StatusBadRequest)
		return
	}

	// Insert into hashmap (id -> client name)
	s.hashMap.Put(identity.ContainerID, identity.Name)

	// Add to priority queue with current timestamp (using container ID as the unique identifier)
	timestamp := s.getCurrentTimestamp()
	s.priorityQueue.Insert(timestamp, identity.ContainerID)

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

	identifier := string(body)
	if identifier == "" {
		http.Error(w, "Identifier cannot be empty", http.StatusBadRequest)
		return
	}

	s.hashMap.Remove(identifier)
	s.priorityQueue.Remove(identifier)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"hashmap_size":        s.hashMap.Size(),
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

	for i, containerID := range values {
		priority := priorities[i]
		timestamp := time.Duration(priority) * time.Microsecond
		actualTime := s.startTime.Add(timestamp)
		age := time.Since(actualTime)

		// Get client name, fallback to empty string if not found
		clientName := ""
		if name, exists := s.hashMap.Get(containerID); exists {
			clientName = name
		}

		items[i] = QueueItem{
			ID:        containerID,
			Name:      clientName,
			Priority:  priority,
			Timestamp: actualTime.Format("15:04:05"),
			Age:       s.formatUptime(age),
		}
	}

	return items
}

func (s *Server) getStatusData() StatusData {
	uptime := time.Since(s.startTime)
	clientEntries := s.hashMap.Entries() // Get client ID-Name pairs
	queueItems := s.getQueueItems()

	s.workersMutex.RLock()
	workersRunning := s.workersRunning
	timeout := s.timeout
	maxAge := s.maxAge
	minAge := s.minAge
	healthCheckInterval := s.healthCheckInterval
	cleanupInterval := s.cleanupInterval
	subsetSize := s.subsetSize
	s.workersMutex.RUnlock()

	// Get client containers
	var clientContainers []ClientContainer
	if s.clientManager != nil {
		containers, err := s.clientManager.GetClientContainers(context.Background())
		if err != nil {
			log.Printf("Failed to get client containers: %v", err)
		} else {
			clientContainers = containers
		}
	}

	return StatusData{
		HashSetSize:       s.hashMap.Size(),
		PriorityQueueSize: s.priorityQueue.Size(),
		UptimeSeconds:     uptime.Seconds(),
		UptimeFormatted:   s.formatUptime(uptime),
		CurrentTime:       time.Now().Format("15:04:05"),
		ClientEntries:     clientEntries,
		QueueItems:        queueItems,
		WorkersRunning:    workersRunning,
		ClientContainers:  clientContainers,
		Config: ServerConfig{
			Timeout:             timeout.String(),
			MaxAge:              maxAge.String(),
			MinAge:              minAge.String(),
			HealthCheckInterval: healthCheckInterval.String(),
			CleanupInterval:     cleanupInterval.String(),
			SubsetSize:          subsetSize,
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

	restartWorkers := false

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

	// Update min age
	if minAgeStr := r.FormValue("minAge"); minAgeStr != "" {
		if minAge, err := time.ParseDuration(minAgeStr); err == nil {
			s.workersMutex.Lock()
			s.minAge = minAge
			s.workersMutex.Unlock()
		}
	}

	// Update health check interval
	if healthCheckIntervalStr := r.FormValue("healthCheckInterval"); healthCheckIntervalStr != "" {
		if healthCheckInterval, err := time.ParseDuration(healthCheckIntervalStr); err == nil {
			s.workersMutex.Lock()
			s.healthCheckInterval = healthCheckInterval
			s.workersMutex.Unlock()
			restartWorkers = true
		}
	}

	// Update cleanup interval
	if cleanupIntervalStr := r.FormValue("cleanupInterval"); cleanupIntervalStr != "" {
		if cleanupInterval, err := time.ParseDuration(cleanupIntervalStr); err == nil {
			s.workersMutex.Lock()
			s.cleanupInterval = cleanupInterval
			s.workersMutex.Unlock()
			restartWorkers = true
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

	// Restart workers if interval parameters changed
	if restartWorkers {
		s.workersMutex.RLock()
		workersWereRunning := s.workersRunning
		s.workersMutex.RUnlock()

		if workersWereRunning {
			s.stopWorkersFunc()
			s.startWorkersFunc()
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
	id := r.FormValue("id")
	if name != "" && id != "" {
		s.hashMap.Put(id, name)
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

	if id, ok := request["id"]; ok && id != "" {
		// Remove by container ID from hashmap
		if _, existed := s.hashMap.Remove(id); existed {
			// Also remove from priority queue (using container ID)
			s.priorityQueue.Remove(id)
		}
	} else if name, ok := request["name"]; ok && name != "" {
		// Fallback: remove by name (search through values)
		entries := s.hashMap.Entries()
		for _, entry := range entries {
			if entry.Value == name {
				containerID := entry.Key
				s.hashMap.Remove(containerID)
				s.priorityQueue.Remove(containerID)
				break
			}
		}
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

	if id, ok := request["id"]; ok && id != "" {
		s.priorityQueue.Remove(id)
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

// Client management handlers
func (s *Server) addClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.clientManager == nil {
		http.Error(w, "Client manager not available", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	clientName := r.FormValue("clientName")
	if clientName == "" {
		http.Error(w, "Client name is required", http.StatusBadRequest)
		return
	}

	if err := s.clientManager.CreateClient(r.Context(), clientName); err != nil {
		log.Printf("Failed to create client %s: %v", clientName, err)
		http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) removeClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.clientManager == nil {
		http.Error(w, "Client manager not available", http.StatusServiceUnavailable)
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

	containerName, ok := request["containerName"]
	if !ok || containerName == "" {
		http.Error(w, "Container name is required", http.StatusBadRequest)
		return
	}

	if err := s.clientManager.RemoveClient(r.Context(), containerName); err != nil {
		log.Printf("Failed to remove client %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to remove client: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) bulkCreateClientsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.clientManager == nil {
		http.Error(w, "Client manager not available", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	countStr := r.FormValue("count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		http.Error(w, "Invalid count", http.StatusBadRequest)
		return
	}

	if count > 200 {
		http.Error(w, "Count cannot exceed 200", http.StatusBadRequest)
		return
	}

	_, err = s.clientManager.BulkCreateClients(r.Context(), count)
	if err != nil {
		log.Printf("Failed to bulk create clients: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create clients: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getStatusData()
	statusContent(data).Render(r.Context(), w)
}

func (s *Server) removeAllClientsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.clientManager == nil {
		http.Error(w, "Client manager not available", http.StatusServiceUnavailable)
		return
	}

	// Use optimized bulk removal for better performance
	if err := s.clientManager.RemoveAllClients(r.Context()); err != nil {
		log.Printf("Failed to remove all clients: %v", err)
		http.Error(w, fmt.Sprintf("Failed to remove clients: %v", err), http.StatusInternalServerError)
		return
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
	server.minAge = getEnvDuration("MIN_AGE", defaultMinAge)
	server.healthCheckInterval = getEnvDuration("HEALTH_CHECK_INTERVAL", defaultHealthCheckInterval)
	server.cleanupInterval = getEnvDuration("CLEANUP_INTERVAL", defaultCleanupInterval)
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

	// Client management endpoints
	http.HandleFunc("/add-client", server.addClientHandler)
	http.HandleFunc("/remove-client", server.removeClientHandler)
	http.HandleFunc("/bulk-create-clients", server.bulkCreateClientsHandler)
	http.HandleFunc("/remove-all-clients", server.removeAllClientsHandler)

	log.Printf("Server starting on port 8080...")

	// Cleanup on exit
	defer func() {
		if server.clientManager != nil {
			server.clientManager.Close()
		}
	}()

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
