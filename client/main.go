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
)

type ClientIdentity struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
	Source      string `json:"source"`
}

type Client struct {
	identity        ClientIdentity
	bootstrapURL    string
	connected       bool
	names           []string
	lastUpdate      time.Time
	connectionError string
	mutex           sync.RWMutex
	autoConnect     bool
	retryInterval   time.Duration
	maxRetries      int
	shutdownChan    chan bool
}

// DiscoverClientIdentity discovers the client's identity from environment and system
func DiscoverClientIdentity() ClientIdentity {
	identity := ClientIdentity{}

	// Try CLIENT_NAME environment variable first (set by bootstrap)
	if name := os.Getenv("CLIENT_NAME"); name != "" {
		identity.Name = name
		identity.Source = "CLIENT_NAME environment variable"
	} else {
		// Fallback to hostname-based name
		if hostname, err := os.Hostname(); err == nil {
			identity.Name = fmt.Sprintf("client-%s", hostname[:8]) // Use first 8 chars of container ID
			identity.Source = "hostname fallback"
		} else {
			// Last resort default
			identity.Name = "default-client"
			identity.Source = "default fallback"
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

	log.Printf("Discovered client identity: Name='%s', ContainerID='%s', Source='%s'",
		identity.Name, identity.ContainerID, identity.Source)

	return &Client{
		identity:      identity,
		bootstrapURL:  bootstrapURL,
		connected:     false,
		names:         []string{},
		lastUpdate:    time.Now(),
		autoConnect:   autoConnect,
		retryInterval: retryInterval,
		maxRetries:    maxRetries,
		shutdownChan:  make(chan bool, 1),
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

	url := fmt.Sprintf("%s/connect", c.bootstrapURL)
	resp, err := http.Post(url, "text/plain", bytes.NewBufferString(c.identity.ContainerID))
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

	var names []string
	if err := json.NewDecoder(resp.Body).Decode(&names); err != nil {
		c.connectionError = fmt.Sprintf("Failed to decode response: %v", err)
		c.connected = false
		return err
	}

	c.names = names
	c.connected = true
	c.connectionError = ""
	c.lastUpdate = time.Now()
	log.Printf("Connected to bootstrap server. Received %d names", len(names))
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
	c.names = []string{}
	c.connectionError = ""
	c.lastUpdate = time.Now()
	log.Printf("Disconnected from bootstrap server")
	return nil
}

// autoConnectWorker handles automatic connection with retry logic
func (c *Client) autoConnectWorker() {
	if !c.autoConnect {
		return
	}

	log.Printf("Starting auto-connect worker (retry interval: %v, max retries: %d)", c.retryInterval, c.maxRetries)

	// Initial connection attempt with delay to let server start
	time.Sleep(2 * time.Second)

	retryCount := 0
	ticker := time.NewTicker(c.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.shutdownChan:
			log.Printf("Auto-connect worker shutting down")
			return
		case <-ticker.C:
			c.mutex.RLock()
			isConnected := c.connected
			c.mutex.RUnlock()

			if !isConnected {
				if c.maxRetries > 0 && retryCount >= c.maxRetries {
					log.Printf("Max retries (%d) reached, stopping auto-connect attempts", c.maxRetries)
					return
				}

				log.Printf("Attempting auto-connect (attempt %d)...", retryCount+1)
				if err := c.connect(); err != nil {
					retryCount++
					log.Printf("Auto-connect failed: %v (retry %d)", err, retryCount)
				} else {
					retryCount = 0 // Reset retry count on successful connection
					log.Printf("Auto-connect successful!")
				}
			} else {
				retryCount = 0 // Reset retry count if already connected
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
		ClientName:      c.identity.Name,
		ContainerID:     c.identity.ContainerID,
		IdentitySource:  c.identity.Source,
		BootstrapURL:    c.bootstrapURL,
		Connected:       c.connected,
		Names:           append([]string{}, c.names...), // Copy slice
		NamesCount:      len(c.names),
		LastUpdate:      c.lastUpdate.Format("15:04:05"),
		ConnectionError: c.connectionError,
		CurrentTime:     time.Now().Format("15:04:05"),
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

	log.Printf("Starting client '%s' on port %s", client.GetClientName(), port)
	log.Printf("Bootstrap server URL: %s", bootstrapURL)
	log.Printf("Container ID: %s", client.GetIdentity().ContainerID)
	log.Printf("Auto-connect: %v (retry interval: %v, max retries: %d)", autoConnect, retryInterval, maxRetries)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
