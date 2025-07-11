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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Configuration updated"))
}

// startWorkersHandler starts background workers
func (h *ClientManagerHandlers) startWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.clientManager.StartWorkers()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Workers started"))
}

// stopWorkersHandler stops background workers
func (h *ClientManagerHandlers) stopWorkersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.clientManager.StopWorkers()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Workers stopped"))
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

	containerID := r.FormValue("id")
	name := r.FormValue("name")

	if containerID == "" || name == "" {
		http.Error(w, "Container ID and name are required", http.StatusBadRequest)
		return
	}

	identity := ClientIdentity{
		ContainerID: containerID,
		Name:        name,
	}

	h.clientManager.AddClient(identity)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Client added"))
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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Client removed"))
}

// clearQueueHandler clears the priority queue
func (h *ClientManagerHandlers) clearQueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear both the hashmap and priority queue
	h.clientManager.Clear()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Queue cleared"))
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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Item removed from queue"))
}

// refreshHandler handles manual refresh requests
func (h *ClientManagerHandlers) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simply return success - the UI will auto-refresh anyway
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data refreshed"))
}
