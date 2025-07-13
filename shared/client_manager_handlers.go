package shared

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

// ClientManagerHandlers provides HTTP handlers for the client manager
type ClientManagerHandlers struct {
	clientManager *ClientManager
}

// NewClientManagerHandlers creates a new ClientManagerHandlers instance
func NewClientManagerHandlers(clientManager *ClientManager) *ClientManagerHandlers {
	return &ClientManagerHandlers{
		clientManager: clientManager,
	}
}

// RegisterHandlers registers all client manager HTTP handlers
func (h *ClientManagerHandlers) RegisterHandlers() {
	// Configuration handlers
	http.HandleFunc("/client-manager/update-config", h.updateConfigHandler)
	http.HandleFunc("/client-manager/start-workers", h.startWorkersHandler)
	http.HandleFunc("/client-manager/stop-workers", h.stopWorkersHandler)

	// Client management handlers
	http.HandleFunc("/client-manager/add-client", h.addClientHandler)
	http.HandleFunc("/client-manager/remove-client", h.removeClientHandler)

	// Queue management handlers
	http.HandleFunc("/client-manager/clear-queue", h.clearQueueHandler)
	http.HandleFunc("/client-manager/remove-from-queue", h.removeFromQueueHandler)

	// Data refresh handlers
	http.HandleFunc("/client-manager/refresh", h.refreshHandler)
}

// updateConfigHandler handles configuration updates
func (h *ClientManagerHandlers) updateConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	config := h.clientManager.GetConfig()

	// Parse timeout
	if timeout := r.FormValue("timeout"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			config.Timeout = d
		}
	}

	// Parse max age
	if maxAge := r.FormValue("maxAge"); maxAge != "" {
		if d, err := time.ParseDuration(maxAge); err == nil {
			config.MaxAge = d
		}
	}

	// Parse min age
	if minAge := r.FormValue("minAge"); minAge != "" {
		if d, err := time.ParseDuration(minAge); err == nil {
			config.MinAge = d
		}
	}

	// Parse health check interval
	if interval := r.FormValue("healthCheckInterval"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			config.HealthCheckInterval = d
		}
	}

	// Parse cleanup interval
	if cleanup := r.FormValue("cleanupInterval"); cleanup != "" {
		if d, err := time.ParseDuration(cleanup); err == nil {
			config.CleanupInterval = d
		}
	}

	// Parse subset size
	if subset := r.FormValue("subsetSize"); subset != "" {
		if s, err := strconv.Atoi(subset); err == nil && s > 0 {
			config.SubsetSize = s
		}
	}

	// Update configuration
	h.clientManager.UpdateConfig(config)

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// startWorkersHandler starts background workers
func (h *ClientManagerHandlers) startWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.clientManager.StartWorkers()

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// stopWorkersHandler stops background workers
func (h *ClientManagerHandlers) stopWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.clientManager.StopWorkers()

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// addClientHandler manually adds a client
func (h *ClientManagerHandlers) addClientHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	containerID := r.FormValue("container_id")
	name := r.FormValue("name")
	hostname := r.FormValue("hostname")

	if containerID == "" || name == "" {
		http.Error(w, "Container ID and name are required", http.StatusBadRequest)
		return
	}

	identity := ClientIdentity{
		ContainerID: containerID,
		Name:        name,
		Hostname:    hostname,
	}

	h.clientManager.AddClient(identity)

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// removeClientHandler removes a client
func (h *ClientManagerHandlers) removeClientHandler(w http.ResponseWriter, r *http.Request) {
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

	id, ok := request["id"]
	if !ok || id == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	h.clientManager.RemoveClient(id)

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// clearQueueHandler clears the priority queue
func (h *ClientManagerHandlers) clearQueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear both the hashmap and priority queue
	h.clientManager.Clear()

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// removeFromQueueHandler removes an item from the priority queue
func (h *ClientManagerHandlers) removeFromQueueHandler(w http.ResponseWriter, r *http.Request) {
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

	id, ok := request["id"]
	if !ok || id == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	// Remove the client entirely (from both hashmap and priority queue)
	h.clientManager.RemoveClient(id)

	// Return the updated client manager component
	h.renderClientManagerComponent(w, r)
}

// renderClientManagerComponent renders the client manager component with current data
func (h *ClientManagerHandlers) renderClientManagerComponent(w http.ResponseWriter, r *http.Request) {
	// Get fresh data from client manager
	allClients := h.clientManager.GetAllClients()
	queueItems := h.clientManager.GetQueueItems()
	config := h.clientManager.GetConfig()

	// Convert client identities to ClientEntry format
	clientEntries := make([]ClientEntry, len(allClients))
	for i, client := range allClients {
		clientEntries[i] = ClientEntry{
			ContainerID: client.ContainerID,
			Name:        client.Name,
			Hostname:    client.Hostname,
		}
	}

	// Create ClientManagerData
	data := ClientManagerData{
		HashSetSize:       h.clientManager.Size(),
		PriorityQueueSize: len(queueItems),
		ClientEntries:     clientEntries,
		QueueItems:        queueItems,
		Config: ServiceConfig{
			Timeout:             config.Timeout.String(),
			MaxAge:              config.MaxAge.String(),
			MinAge:              config.MinAge.String(),
			HealthCheckInterval: config.HealthCheckInterval.String(),
			CleanupInterval:     config.CleanupInterval.String(),
			SubsetSize:          config.SubsetSize,
		},
		WorkersRunning: h.clientManager.IsWorkersRunning(),
	}

	// Return the client manager component
	ClientManagerComponent(data).Render(r.Context(), w)
}

// refreshHandler handles manual refresh requests
func (h *ClientManagerHandlers) refreshHandler(w http.ResponseWriter, r *http.Request) {
	// Support both GET and POST for polling
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Use the shared rendering function
	h.renderClientManagerComponent(w, r)
}
