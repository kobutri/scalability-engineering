# Scalability Engineering Bootstrap Server

A high-performance bootstrap server for distributed systems with dynamic client management and service discovery capabilities.

## Overview

This project implements a bootstrap server that maintains a registry of connected clients and provides them with a subset of known peers for distributed system connectivity. It includes a web management interface and automated client lifecycle management through Docker.

## Features

- **Service Registry**: Thread-safe storage of client names with O(1) operations
- **Peer Discovery**: Returns random subsets of connected clients for distributed connectivity
- **Health Monitoring**: Automatic health checks with configurable intervals and timeouts
- **Web Management**: Real-time dashboard for monitoring and configuration
- **Docker Integration**: Automated client container creation and management
- **Persistent Storage**: Data persistence across server restarts
- **Configurable Parameters**: Runtime configuration of timeouts, intervals, and subset sizes

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Build and start the entire system
./build.sh
docker-compose up
```

Access the web interface at `http://localhost:8080/status-page`

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

## Project Structure

```
scalability-engineering/
├── bootstrap/          # Main bootstrap server
│   ├── main.go        # Server implementation
│   ├── hashset.go     # Thread-safe set data structure
│   ├── priority_queue.go # Priority queue for time-based operations
│   └── docker_client.go # Docker container management
├── client/            # Example client implementation
│   ├── main.go       # Client with web UI
│   └── client.templ  # Client dashboard template
└── data/             # Persistent storage directory
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

## Development

### Running Tests

```bash
cd bootstrap
go test -v
go test -race  # Test for race conditions
```

### Building Docker Images

```bash
./build.sh  # Builds both bootstrap and client images
```

### Docker Compose Files

- `docker-compose.yml` - Production configuration
- `docker-compose.dev.yml` - Development with hot reloading

## Architecture

The bootstrap server uses:

- **Thread-safe HashSet**: For O(1) client registration/lookup
- **Priority Queue**: For time-based health check scheduling  
- **Docker API**: For dynamic client container management
- **Fine-grained Locking**: Minimal contention under high concurrency
- **Persistent Storage**: JSON-based data persistence

## License

This implementation is provided as-is for educational and practical use. 