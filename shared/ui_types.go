package shared

import (
	"fmt"
	"time"
)

// StatusData represents the unified status data structure for both bootstrap and client
type StatusData struct {
	// Service identification
	ServiceType  string // "bootstrap" or "client"
	ServiceName  string
	ContainerID  string
	BootstrapURL string

	// Connection status (for clients)
	Connected       bool
	ConnectionError string
	LastUpdate      string

	// Client management data
	HashSetSize       int                     // Number of clients in hashmap
	PriorityQueueSize int                     // Number of clients in priority queue
	ClientEntries     []Entry[string, string] // Contains ID-Name pairs
	QueueItems        []QueueItem             // Priority queue items

	// Configuration and workers
	Config         ServiceConfig
	WorkersRunning bool

	// Time and uptime
	UptimeSeconds   float64
	UptimeFormatted string
	CurrentTime     string

	// Service-specific data
	ClientContainers []ClientContainer // For bootstrap (Docker management)
	ClientIdentities []ClientIdentity  // For client (bootstrap peers)
}

// QueueItem is defined in client_manager.go

// ServiceConfig holds configuration for client management
type ServiceConfig struct {
	Timeout             string
	MaxAge              string
	MinAge              string
	HealthCheckInterval string
	CleanupInterval     string
	SubsetSize          int
}

// ClientContainer represents a Docker container for display
type ClientContainer struct {
	Name     string
	Status   string
	ID       string
	HostPort string
	WebURL   string
}

// UIConfig holds UI configuration options
type UIConfig struct {
	Title                 string
	ShowClientManagement  bool     // Whether to show Docker container management
	ShowConnectionControl bool     // Whether to show connect/disconnect controls
	RefreshInterval       string   // e.g., "2s" or "3s"
	Features              []string // List of enabled features
}

// GetDefaultUIConfig returns default UI configuration for a service type
func GetDefaultUIConfig(serviceType string) UIConfig {
	switch serviceType {
	case "bootstrap":
		return UIConfig{
			Title:                 "Bootstrap Server Control Panel",
			ShowClientManagement:  true,
			ShowConnectionControl: false,
			RefreshInterval:       "2s",
			Features:              []string{"stats", "config", "clients", "queue", "containers"},
		}
	case "client":
		return UIConfig{
			Title:                 "Client Control Panel",
			ShowClientManagement:  false, // Will be enabled with new shared UI
			ShowConnectionControl: true,
			RefreshInterval:       "3s",
			Features:              []string{"stats", "config", "clients", "queue", "connection", "chat"},
		}
	default:
		return UIConfig{
			Title:                 "Service Control Panel",
			ShowClientManagement:  false,
			ShowConnectionControl: false,
			RefreshInterval:       "3s",
			Features:              []string{"stats"},
		}
	}
}

// FormatDuration formats a duration for display in the UI
func FormatDuration(duration time.Duration) string {
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
