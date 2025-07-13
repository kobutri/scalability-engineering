package shared

// ClientEntry represents a client entry for display in the client manager
type ClientEntry struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
}

// ClientManagerData contains only the data needed by the client manager component
type ClientManagerData struct {
	HashSetSize       int           // Number of clients in hashmap
	PriorityQueueSize int           // Number of clients in priority queue
	ClientEntries     []ClientEntry // Contains client information
	QueueItems        []QueueItem   // Priority queue items
	Config            ServiceConfig // Configuration settings
	WorkersRunning    bool          // Whether workers are running
}
