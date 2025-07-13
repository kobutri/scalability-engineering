# Scalability Engineering Bootstrap Server

A high-performance bootstrap server for distributed systems with dynamic client management and service discovery capabilities.

## Overview

This project implements a bootstrap server that maintains a registry of connected clients and provides them with a subset of known peers for distributed system connectivity. It includes a web management interface, automated client lifecycle management through Docker, and a comprehensive benchmarking tool.

## Features

- **Service Registry**: Thread-safe storage of client names with O(1) operations
- **Peer Discovery**: Returns random subsets of connected clients for distributed connectivity
- **Health Monitoring**: Automatic health checks with configurable intervals and timeouts
- **Web Management**: Real-time dashboard for monitoring and configuration
- **Docker Integration**: Automated client container creation and management
- **Persistent Storage**: Data persistence across server restarts
- **Configurable Parameters**: Runtime configuration of timeouts, intervals, and subset sizes
- **Benchmark Tool**: Comprehensive load testing with real-time metrics and interactive dashboard

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Build and start the entire system
./build.sh
docker-compose up
```

Access the web interface at `http://localhost:8080/status-page`

### Start with Benchmark Tool

```bash
# Start bootstrap server and benchmark tool
docker-compose --profile benchmark up
```

Access the benchmark dashboard at `http://localhost:8090`

### Development Mode

```bash
# Start with hot reloading
./dev.sh
```

### Manual Setup

```bash
# Start the bootstrap server
cd bootstrap
go run .

# In another terminal, start a client
cd client
CLIENT_NAME=my-client go run .

# In another terminal, start the benchmark tool
cd benchmark
./run.sh
```

## API Endpoints

### Core API
- `POST /connect` - Register a client and receive peer list
- `POST /delete` - Unregister a client
- `GET /status` - Get server statistics (JSON)
- `GET /` - Health check endpoint

### Management Interface
- `GET /status-page` - Web dashboard
- `POST /add-client` - Create a new Docker client container
- `POST /remove-client` - Remove a client container
- `POST /bulk-create-clients` - Create multiple clients at once
- `GET /chat/dashboard` - Chat Interface

## Configuration

Configure via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HEALTH_TIMEOUT` | `5s` | Timeout for client health checks |
| `MAX_AGE` | `10s` | Maximum age before removing from active list |
| `MIN_AGE` | `2s` | Minimum age before health checking |
| `HEALTH_CHECK_INTERVAL` | `100ms` | How often to perform health checks |
| `CLEANUP_INTERVAL` | `1s` | How often to clean up expired entries |
| `SUBSET_SIZE` | `5` | Number of peers returned to clients |
| `DATA_DIR` | `../data` | Directory for persistent storage |

## Benchmark Tool

The benchmark tool provides comprehensive load testing for the bootstrap server with:

- **Real-time Metrics**: Throughput, error rate, P99 latency
- **Interactive Dashboard**: HTMX-powered web interface
- **Configurable Load**: Adjustable client count and operations per second
- **Worker Pool**: Concurrent operations with configurable thread count
- **Live Graphs**: Visual monitoring of performance metrics

### Launch Benchmark Tool

```bash
# Run basic benchmark (http://localhost:8090/)
cd benchmark
docker compose -f docker-compose.benchmark.yml up -d
```

### Benchmark Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_CLIENTS` | 100 | Maximum number of concurrent clients |
| `THROTTLE_OPS` | 10 | Operations per second throttling |
| `BOOTSTRAP_URL` | `http://localhost:8080` | Bootstrap server URL |
| `FIXED_HOSTNAME` | `benchmark-host` | Fixed hostname for generated clients |
| `WORKER_COUNT` | 10 | Number of worker threads |
| `METRICS_INTERVAL` | 1s | Metrics calculation interval |
| `LATENCY_BUFFER_SIZE` | 1000 | Buffer size for latency calculations |
| `PORT` | 8090 | Dashboard port |

## Project Structure

```
scalability-engineering/
├── bootstrap/                          # Main bootstrap server
│   ├── main.go                         # Server implementation
│   ├── hashset.go                      # Thread-safe set data structure
│   ├── priority_queue.go               # Priority queue for time-based operations
│   ├── dashboard.templ                 # htmx template for the bootstrap dashboard components
│   └── docker_client.go                # Docker container management
├── client/                             # Example client implementation
│   ├── main.go                         # Client with web UI
│   ├── connection_control.templ        # Control buttons for connection / disconnecting the client
│   ├── dashboard.templ                 # Dashboard components for the client UI
│   ├── db.go                           # Chat implementation
│   └── client.templ                    # Client dashboard template
├── benchmark/                          # Benchmark tool
│   ├── main.go                         # Benchmark implementation
│   ├── handlers.go                     # HTTP handlers
│   ├── dashboard.templ                 # Dashboard template
│   ├── run.sh                          # Run script
│   └── README.md                       # Benchmark documentation
├── shared/                             # Shared components
│   ├── client_manager.go               # Client management
│   ├── hashset.go                      # Thread-safe data structures
│   ├── client_manager_component.templ  # Shared UI components
│   ├── client_manager_handlers.go      # Handler functions for the client manager
│   └── priority_queue.go               # Priority queue implementation
└── data/                               # Persistent storage directory
```

## Usage Examples

### Register a Client

```bash
curl -X POST http://localhost:8080/connect -d "my-client-name"
```

Response: JSON array of peer client names

### Health Check

```bash
curl http://localhost:8080/
```

### Create Multiple Clients

```bash
curl -X POST http://localhost:8080/bulk-create-clients -d "count=10"
```

## Architecture

The bootstrap server uses:

- **Thread-safe HashSet**: For O(1) client registration/lookup
- **Priority Queue**: For time-based health check scheduling  
- **Docker API**: For dynamic client container management
- **Fine-grained Locking**: Minimal contention under high concurrency
- **Persistent Storage**: JSON-based data persistence

The benchmark tool provides:

- **Worker Pool**: Concurrent operations with configurable parallelism
- **Throttling**: Rate-limited operations for controlled load testing
- **Metrics Collection**: Real-time performance monitoring
- **Interactive Dashboard**: Web-based control and visualization

## License

This implementation is provided as-is for educational and practical use. 

## Prototyping Requirements

1) **Managed State:** Chat Messages, Registered ClientIdentities, QueryQueues, Live Data
2) **Scale vertically and horizontally:** Network scales with additional peers, Bootstrap Server can limit the  amount of requests per second
3) **No Overloading at full scale:** Once a client is registered, the bootstrap server no longer needs to be involved in contact establishment. Clients only share a controlled number of peer contacts with each other. Alive requests to the Bootstrap Server are also limited.
4) **Additional Strategies:** 
**Sharding** (peers store their own chat messages, clientIdentities, queues etc.), 
**Priority Queue** (checking for alive peers and only using them for requests via the query queue), 
**Replication** (clientIdentities can be obtained through other peers), 
**Eventual Consistency** (All peers will eventually get to know all the other peers in the network, through the queryQueue and the expansionHandler())