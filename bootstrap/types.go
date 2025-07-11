package main

import (
	"shared"
)

// BootstrapData contains static data needed for the bootstrap service dashboard
// Dynamic data like header info, client manager data, and client containers
// are now handled by separate polling endpoints
type BootstrapData struct {
	// Service identification (used for initial header and static elements)
	ServiceName     string
	ContainerID     string
	UptimeSeconds   float64
	UptimeFormatted string
	CurrentTime     string

	// Static client manager data for initial load
	ClientManagerData shared.ClientManagerData

	// Static client containers data for initial load
	ClientContainers []ClientContainer
}
