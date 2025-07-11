package shared

import (
	"fmt"
	"time"
)

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
