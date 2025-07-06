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
	"strings"
	"sync"
	"time"
)

type ClientIdentity struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
}

type Client struct {
	identity         ClientIdentity
	bootstrapURL     string
	connected        bool
	clientIdentities []ClientIdentity
	lastUpdate       time.Time
	connectionError  string
	mutex            sync.RWMutex
	autoConnect      bool
	retryInterval    time.Duration
	maxRetries       int
	shutdownChan     chan bool
}

// DiscoverClientIdentity discovers the client's identity from environment and system
func DiscoverClientIdentity() ClientIdentity {
	identity := ClientIdentity{}

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

	return &Client{
		identity:         identity,
		bootstrapURL:     bootstrapURL,
		connected:        false,
		clientIdentities: []ClientIdentity{},
		lastUpdate:       time.Now(),
		autoConnect:      autoConnect,
		retryInterval:    retryInterval,
		maxRetries:       maxRetries,
		shutdownChan:     make(chan bool, 1),
	}
}

func (c *Client) GetClientName() string {
	return c.identity.Name
}

func (c *Client) GetIdentity() ClientIdentity {
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

	var clientIdentities []ClientIdentity
	if err := json.NewDecoder(resp.Body).Decode(&clientIdentities); err != nil {
		c.connectionError = fmt.Sprintf("Failed to decode response: %v", err)
		c.connected = false
		return err
	}

	c.clientIdentities = clientIdentities
	c.connected = true
	c.connectionError = ""
	c.lastUpdate = time.Now()
	log.Printf("Connected to bootstrap server. Received %d client identities", len(clientIdentities))
	return nil
}

func (c *Client) disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.connected {
		return nil
	}

	url := fmt.Sprintf("%s/delete", c.bootstrapURL)
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString(c.identity.Name))
	if err != nil {
		c.connectionError = fmt.Sprintf("Failed to disconnect: %v", err)
		return err
	}
	defer resp.Body.Close()

	c.connected = false
	c.clientIdentities = []ClientIdentity{}
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

// Shutdown gracefully stops the auto-connect worker
func (c *Client) Shutdown() {
	if c.autoConnect {
		close(c.shutdownChan)
	}
}

func (c *Client) getStatus() ClientStatus {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	status := ClientStatus{
		ClientName:       c.identity.Name,
		ContainerID:      c.identity.ContainerID,
		BootstrapURL:     c.bootstrapURL,
		Connected:        c.connected,
		ClientIdentities: append([]ClientIdentity{}, c.clientIdentities...), // Copy slice
		NamesCount:       len(c.clientIdentities),
		LastUpdate:       c.lastUpdate.Format("15:04:05"),
		ConnectionError:  c.connectionError,
		CurrentTime:      time.Now().Format("15:04:05"),
	}

	return status
}

func (c *Client) connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.connect()
	status := c.getStatus()
	clientContent(status).Render(r.Context(), w)
}

func (c *Client) disconnectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.disconnect()
	status := c.getStatus()
	clientContent(status).Render(r.Context(), w)
}

func (c *Client) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if c.connected {
		c.connect() // Reconnect to refresh names
	}
	status := c.getStatus()
	clientContent(status).Render(r.Context(), w)
}

func (c *Client) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := c.getStatus()
	clientPage(status).Render(r.Context(), w)
}

func (c *Client) statusDataHandler(w http.ResponseWriter, r *http.Request) {
	status := c.getStatus()
	clientContent(status).Render(r.Context(), w)
}

func (c *Client) aliveHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// opens chat
func (c *Client) chatHandler(w http.ResponseWriter, r *http.Request) {
	containerID := strings.TrimPrefix(r.URL.Path, "/chat/")
	chatKey := makeChatKey(c.identity.ContainerID, containerID)

	var peer *ClientIdentity
	for _, id := range c.clientIdentities {
		if id.ContainerID == containerID {
			peer = &id
			break
		}
	}
	if peer == nil {
		http.NotFound(w, r)
		return
	}

	chatMutex.RLock()
	messages := chatStorage[chatKey]
	chatMutex.RUnlock()

	chatBox(*peer, messages).Render(r.Context(), w)
}

// handles received messages
func (c *Client) chatSendHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	containerID := strings.TrimPrefix(r.URL.Path, "/chat/")
	containerID = strings.TrimSuffix(containerID, "/send")
	msgText := r.FormValue("message")

	msg := Message{
		From:    c.identity.Name,
		To:      containerID,
		Content: msgText,
		Time:    time.Now(),
	}

	chatKey := makeChatKey(c.identity.ContainerID, containerID)

	chatMutex.Lock()
	chatStorage[chatKey] = append(chatStorage[chatKey], msg)
	chatMutex.Unlock()

	// Nur neue Nachricht als HTML zurückgeben
	fmt.Fprintf(w, "<div><strong>%s:</strong> %s</div>", msg.From, msg.Content)
}

type Message struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
	Time    time.Time
}

func makeChatKey(a, b string) string {
	if a < b {
		return a + "::" + b
	}
	return b + "::" + a
}

var chatStorage = make(map[string][]Message) // key = containerID
var chatMutex sync.RWMutex

func main() {
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

	// Start auto-connect worker if enabled
	if autoConnect {
		go client.autoConnectWorker()
	}

	http.HandleFunc("/", client.statusHandler)
	http.HandleFunc("/status-data", client.statusDataHandler)
	http.HandleFunc("/connect", client.connectHandler)
	http.HandleFunc("/disconnect", client.disconnectHandler)
	http.HandleFunc("/refresh", client.refreshHandler)
	http.HandleFunc("/alive", client.aliveHandler)

	http.HandleFunc("/chat/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/send") {
			client.chatSendHandler(w, r)
		} else {
			client.chatHandler(w, r)
		}
	})

	log.Printf("Starting client '%s' on port %s", client.GetClientName(), port)
	log.Printf("Bootstrap server URL: %s", bootstrapURL)
	log.Printf("Container ID: %s", client.GetIdentity().ContainerID)
	log.Printf("Auto-connect: %v (retry interval: %v, max retries: %d)", autoConnect, retryInterval, maxRetries)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
