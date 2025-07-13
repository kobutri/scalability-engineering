package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
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

	sequenceCounter int64

	// Shared client manager handlers
	clientManagerHandlers *shared.ClientManagerHandlers
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

	// Get container ID and hostname from hostname (Docker sets hostname to container ID)
	if hostname, err := os.Hostname(); err == nil {
		identity.ContainerID = hostname
		identity.Hostname = hostname // For inter-container communication
	} else {
		identity.ContainerID = "unknown"
		identity.Hostname = "unknown"
	}

	return identity
}

func NewClient(bootstrapURL string, autoConnect bool, retryInterval time.Duration, maxRetries int) *Client {
	identity := DiscoverClientIdentity()

	log.Printf("Discovered client identity: Name='%s', ContainerID='%s', Hostname='%s'",
		identity.Name, identity.ContainerID, identity.Hostname)

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
	// In NewClient():
	config.PriorityQueueAddCallback = func(containerID string, _ int64) {
		// Setze initial niedrige Priorität bei erstem Insert
		if !client.queryQueue.Contains(containerID) {
			client.sequenceCounter++
			client.queryQueue.Insert(client.sequenceCounter, containerID)
		}
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

	// Convert client identities to ClientEntry format
	clientEntries := make([]shared.ClientEntry, len(allClients))
	for i, client := range allClients {
		clientEntries[i] = shared.ClientEntry{
			ContainerID: client.ContainerID,
			Name:        client.Name,
			Hostname:    client.Hostname,
		}
	}

	uptime := time.Since(c.startTime)

	return ClientData{
		ServiceName:     c.identity.Name,
		ContainerID:     c.identity.ContainerID,
		UptimeSeconds:   uptime.Seconds(),
		UptimeFormatted: shared.FormatDuration(uptime),
		CurrentTime:     time.Now().Format("15:04:05"),

		QueryQueueItems: getSortedQueueEntries(c.queryQueue),

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

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode message: %v", err), http.StatusBadRequest)
		return
	}

	// validate
	if msg.SenderID == "" || msg.MessageID == "" || msg.Message == "" {
		http.Error(w, "senderID, messageID and message are required", http.StatusBadRequest)
		return
	}

	// set standard value
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().Format(time.RFC3339)
	}
	if msg.Status == "" {
		msg.Status = "received"
	}

	// Save message
	if err := RcvMessage(msg); err != nil {
		log.Printf("Failed to store message: %v", err)
		http.Error(w, fmt.Sprintf("failed to store message: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("message received"))
}

func (c *Client) chatDashboardHandler(w http.ResponseWriter, r *http.Request) {

	// Get messages for all contacts
	contactMessages, err := c.GetAllContactMessages()

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get contact messages: %v", err), http.StatusInternalServerError)
		return

	}

	log.Println("Contacts and messages:", contactMessages)
	chatDashboard(contactMessages, c.identity).Render(r.Context(), w)
}

// GetContactsFromQueue returns all contacts from priority queue
func (c *Client) GetContactsFromQueue() []shared.ClientIdentity {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	_, containerIDs := c.queryQueue.ToSlices()

	var contacts []shared.ClientIdentity

	// find corresponding
	for _, containerID := range containerIDs {
		if client, exists := c.clientManager.GetClient(containerID); exists {
			contacts = append(contacts, client)
		}
	}

	return contacts
}

func (c *Client) chatHistoryHandler(w http.ResponseWriter, r *http.Request) {

	// Parse selected contact from form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	selectedContact := r.FormValue("selected_contact")
	if selectedContact == "" {
		http.Error(w, "No contact selected", http.StatusBadRequest)
		return
	}

	log.Printf("Selected contact: %s", selectedContact)

	// Get only the messages for this specific contact
	messages, err := GetChatWithContact(c.identity.ContainerID, selectedContact)
	if err != nil {
		log.Printf("Failed to get messages for contact %s: %v", selectedContact, err)
		messages = []Message{}
	}

	// Get the contact name for display
	contactName := c.GetContactName(selectedContact)

	log.Printf("History with %s (%d messages)", contactName, len(messages))
	chatHistory(messages, c.identity, selectedContact, contactName).Render(r.Context(), w)
}

func (c *Client) sendMessageHandler(w http.ResponseWriter, r *http.Request) {

	// Formular-Daten parsen
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	targetContainerID := r.FormValue("contact")
	messageText := r.FormValue("message")

	if targetContainerID == "" || messageText == "" {
		http.Error(w, "Contact and message are required", http.StatusBadRequest)
		return
	}

	// Get target client to access hostname
	targetClient, exists := c.clientManager.GetClient(targetContainerID)
	if !exists {
		http.Error(w, "Target client not found", http.StatusBadRequest)
		return
	}

	// Use hostname for inter-container communication
	if targetClient.Hostname == "" {
		http.Error(w, "Target client hostname not available", http.StatusBadRequest)
		return
	}

	// Nachricht mit automatisch generierten Feldern erstellen
	msgID := generateMessageID()
	timestamp := time.Now().Format(time.RFC3339)
	sendMsg := Message{
		SenderID:   c.identity.ContainerID,
		ReceiverID: targetContainerID,
		MessageID:  msgID,
		Timestamp:  timestamp,
		Status:     "received", // Status for the receiving side
		Message:    messageText,
	}

	// JSON kodieren
	jsonData, err := json.Marshal(sendMsg)
	if err != nil {
		log.Printf("Failed to create JSON: %v", err)
		http.Error(w, "Failed to encode message", http.StatusInternalServerError)
		return
	}

	// Use target hostname for HTTP request with namespaced endpoint
	targetURL := fmt.Sprintf("http://%s:9090/chat/message", targetClient.Hostname)

	log.Printf("Sending message to %s (hostname: %s): %s", targetContainerID, targetClient.Hostname, sendMsg.Message)

	// Send message synchronously first
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send message to %s: %v", targetContainerID, err)
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Check if the send was successful
	if resp.StatusCode != http.StatusOK {
		log.Printf("Message send to %s failed with status %d", targetContainerID, resp.StatusCode)
		http.Error(w, fmt.Sprintf("Failed to send message: server returned status %d", resp.StatusCode), http.StatusBadGateway)
		return
	}

	log.Printf("Message sent successfully to %s with status: %s", targetContainerID, resp.Status)

	// Only save to database if send was successful
	saveMsg := Message{
		SenderID:   c.identity.ContainerID,
		ReceiverID: targetContainerID,
		MessageID:  msgID,
		Timestamp:  timestamp,
		Status:     "sent", // Status for our local copy
		Message:    messageText,
	}

	if err := RcvMessage(saveMsg); err != nil {
		log.Printf("Failed to save message to database: %v", err)
		http.Error(w, "Message sent but failed to save locally", http.StatusInternalServerError)
		return
	}

	// Return updated chat area for HTMX
	messages, err := GetChatWithContact(c.identity.ContainerID, targetContainerID)
	if err != nil {
		log.Printf("Failed to get messages for contact %s: %v", targetContainerID, err)
		messages = []Message{}
	}

	// Get the contact name for display
	contactName := c.GetContactName(targetContainerID)

	chatHistory(messages, c.identity, targetContainerID, contactName).Render(r.Context(), w)
}

type ContactMessages struct {
	Contact     string // Container ID for logic
	ContactName string // Display name for UI
	Messages    []Message
}

// Optimized function: get all contacts from DB with most recent message,
// then append contacts from queue that aren't already in the list
func (c *Client) GetAllContactMessages() ([]ContactMessages, error) {
	// Step 1: Get all contacts from database with their most recent message
	dbContacts, err := GetContactsWithLastMessage(c.identity.ContainerID)
	if err != nil {
		return nil, err
	}

	// Step 2: Create map for efficient lookup and result slice
	contactMap := make(map[string]ContactMessages)
	var result []ContactMessages

	// Add database contacts to map and result
	for _, dbContact := range dbContacts {
		var messages []Message
		if dbContact.LastMessage != nil {
			messages = []Message{*dbContact.LastMessage}
		}

		contactMsg := ContactMessages{
			Contact:     dbContact.ContainerID,
			ContactName: dbContact.Name,
			Messages:    messages,
		}
		contactMap[dbContact.ContainerID] = contactMsg
		result = append(result, contactMsg)
	}

	// Step 3: Add contacts from queue that aren't already in the database
	queueContacts := c.GetContactsFromQueue()
	for _, queueContact := range queueContacts {
		if _, exists := contactMap[queueContact.ContainerID]; !exists {
			// This contact is not in the database yet - add it
			err := AddContact(Contact{
				ContainerID: queueContact.ContainerID,
				Name:        queueContact.Name,
				Hostname:    queueContact.Hostname,
			})
			if err != nil {
				log.Printf("Failed to add contact to database: %v", err)
			}

			contactMsg := ContactMessages{
				Contact:     queueContact.ContainerID,
				ContactName: queueContact.Name,
				Messages:    []Message{}, // No messages yet
			}
			result = append(result, contactMsg)
		}
	}

	// Step 4: Sort contacts by most recent message timestamp
	sort.Slice(result, func(i, j int) bool {
		// Fall 1: Both contacts have messages
		if len(result[i].Messages) > 0 && len(result[j].Messages) > 0 {
			// Compare the latest message timestamp
			latestI := result[i].Messages[len(result[i].Messages)-1].Timestamp
			latestJ := result[j].Messages[len(result[j].Messages)-1].Timestamp

			if latestI != latestJ {
				return latestI > latestJ // newest first
			}
			return result[i].Contact < result[j].Contact // alphabetical for same timestamp
		}

		// Fall 2: Only one has messages - that one comes first
		if len(result[i].Messages) > 0 {
			return true
		}
		if len(result[j].Messages) > 0 {
			return false
		}

		// Fall 3: No messages - sort alphabetically
		return result[i].Contact < result[j].Contact
	})

	return result, nil
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

	// Get target client to access hostname
	targetClient, exists := c.clientManager.GetClient(containerID)
	if !exists {
		log.Printf("Target client %s not found in client manager", containerID)
		return
	}

	// Use hostname for inter-container communication
	if targetClient.Hostname == "" {
		log.Printf("Target client %s has no hostname available", containerID)
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

	// Make POST request to the client's contacts endpoint using hostname
	targetURL := fmt.Sprintf("http://%s:9090/contacts", targetClient.Hostname)
	resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to query contacts from %s (hostname: %s): %v", containerID, targetClient.Hostname, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Contacts query to %s (hostname: %s) returned status %d", containerID, targetClient.Hostname, resp.StatusCode)
		return
	}

	// Parse response
	var receivedContacts []shared.ClientIdentity
	if err := json.NewDecoder(resp.Body).Decode(&receivedContacts); err != nil {
		log.Printf("Failed to decode contacts response from %s: %v", containerID, err)
		return
	}

	c.addNewContacts(receivedContacts)

	// Reinsert with updated priority
	c.sequenceCounter++
	c.queryQueue.Insert(c.sequenceCounter, containerID)
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

// Returns the contact list partial for periodic updates
func (c *Client) chatContactsHandler(w http.ResponseWriter, r *http.Request) {
	contactMessages, err := c.GetAllContactMessages()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get contact messages: %v", err), http.StatusInternalServerError)
		return
	}

	// Get selected contact from query parameter if any
	selectedContact := r.URL.Query().Get("selected")

	contactListPartial(contactMessages, selectedContact).Render(r.Context(), w)
}

// Helper functions for name resolution

// GetContactName resolves a container ID to a display name
func (c *Client) GetContactName(containerID string) string {
	if client, exists := c.clientManager.GetClient(containerID); exists {
		return client.Name
	}
	// If not found in client manager, return the container ID as fallback
	return containerID
}

// GetContactsWithNames returns contacts with resolved names for display
func (c *Client) GetContactsWithNames() []struct {
	ContainerID string
	Name        string
} {
	var result []struct {
		ContainerID string
		Name        string
	}

	contacts := c.GetContactsFromQueue()
	for _, contact := range contacts {
		result = append(result, struct {
			ContainerID string
			Name        string
		}{
			ContainerID: contact.ContainerID,
			Name:        contact.Name,
		})
	}

	return result
}

func (c *Client) queryQueueHandler(w http.ResponseWriter, r *http.Request) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	items := getSortedQueueEntries(c.queryQueue)
	QueryQueueComponent(items).Render(r.Context(), w)
}

func getSortedQueueEntries(pq *shared.PriorityQueue[int64, string]) []shared.Entry[int64, string] {
	return pq.GetSortedEntries()
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
	go client.contactExpansionWorker(10 * time.Second)

	// Register shared client manager handlers
	client.clientManagerHandlers.RegisterHandlers()

	// Register HTTP handlers
	http.HandleFunc("/header-status", client.headerStatusHandler)
	http.HandleFunc("/connection-status", client.connectionStatusHandler)
	http.HandleFunc("/connect", client.connectHandler)
	http.HandleFunc("/disconnect", client.disconnectHandler)
	http.HandleFunc("/refresh", client.refreshHandler)
	http.HandleFunc("/alive", client.aliveHandler)

	http.HandleFunc("/contacts", client.contactsHandler)
	http.HandleFunc("/query-queue", client.queryQueueHandler)

	// Chat Endpoints - all namespaced under /chat/
	http.HandleFunc("/chat/message", client.messageHandler)          // Receive messages
	http.HandleFunc("/chat/dashboard", client.chatDashboardHandler)  // Show chat dashboard
	http.HandleFunc("/chat/conversation", client.chatHistoryHandler) // Show chat conversation
	http.HandleFunc("/chat/send", client.sendMessageHandler)         // Send messages
	http.HandleFunc("/chat/contacts", client.chatContactsHandler)    // Contact list partial

	// Status handler - moved to end to avoid catching other paths
	http.HandleFunc("/", client.statusHandler)

	log.Printf("Starting client '%s' on port %s", client.GetClientName(), port)
	log.Printf("Bootstrap server URL: %s", bootstrapURL)
	log.Printf("Container ID: %s", client.GetIdentity().ContainerID)
	log.Printf("Auto-connect: %v (retry interval: %v, max retries: %d)", autoConnect, retryInterval, maxRetries)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
