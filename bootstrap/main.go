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
	"time"

	"shared"
)

type Server struct {
	clientManager *shared.ClientManager
	startTime     time.Time

	// Client management (Docker)
	dockerClientManager *ClientManager

	// Shared client manager handlers
	clientManagerHandlers *shared.ClientManagerHandlers
}

func NewServer(dataPath string) *Server {
	// Create ClientManager with persistence
	config := shared.DefaultClientManagerConfig()
	config.PersistenceConfig.FilePath = dataPath
	clientManager := shared.NewClientManager(config)

	// Initialize docker client manager
	workingDir, _ := os.Getwd()
	composeFile := filepath.Join(workingDir, "..", "docker-compose.yml")
	projectName := "scalability-engineering" // This should match your project name

	dockerClientManager, err := NewClientManager(projectName, composeFile, workingDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize docker client manager: %v", err)
		// Continue without docker client manager
	}

	// Create shared client manager handlers
	clientManagerHandlers := shared.NewClientManagerHandlers(clientManager)

	return &Server{
		clientManager:         clientManager,
		startTime:             time.Now(),
		dockerClientManager:   dockerClientManager,
		clientManagerHandlers: clientManagerHandlers,
	}
}

func (s *Server) getRandomSubset() []shared.ClientIdentity {
	return s.clientManager.GetRandomSubset()
}

func (s *Server) getBootstrapData() BootstrapData {
	uptime := time.Since(s.startTime)
	allClients := s.clientManager.GetAllClients()
	queueItems := s.clientManager.GetQueueItems()
	config := s.clientManager.GetConfig()

	// Convert client identities to entries format
	clientEntries := make([]shared.Entry[string, string], len(allClients))
	for i, client := range allClients {
		clientEntries[i] = shared.Entry[string, string]{
			Key:   client.ContainerID,
			Value: client.Name,
		}
	}

	// Get client containers for Docker management
	var clientContainers []shared.ClientContainer
	if s.dockerClientManager != nil {
		containers, err := s.dockerClientManager.GetClientContainers(context.Background())
		if err != nil {
			log.Printf("Failed to get client containers: %v", err)
		} else {
			// Convert to shared.ClientContainer
			clientContainers = make([]shared.ClientContainer, len(containers))
			for i, container := range containers {
				clientContainers[i] = shared.ClientContainer{
					Name:     container.Name,
					Status:   container.Status,
					ID:       container.CreatedAt, // Use CreatedAt as ID since ID field doesn't exist
					HostPort: container.HostPort,
					WebURL:   container.WebURL,
				}
			}
		}
	}

	return BootstrapData{
		ServiceName:     "Bootstrap Server",
		ContainerID:     "bootstrap-server",
		UptimeSeconds:   uptime.Seconds(),
		UptimeFormatted: shared.FormatDuration(uptime),
		CurrentTime:     time.Now().Format("15:04:05"),
		ClientManagerData: shared.ClientManagerData{
			HashSetSize:       s.clientManager.Size(),
			PriorityQueueSize: len(queueItems),
			ClientEntries:     clientEntries,
			QueueItems:        queueItems,
			Config: shared.ServiceConfig{
				Timeout:             config.Timeout.String(),
				MaxAge:              config.MaxAge.String(),
				MinAge:              config.MinAge.String(),
				HealthCheckInterval: config.HealthCheckInterval.String(),
				CleanupInterval:     config.CleanupInterval.String(),
				SubsetSize:          config.SubsetSize,
			},
			WorkersRunning: s.clientManager.IsWorkersRunning(),
		},
		ClientContainers: clientContainers,
	}
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

	var identity shared.ClientIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if identity.ContainerID == "" || identity.Name == "" {
		http.Error(w, "ContainerID and Name cannot be empty", http.StatusBadRequest)
		return
	}

	// Add client to manager
	s.clientManager.AddClient(identity)

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

	s.clientManager.RemoveClient(identifier)

	w.WriteHeader(http.StatusOK)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.clientManager.GetStats()
	status := map[string]interface{}{
		"hashmap_size":        stats.HashMapSize,
		"priority_queue_size": stats.PriorityQueueSize,
		"uptime_seconds":      stats.UptimeSeconds,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) statusPageHandler(w http.ResponseWriter, r *http.Request) {
	data := s.getBootstrapData()
	BootstrapDashboard(data).Render(r.Context(), w)
}

func (s *Server) statusDataHandler(w http.ResponseWriter, r *http.Request) {
	data := s.getBootstrapData()
	BootstrapContent(data).Render(r.Context(), w)
}

// Client management handlers (Docker-specific)
func (s *Server) addClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.dockerClientManager == nil {
		http.Error(w, "Docker client manager not available", http.StatusServiceUnavailable)
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

	if err := s.dockerClientManager.CreateClient(r.Context(), clientName); err != nil {
		log.Printf("Failed to create client %s: %v", clientName, err)
		http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getBootstrapData()
	BootstrapContent(data).Render(r.Context(), w)
}

func (s *Server) removeClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.dockerClientManager == nil {
		http.Error(w, "Docker client manager not available", http.StatusServiceUnavailable)
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

	if err := s.dockerClientManager.RemoveClient(r.Context(), containerName); err != nil {
		log.Printf("Failed to remove client %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to remove client: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getBootstrapData()
	BootstrapContent(data).Render(r.Context(), w)
}

func (s *Server) bulkCreateClientsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.dockerClientManager == nil {
		http.Error(w, "Docker client manager not available", http.StatusServiceUnavailable)
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

	_, err = s.dockerClientManager.BulkCreateClients(r.Context(), count)
	if err != nil {
		log.Printf("Failed to bulk create clients: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create clients: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getBootstrapData()
	BootstrapContent(data).Render(r.Context(), w)
}

func (s *Server) removeAllClientsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.dockerClientManager == nil {
		http.Error(w, "Docker client manager not available", http.StatusServiceUnavailable)
		return
	}

	// Use optimized bulk removal for better performance
	if err := s.dockerClientManager.RemoveAllClients(r.Context()); err != nil {
		log.Printf("Failed to remove all clients: %v", err)
		http.Error(w, fmt.Sprintf("Failed to remove clients: %v", err), http.StatusInternalServerError)
		return
	}

	data := s.getBootstrapData()
	BootstrapContent(data).Render(r.Context(), w)
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
	config := server.clientManager.GetConfig()
	config.Timeout = getEnvDuration("HEALTH_TIMEOUT", config.Timeout)
	config.MaxAge = getEnvDuration("MAX_AGE", config.MaxAge)
	config.MinAge = getEnvDuration("MIN_AGE", config.MinAge)
	config.HealthCheckInterval = getEnvDuration("HEALTH_CHECK_INTERVAL", config.HealthCheckInterval)
	config.CleanupInterval = getEnvDuration("CLEANUP_INTERVAL", config.CleanupInterval)
	config.SubsetSize = getEnvInt("SUBSET_SIZE", config.SubsetSize)
	server.clientManager.UpdateConfig(config)

	// Start background workers
	server.clientManager.StartWorkers()

	// Register shared client manager handlers
	server.clientManagerHandlers.RegisterHandlers()

	// Setup HTTP routes
	http.HandleFunc("/connect", server.connectHandler)
	http.HandleFunc("/delete", server.deleteHandler)
	http.HandleFunc("/", server.healthHandler)
	http.HandleFunc("/status", server.statusHandler)
	http.HandleFunc("/status-page", server.statusPageHandler)
	http.HandleFunc("/status-data", server.statusDataHandler)

	// Docker client management endpoints
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
		if server.dockerClientManager != nil {
			server.dockerClientManager.Close()
		}
	}()

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
