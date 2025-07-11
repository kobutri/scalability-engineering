package main

import (
	"shared"
)

// ClientData contains all data needed for the client service
type ClientData struct {
	// Service identification
	ServiceName     string
	ContainerID     string
	UptimeSeconds   float64
	UptimeFormatted string
	CurrentTime     string

	// Client manager component data
	ClientManagerData shared.ClientManagerData

	// Client-specific data
	BootstrapURL    string
	Connected       bool
	ConnectionError string
	LastUpdate      string
}
