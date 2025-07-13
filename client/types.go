package main

import (
	"shared"
)

// ClientData contains static data needed for the client service dashboard
// Dynamic data like header info, client manager data, and connection status
// are now handled by separate polling endpoints
type ClientData struct {
	// Service identification (used for initial header and static elements)
	ServiceName     string
	ContainerID     string
	UptimeSeconds   float64
	UptimeFormatted string
	CurrentTime     string

	// Static client manager data for initial load
	ClientManagerData shared.ClientManagerData

	// Static connection data for initial load
	BootstrapURL    string
	Connected       bool
	ConnectionError string
	LastUpdate      string

	QueryQueueItems []shared.Entry[int64, string]
}
