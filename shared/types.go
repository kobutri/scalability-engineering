package shared

// ClientManagerData contains only the data needed by the client manager component
type ClientManagerData struct {
	HashSetSize       int                     // Number of clients in hashmap
	PriorityQueueSize int                     // Number of clients in priority queue
	ClientEntries     []Entry[string, string] // Contains ID-Name pairs
	QueueItems        []QueueItem             // Priority queue items
	Config            ServiceConfig           // Configuration settings
	WorkersRunning    bool                    // Whether workers are running
}
