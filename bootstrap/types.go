package main

import (
	"shared"
)

// BootstrapData contains all data needed for the bootstrap service
type BootstrapData struct {
	// Service identification
	ServiceName     string
	ContainerID     string
	UptimeSeconds   float64
	UptimeFormatted string
	CurrentTime     string

	// Client manager component data
	ClientManagerData shared.ClientManagerData

	// Bootstrap-specific data
	ClientContainers []shared.ClientContainer
}
