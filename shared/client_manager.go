package shared

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// PriorityQueueAddCallback is called when an element is added to the priority queue
type PriorityQueueAddCallback func(containerID string, priority int64)

// PriorityQueueRemoveCallback is called when an element is removed from the priority queue
type PriorityQueueRemoveCallback func(containerID string, priority int64)

// ClientIdentity represents a client in the system
type ClientIdentity struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
	Hostname    string `json:"domain_name"`
}

// ClientManagerConfig holds configuration for the client manager
type ClientManagerConfig struct {
	Timeout                     time.Duration
	MaxAge                      time.Duration
	MinAge                      time.Duration
	HealthCheckInterval         time.Duration
	CleanupInterval             time.Duration
	SubsetSize                  int
	PersistenceConfig           PersistenceConfig
	PriorityQueueAddCallback    PriorityQueueAddCallback    // Optional callback for priority queue add operations
	PriorityQueueRemoveCallback PriorityQueueRemoveCallback // Optional callback for priority queue remove operations
}

// DefaultClientManagerConfig returns a default configuration
func DefaultClientManagerConfig() ClientManagerConfig {
	return ClientManagerConfig{
		Timeout:             5 * time.Second,
		MaxAge:              10 * time.Second,
		MinAge:              2 * time.Second,
		HealthCheckInterval: 100 * time.Millisecond,
		CleanupInterval:     1 * time.Second,
		SubsetSize:          5,
		PersistenceConfig: PersistenceConfig{
			Enabled:          true,
			SnapshotInterval: 30 * time.Second,
			MaxRetries:       3,
		},
	}
}

// ClientManager manages client identities with health checks and persistence
type ClientManager struct {
	hashMap                     *HashMap[string, string]      // container ID -> client name
	priorityQueue               *PriorityQueue[int64, string] // timestamp -> container ID
	startTime                   time.Time
	config                      ClientManagerConfig
	priorityQueueAddCallback    PriorityQueueAddCallback    // Optional callback for priority queue add operations
	priorityQueueRemoveCallback PriorityQueueRemoveCallback // Optional callback for priority queue remove operations

	// Worker control
	workersRunning bool
	stopWorkers    chan struct{}
	workersMutex   sync.RWMutex
	workerWG       sync.WaitGroup
}

// NewClientManager creates a new client manager with the given configuration
func NewClientManager(config ClientManagerConfig) *ClientManager {
	// Create HashMap with persistence if enabled
	var hashMap *HashMap[string, string]
	if config.PersistenceConfig.Enabled {
		hashMap = NewHashMapWithPersistence[string, string](config.PersistenceConfig)
		// Try to load existing data
		if config.PersistenceConfig.FilePath != "" {
			if err := hashMap.LoadFromDisk(config.PersistenceConfig.FilePath); err != nil {
				log.Printf("Failed to load existing client data: %v", err)
			}
		}
	} else {
		hashMap = NewHashMap[string, string]()
	}

	return &ClientManager{
		hashMap:                     hashMap,
		priorityQueue:               NewPriorityQueue[int64, string](),
		startTime:                   time.Now(),
		config:                      config,
		priorityQueueAddCallback:    config.PriorityQueueAddCallback,
		priorityQueueRemoveCallback: config.PriorityQueueRemoveCallback,
		stopWorkers:                 make(chan struct{}),
	}
}

// StartWorkers starts the health check and cleanup workers
func (cm *ClientManager) StartWorkers() {
	cm.workersMutex.Lock()
	defer cm.workersMutex.Unlock()

	if cm.workersRunning {
		return
	}

	cm.stopWorkers = make(chan struct{})
	cm.workersRunning = true

	cm.workerWG.Add(2)
	go cm.healthCheckWorker()
	go cm.cleanupWorker()
}

// StopWorkers stops the health check and cleanup workers
func (cm *ClientManager) StopWorkers() {
	cm.workersMutex.Lock()
	defer cm.workersMutex.Unlock()

	if !cm.workersRunning {
		return
	}

	close(cm.stopWorkers)
	cm.workersRunning = false
	cm.workerWG.Wait()
}

// getCurrentTimestamp returns the current timestamp in microseconds since start
func (cm *ClientManager) getCurrentTimestamp() int64 {
	return time.Since(cm.startTime).Microseconds()
}

// callPriorityQueueAddCallback safely calls the priority queue add callback if it's set
func (cm *ClientManager) callPriorityQueueAddCallback(containerID string, priority int64) {
	if cm.priorityQueueAddCallback != nil {
		cm.priorityQueueAddCallback(containerID, priority)
	}
}

// callPriorityQueueRemoveCallback safely calls the priority queue remove callback if it's set
func (cm *ClientManager) callPriorityQueueRemoveCallback(containerID string, priority int64) {
	if cm.priorityQueueRemoveCallback != nil {
		cm.priorityQueueRemoveCallback(containerID, priority)
	}
}

// AddClient adds a client to the manager
func (cm *ClientManager) AddClient(identity ClientIdentity) {
	if identity.ContainerID == "" || identity.Name == "" {
		return
	}

	// Store in hashmap
	cm.hashMap.Put(identity.ContainerID, identity.Name)

	// Add to priority queue with current timestamp
	timestamp := cm.getCurrentTimestamp()
	cm.priorityQueue.Insert(timestamp, identity.ContainerID)

	// Call callback for priority queue operation
	cm.callPriorityQueueAddCallback(identity.ContainerID, timestamp)
}

// RemoveClient removes a client from the manager
func (cm *ClientManager) RemoveClient(containerID string) bool {
	removed := false
	if _, existed := cm.hashMap.Remove(containerID); existed {
		removed = true
	}

	// Get priority before removing to pass to callback
	var priority int64
	if p, exists := cm.priorityQueue.GetPriority(containerID); exists {
		priority = p
	}

	if cm.priorityQueue.Remove(containerID) {
		removed = true
		// Call callback for priority queue operation
		cm.callPriorityQueueRemoveCallback(containerID, priority)
	}

	return removed
}

// GetClient retrieves a client by container ID
func (cm *ClientManager) GetClient(containerID string) (ClientIdentity, bool) {
	name, exists := cm.hashMap.Get(containerID)
	if !exists {
		return ClientIdentity{}, false
	}

	return ClientIdentity{
		Name:        name,
		ContainerID: containerID,
	}, true
}

// GetAllClients returns all client identities
func (cm *ClientManager) GetAllClients() []ClientIdentity {
	entries := cm.hashMap.Entries()
	clients := make([]ClientIdentity, 0, len(entries))

	for _, entry := range entries {
		clients = append(clients, ClientIdentity{
			Name:        entry.Value,
			ContainerID: entry.Key,
		})
	}

	return clients
}

// GetRandomSubset returns a random subset of clients
func (cm *ClientManager) GetRandomSubset() []ClientIdentity {
	containerIDs := cm.priorityQueue.GetRandomSubset(cm.config.SubsetSize)
	identities := make([]ClientIdentity, 0, len(containerIDs))

	for _, containerID := range containerIDs {
		if name, exists := cm.hashMap.Get(containerID); exists {
			identities = append(identities, ClientIdentity{
				Name:        name,
				ContainerID: containerID,
			})
		}
	}

	return identities
}

// ContainsClient checks if a client exists
func (cm *ClientManager) ContainsClient(containerID string) bool {
	return cm.hashMap.ContainsKey(containerID)
}

// Size returns the number of clients
func (cm *ClientManager) Size() int {
	return cm.hashMap.Size()
}

// Clear removes all clients
func (cm *ClientManager) Clear() {
	cm.hashMap.Clear()
	cm.priorityQueue.Clear()
}

// UpdateConfig updates the manager configuration
func (cm *ClientManager) UpdateConfig(newConfig ClientManagerConfig) {
	cm.workersMutex.Lock()
	defer cm.workersMutex.Unlock()

	restartWorkers := false
	if cm.config.HealthCheckInterval != newConfig.HealthCheckInterval ||
		cm.config.CleanupInterval != newConfig.CleanupInterval {
		restartWorkers = true
	}

	cm.config = newConfig
	cm.priorityQueueAddCallback = newConfig.PriorityQueueAddCallback
	cm.priorityQueueRemoveCallback = newConfig.PriorityQueueRemoveCallback

	if restartWorkers && cm.workersRunning {
		// Restart workers with new intervals
		close(cm.stopWorkers)
		cm.workerWG.Wait()
		cm.stopWorkers = make(chan struct{})
		cm.workerWG.Add(2)
		go cm.healthCheckWorker()
		go cm.cleanupWorker()
	}
}

// GetConfig returns the current configuration
func (cm *ClientManager) GetConfig() ClientManagerConfig {
	cm.workersMutex.RLock()
	defer cm.workersMutex.RUnlock()
	return cm.config
}

// IsWorkersRunning returns whether workers are running
func (cm *ClientManager) IsWorkersRunning() bool {
	cm.workersMutex.RLock()
	defer cm.workersMutex.RUnlock()
	return cm.workersRunning
}

// Close shuts down the client manager
func (cm *ClientManager) Close() error {
	cm.StopWorkers()
	return cm.hashMap.Close()
}

// healthCheckWorker runs periodic health checks on clients
func (cm *ClientManager) healthCheckWorker() {
	defer cm.workerWG.Done()

	cm.workersMutex.RLock()
	interval := cm.config.HealthCheckInterval
	cm.workersMutex.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopWorkers:
			return
		case <-ticker.C:
			// Get a random container ID from the hashmap
			containerID, _, ok := cm.hashMap.RandomEntry()
			if !ok {
				continue
			}

			// Check if it's alive in a separate goroutine to avoid blocking
			go func(containerID string) {
				currentTime := cm.getCurrentTimestamp()

				// Check if already in priority queue
				if cm.priorityQueue.Contains(containerID) {
					if priority, ok := cm.priorityQueue.GetPriority(containerID); ok {
						age := currentTime - priority

						cm.workersMutex.RLock()
						maxAgeMicros := cm.config.MaxAge.Microseconds()
						minAgeMicros := cm.config.MinAge.Microseconds()
						cm.workersMutex.RUnlock()

						// If older than maxAge, remove it
						if age > maxAgeMicros {
							if cm.priorityQueue.Remove(containerID) {
								cm.callPriorityQueueRemoveCallback(containerID, priority)
							}
							return
						}

						// If newer than minAge, skip alive check
						if age < minAgeMicros {
							return
						}
					}
				}

				// Proceed with alive check
				if cm.isAlive(containerID) {
					// Add to priority queue with current timestamp
					timestamp := cm.getCurrentTimestamp()
					cm.priorityQueue.Insert(timestamp, containerID)
					cm.callPriorityQueueAddCallback(containerID, timestamp)
				} else {
					// Get priority before removing to pass to callback
					var priority int64
					if p, exists := cm.priorityQueue.GetPriority(containerID); exists {
						priority = p
					}

					// Remove from both structures
					if cm.priorityQueue.Remove(containerID) {
						cm.callPriorityQueueRemoveCallback(containerID, priority)
					}
					cm.hashMap.Remove(containerID)
				}
			}(containerID)
		}
	}
}

// cleanupWorker removes old entries from the priority queue
func (cm *ClientManager) cleanupWorker() {
	defer cm.workerWG.Done()

	cm.workersMutex.RLock()
	interval := cm.config.CleanupInterval
	cm.workersMutex.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopWorkers:
			return
		case <-ticker.C:
			cm.workersMutex.RLock()
			maxAgeMicros := cm.config.MaxAge.Microseconds()
			cm.workersMutex.RUnlock()

			currentTime := cm.getCurrentTimestamp()
			cutoffTime := currentTime - maxAgeMicros

			// Remove old entries from priority queue
			for {
				priority, _, ok := cm.priorityQueue.Peek()
				if !ok || priority >= cutoffTime {
					break
				}

				// Extract the minimum and call callback
				extractedPriority, extractedContainerID, ok := cm.priorityQueue.ExtractMin()
				if ok {
					cm.callPriorityQueueRemoveCallback(extractedContainerID, extractedPriority)
				}
			}
		}
	}
}

// isAlive checks if a client is alive by pinging its /alive endpoint
func (cm *ClientManager) isAlive(containerID string) bool {
	url := fmt.Sprintf("http://%s:9090/alive", containerID)

	cm.workersMutex.RLock()
	timeout := cm.config.Timeout
	cm.workersMutex.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

// GetStats returns statistics about the client manager
type ClientManagerStats struct {
	HashMapSize       int
	PriorityQueueSize int
	WorkersRunning    bool
	UptimeSeconds     float64
}

func (cm *ClientManager) GetStats() ClientManagerStats {
	return ClientManagerStats{
		HashMapSize:       cm.hashMap.Size(),
		PriorityQueueSize: cm.priorityQueue.Size(),
		WorkersRunning:    cm.IsWorkersRunning(),
		UptimeSeconds:     time.Since(cm.startTime).Seconds(),
	}
}

// QueueItem represents an item in the priority queue for display
type QueueItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Priority  int64  `json:"priority"`
	Timestamp string `json:"timestamp"`
	Age       string `json:"age"`
}

// GetQueueItems returns formatted queue items for display
func (cm *ClientManager) GetQueueItems() []QueueItem {
	priorities, values := cm.priorityQueue.ToSlices()
	items := make([]QueueItem, len(values))

	for i, containerID := range values {
		priority := priorities[i]
		timestamp := time.Duration(priority) * time.Microsecond
		actualTime := cm.startTime.Add(timestamp)
		age := time.Since(actualTime)

		// Get client name, fallback to empty string if not found
		clientName := ""
		if name, exists := cm.hashMap.Get(containerID); exists {
			clientName = name
		}

		items[i] = QueueItem{
			ID:        containerID,
			Name:      clientName,
			Priority:  priority,
			Timestamp: actualTime.Format("15:04:05"),
			Age:       formatDuration(age),
		}
	}

	return items
}

// formatDuration formats a duration for display
func formatDuration(duration time.Duration) string {
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
