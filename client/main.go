package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"shared"
)

type Client struct {
	identity        shared.ClientIdentity
	bootstrapURL    string
	connected       bool
	clientManager   *shared.ClientManager
	lastUpdate      time.Time
	connectionError string
	mutex           sync.RWMutex
	autoConnect     bool
	retryInterval   time.Duration
	maxRetries      int
	shutdownChan    chan bool

	queryQueue *shared.PriorityQueue[int64, string] // clients since last queried
	startTime  time.Time

	// Shared client manager handlers
	clientManagerHandlers *shared.ClientManagerHandlers
}

type Message struct {
	ContainerID string `json:"container_id"`
	MessageID   string `json:"message_id"`
	Timestamp   string `json:"timestamp"`
	Status      string `json:"status"` // sent, delivered, received
	Message     string `json:"message"`
	Port        int    `json:"port"`
}

// DiscoverClientIdentity discovers the client's identity from environment and system
func DiscoverClientIdentity() shared.ClientIdentity {
	identity := shared.ClientIdentity{}

	// Try CLIENT_NAME environment variable first (set by bootstrap)
	if name := os.Getenv("CLIENT_NAME"); name != "" {
		identity.Name = name
	} else {
		// Fallback to hostname-based name
		if hostname, err := os.Hostname(); err == nil {
			identity.Name = fmt.Sprintf("client-%s", hostname[:8]) // Use first 8 chars of container ID
		} else {
			// Last resort default
			identity.Name = "default-client"
		}
	}

	// Get container ID from hostname (Docker sets hostname to container ID)
	if hostname, err := os.Hostname(); err == nil {
		identity.ContainerID = hostname
	} else {
		identity.ContainerID = "unknown"
	}

	return identity
}

func NewClient(bootstrapURL string, autoConnect bool, retryInterval time.Duration, maxRetries int) *Client {
	identity := DiscoverClientIdentity()

	log.Printf("Discovered client identity: Name='%s', ContainerID='%s'",
		identity.Name, identity.ContainerID)

	// Create client manager with persistence
	config := shared.DefaultClientManagerConfig()
	config.PersistenceConfig.FilePath = fmt.Sprintf("../data/client-%s.json", identity.ContainerID)

	// Create the client first so we can reference it in the callbacks
	client := &Client{
		identity:      identity,
		bootstrapURL:  bootstrapURL,
		connected:     false,
		lastUpdate:    time.Now(),
		autoConnect:   autoConnect,
		retryInterval: retryInterval,
		maxRetries:    maxRetries,
		shutdownChan:  make(chan bool, 1),
		startTime:     time.Now(),
		queryQueue:    shared.NewPriorityQueue[int64, string](),
	}

	// Set up callbacks to maintain queryQueue
	config.PriorityQueueAddCallback = func(containerID string, priority int64) {
		// Use -priority as key (negative priority for reverse order)
		client.queryQueue.Insert(-priority, containerID)
	}

	config.PriorityQueueRemoveCallback = func(containerID string, priority int64) {
		// Remove from queryQueue
		client.queryQueue.Remove(containerID)
	}

	clientManager := shared.NewClientManager(config)

	// Create shared client manager handlers
	clientManagerHandlers := shared.NewClientManagerHandlers(clientManager)

	// Update the client with the remaining fields
	client.clientManager = clientManager
	client.clientManagerHandlers = clientManagerHandlers

	return client
}

func (c *Client) GetClientName() string {
	return c.identity.Name
}

func (c *Client) GetIdentity() shared.ClientIdentity {
	return c.identity
}

func (c *Client) connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	identityJSON, err := json.Marshal(c.identity)
	if err != nil {
		c.connectionError = fmt.Sprintf("Failed to encode identity: %v", err)
		c.connected = false
		return err
	}

	url := fmt.Sprintf("%s/connect", c.bootstrapURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(identityJSON))
	if err != nil {
		c.connectionError = fmt.Sprintf("Failed to connect: %v", err)
		c.connected = false
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.connectionError = fmt.Sprintf("Server error: %s", string(body))
		c.connected = false
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var clientIdentities []shared.ClientIdentity
	if err := json.NewDecoder(resp.Body).Decode(&clientIdentities); err != nil {
		c.connectionError = fmt.Sprintf("Failed to decode response: %v", err)
		c.connected = false
		return err
	}

	// Add received client identities to client manager
	newContactsCount := c.addNewContacts(clientIdentities)

	c.connected = true
	c.connectionError = ""
	c.lastUpdate = time.Now()
	log.Printf("Connected to bootstrap server. Received %d client identities, %d were new", len(clientIdentities), newContactsCount)

	return nil
}

func (c *Client) disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return nil
	}

	url := fmt.Sprintf("%s/delete", c.bootstrapURL)
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString(c.identity.ContainerID))
	if err != nil {
		c.connectionError = fmt.Sprintf("Failed to disconnect: %v", err)
		return err
	}
	defer resp.Body.Close()

	c.connected = false
	c.clientManager.Clear()
	c.connectionError = ""
	c.lastUpdate = time.Now()
	log.Printf("Disconnected from bootstrap server")
	return nil
}

// tryAutoConnect attempts a connection and manages retry counting
func (c *Client) tryAutoConnect(retryCount *int, isInitial bool) bool {
	c.mutex.RLock()
	isConnected := c.connected
	c.mutex.RUnlock()

	if isConnected {
		*retryCount = 0 // Reset retry count if already connected
		return true
	}

	if c.maxRetries > 0 && *retryCount >= c.maxRetries {
		log.Printf("Max retries (%d) reached, stopping auto-connect attempts", c.maxRetries)
		return false // Signal to stop trying
	}

	attemptMsg := "auto-connect"
	if isInitial {
		attemptMsg = "initial auto-connect"
	}

	log.Printf("Attempting %s (attempt %d)...", attemptMsg, *retryCount+1)
	if err := c.connect(); err != nil {
		(*retryCount)++
		log.Printf("Auto-connect failed: %v (retry %d)", err, *retryCount)
		return true // Continue trying
	} else {
		*retryCount = 0 // Reset retry count on successful connection
		log.Printf("Auto-connect successful!")
		return true
	}
}

// autoConnectWorker handles automatic connection with retry logic
func (c *Client) autoConnectWorker() {
	if !c.autoConnect {
		return
	}

	log.Printf("Starting auto-connect worker (retry interval: %v, max retries: %d)", c.retryInterval, c.maxRetries)

	retryCount := 0

	// Try to connect immediately on startup
	if !c.tryAutoConnect(&retryCount, true) {
		return // Max retries reached
	}

	ticker := time.NewTicker(c.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdownChan:
			log.Printf("Auto-connect worker shutting down")
			return
		case <-ticker.C:
			if !c.tryAutoConnect(&retryCount, false) {
				return // Max retries reached
			}
		}
	}
}

// Shutdown gracefully stops the auto-connect worker and client manager
func (c *Client) Shutdown() {
	if c.autoConnect {
		close(c.shutdownChan)
	}
	if c.clientManager != nil {
		c.clientManager.Close()
	}
}

// getClientData returns ClientData for the new component-based UI
func (c *Client) getClientData() ClientData {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	allClients := c.clientManager.GetAllClients()
	queueItems := c.clientManager.GetQueueItems()
	config := c.clientManager.GetConfig()

	// Convert client identities to entries format
	clientEntries := make([]shared.Entry[string, string], len(allClients))
	for i, client := range allClients {
		clientEntries[i] = shared.Entry[string, string]{
			Key:   client.ContainerID,
			Value: client.Name,
		}
	}

	uptime := time.Since(c.startTime)

	return ClientData{
		ServiceName:     c.identity.Name,
		ContainerID:     c.identity.ContainerID,
		UptimeSeconds:   uptime.Seconds(),
		UptimeFormatted: shared.FormatDuration(uptime),
		CurrentTime:     time.Now().Format("15:04:05"),
		ClientManagerData: shared.ClientManagerData{
			HashSetSize:       len(allClients),
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
			WorkersRunning: c.clientManager.IsWorkersRunning(),
		},
		BootstrapURL:    c.bootstrapURL,
		Connected:       c.connected,
		ConnectionError: c.connectionError,
		LastUpdate:      c.lastUpdate.Format("15:04:05"),
	}
}

func (c *Client) connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.connect()
	data := c.getClientData()
	ClientContent(data).Render(r.Context(), w)
}

func (c *Client) disconnectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.disconnect()
	data := c.getClientData()
	ClientContent(data).Render(r.Context(), w)
}

func (c *Client) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if c.connected {
		c.connect() // Reconnect to refresh names
	}
	data := c.getClientData()
	ClientContent(data).Render(r.Context(), w)
}

func (c *Client) statusHandler(w http.ResponseWriter, r *http.Request) {
	data := c.getClientData()
	ClientDashboard(data).Render(r.Context(), w)
}

func (c *Client) headerStatusHandler(w http.ResponseWriter, r *http.Request) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	uptime := time.Since(c.startTime)

	headerData := shared.HeaderData{
		ServiceName:     c.identity.Name,
		ContainerID:     c.identity.ContainerID,
		UptimeFormatted: shared.FormatDuration(uptime),
		CurrentTime:     time.Now().Format("15:04:05"),
		BootstrapURL:    c.bootstrapURL,
		WorkersRunning:  c.clientManager.IsWorkersRunning(),
	}

	shared.HeaderComponent(headerData).Render(r.Context(), w)
}

func (c *Client) connectionStatusHandler(w http.ResponseWriter, r *http.Request) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	connectionData := ConnectionControlData{
		Connected:       c.connected,
		BootstrapURL:    c.bootstrapURL,
		ConnectionError: c.connectionError,
	}

	ConnectionControlComponent(connectionData).Render(r.Context(), w)
}

func (c *Client) aliveHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (c *Client) messageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("messageHandler called")
	// CORS-Preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Nur POST erlauben
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode message: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received message from %s: %s", msg.ContainerID, msg.Message)

	// Validierung der erforderlichen Felder
	if msg.ContainerID == "" || msg.MessageID == "" || msg.Message == "" {
		http.Error(w, "containerID, messageID and message are required", http.StatusBadRequest)
		return
	}

	// Setze Standardwerte falls nicht vorhanden
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().Format(time.RFC3339)
	}
	if msg.Status == "" {
		msg.Status = "received"
	}

	// Nachricht speichern
	if err := RcvMessage(msg); err != nil {
		log.Printf("Failed to store message: %v", err)
		http.Error(w, fmt.Sprintf("failed to store message: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("message received"))
}

func (c *Client) chatDashboardHandler(w http.ResponseWriter, r *http.Request) {
	contacts, err := GetContactsWithLastMessages()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Kontakte", http.StatusInternalServerError)
		return
	}

	chatDashboard(contacts).Render(r.Context(), w)
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("sendMessageHandler called")

	// Formular-Daten parsen
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	containerName := r.FormValue("containerID")
	// Nachricht mit automatisch generierten Feldern erstellen
	msg := Message{
		ContainerID: containerName,
		Port:        12345, // Fester Port
		MessageID:   generateMessageID(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Status:      "sent", // Automatisch auf "sent" setzen
		Message:     r.FormValue("message"),
	}

	// JSON kodieren
	jsonData, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "Failed to create JSON", http.StatusInternalServerError)
		return
	}

	receiver := "scalability-engineering-client-" + containerName // Ensure the name has the prefix
	targetURL := fmt.Sprintf("http://%s:9090/message", receiver)

	log.Printf("Sending message to %s: %s", receiver, msg.Message)

	// REST-POST-Request an den Zielcontainer
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message to %s: %v", msg.ContainerID, err),
			http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	log.Printf("Message sent with status: %s", resp.Status)
	// Erfolgsmeldung zurückgeben
	contacts, err := GetContactsWithLastMessages()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Kontakte", http.StatusInternalServerError)
		return
	}

	chatDashboard(contacts).Render(r.Context(), w)
}

func generateMessageID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (c *Client) contactsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Accept contacts from request body and add them to client manager
	var receivedContacts []shared.ClientIdentity
	if err := json.NewDecoder(r.Body).Decode(&receivedContacts); err != nil {
		log.Printf("[contactsHandler] Failed to decode contacts: %v", err)
		http.Error(w, "Failed to decode contacts", http.StatusBadRequest)
		return
	}

	// Add new contacts to client manager
	newContactsCount := c.addNewContacts(receivedContacts)

	log.Printf("[contactsHandler] POST received %d contacts, %d were new",
		len(receivedContacts), newContactsCount)

	// Return a random subset of contacts
	randomSubset := c.clientManager.GetRandomSubset() // Return up to 15 clients
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(randomSubset)
}

func handleGetMessages(w http.ResponseWriter, r *http.Request) {
	containerID := r.URL.Query().Get("containerID")
	if containerID == "" {
		http.Error(w, "containerID fehlt", http.StatusBadRequest)
		return
	}
	messages, err := LoadMessagesForClient(containerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(messages)
}

func (c *Client) whatsAppHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

// addNewContacts adds contacts to the client manager, returning the count of new contacts added
func (c *Client) addNewContacts(contacts []shared.ClientIdentity) int {
	newContactsCount := 0
	for _, contact := range contacts {
		if !c.clientManager.ContainsClient(contact.ContainerID) && contact.ContainerID != c.identity.ContainerID {
			c.clientManager.AddClient(contact)
			newContactsCount++
		}
	}
	return newContactsCount
}

// expandContacts extracts a client from queryQueue and queries it for more contacts
func (c *Client) expandContacts() {
	// Extract the oldest element from queryQueue (highest priority after negation)
	_, containerID, ok := c.queryQueue.ExtractMin()
	if !ok {
		return
	}

	// Get a random subset of current clients to send
	randomSubset := c.clientManager.GetRandomSubset()

	// Always include our own identity in the subset
	randomSubset = append(randomSubset, c.identity)

	// Marshal the subset for POST request
	jsonData, err := json.Marshal(randomSubset)
	if err != nil {
		log.Printf("Failed to marshal random subset: %v", err)
		return
	}

	// Make POST request to the client's contacts endpoint
	targetURL := fmt.Sprintf("http://%s:9090/contacts", containerID)
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to query contacts from %s: %v", containerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Contacts query to %s returned status %d", containerID, resp.StatusCode)
		return
	}

	// Parse response
	var receivedContacts []shared.ClientIdentity
	if err := json.NewDecoder(resp.Body).Decode(&receivedContacts); err != nil {
		log.Printf("Failed to decode contacts response from %s: %v", containerID, err)
		return
	}

	c.addNewContacts(receivedContacts)

	c.queryQueue.Insert(-time.Now().UnixNano(), containerID) // Reinsert with current time as priority
}

// Routine Worker:
func (c *Client) contactExpansionWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdownChan:
			log.Println("Contact expansion worker shutting down")
			return
		case <-ticker.C:
			c.expandContacts()
		}
	}
}

// MAIN
func main() {
	InitiateDB()
	bootstrapURL := os.Getenv("BOOTSTRAP_URL")
	if bootstrapURL == "" {
		bootstrapURL = "http://localhost:8080"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	// Auto-connect configuration
	autoConnect := true
	if env := os.Getenv("AUTO_CONNECT"); env != "" {
		if parsed, err := strconv.ParseBool(env); err == nil {
			autoConnect = parsed
		}
	}

	retryInterval := 10 * time.Second
	if env := os.Getenv("RETRY_INTERVAL"); env != "" {
		if parsed, err := time.ParseDuration(env); err == nil {
			retryInterval = parsed
		}
	}

	maxRetries := 0 // 0 means unlimited retries
	if env := os.Getenv("MAX_RETRIES"); env != "" {
		if parsed, err := strconv.Atoi(env); err == nil {
			maxRetries = parsed
		}
	}

	client := NewClient(bootstrapURL, autoConnect, retryInterval, maxRetries)

	// Start client manager workers
	client.clientManager.StartWorkers()

	// Start auto-connect worker if enabled
	if autoConnect {
		go client.autoConnectWorker()
	}

	// Starte contact-expansion worker, um Kontakte periodisch auszubauen
	go client.contactExpansionWorker(30 * time.Second)

	// Register shared client manager handlers
	client.clientManagerHandlers.RegisterHandlers()

	// Register HTTP handlers
	http.HandleFunc("/header-status", client.headerStatusHandler)
	http.HandleFunc("/connection-status", client.connectionStatusHandler)
	http.HandleFunc("/connect", client.connectHandler)
	http.HandleFunc("/disconnect", client.disconnectHandler)
	http.HandleFunc("/refresh", client.refreshHandler)
	http.HandleFunc("/alive", client.aliveHandler)

	//Chats
	http.HandleFunc("/message", client.messageHandler)    // Receive messages
	http.HandleFunc("/chat", client.chatDashboardHandler) // Show chat dashboard
	http.HandleFunc("/send-message", sendMessageHandler)
	http.HandleFunc("/contacts", client.contactsHandler)
	http.HandleFunc("/messages", handleGetMessages)
	http.HandleFunc("/chats", client.whatsAppHandler) // Show chat dashboard

	//nach unten verschoben, sonst werden andere Unterpfade abgefangen
	http.HandleFunc("/", client.statusHandler)

	log.Printf("Starting client '%s' on port %s", client.GetClientName(), port)
	log.Printf("Bootstrap server URL: %s", bootstrapURL)
	log.Printf("Container ID: %s", client.GetIdentity().ContainerID)
	log.Printf("Auto-connect: %v (retry interval: %v, max retries: %d)", autoConnect, retryInterval, maxRetries)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
