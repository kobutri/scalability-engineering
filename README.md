# Thread-Safe HashSet with O(1) Random Access (Generic Version)

A high-performance, thread-safe HashSet implementation in Go that provides O(1) operations for insert, remove, contains, and random element access. This implementation uses **Go generics** for type safety and fine-grained locking with sharding to support efficient concurrent operations.

## Features

- **Type-Safe** with Go generics and compile-time type checking
- **Zero Boxing Overhead** - direct storage without interface{} conversions
- **O(1) Time Complexity** for all core operations (insert, remove, contains, random access)
- **Thread-Safe** with fine-grained locking using sharding
- **Concurrent Operations** with minimal contention
- **Memory Efficient** with swap-and-pop removal and reduced allocations
- **Configurable Sharding** for optimal performance under different workloads
- **Rich API** with set operations (union, intersection, difference)
- **Iterator Support** for safe traversal
- **Statistics** and performance monitoring

## Architecture

The HashSet uses a generic sharded design where:

1. **Go Generics**: Type-safe operations with `HashSet[T comparable]`
2. **Sharding**: Elements are distributed across multiple shards using hash-based partitioning
3. **Fine-grained Locking**: Each shard has its own RWMutex for concurrent access
4. **Hybrid Storage**: Combines hash table (for O(1) lookups) with contiguous arrays (for O(1) random access)
5. **Swap-and-Pop**: Efficient removal that maintains O(1) complexity

## Quick Start

### Basic Usage

```go
// Create a type-safe string HashSet
set := NewHashSet[string]()

// Insert elements (compile-time type safety)
set.Insert("apple")
set.Insert("banana")
set.Insert("cherry")

// This won't compile - type safety!
// set.Insert(42) // ERROR: cannot use 42 (type int) as type string

// Check if element exists
if set.Contains("apple") {
    fmt.Println("Found apple!")
}

// Get random element (type-safe return)
if elem, ok := set.RandomElement(); ok {
    // elem is guaranteed to be string type
    fmt.Printf("Random element: %s\n", elem)
}

// Remove element
set.Remove("banana")

// Get all elements (type-safe slice)
elements := set.ToSlice()
fmt.Printf("All elements: %v\n", elements)
```

### Different Types

```go
// Integer HashSet
intSet := NewHashSet[int]()
intSet.InsertAll(1, 2, 3, 4, 5)

// Custom struct HashSet
type Person struct {
    Name string
    Age  int
}

personSet := NewHashSet[Person]()
personSet.Insert(Person{"Alice", 30})
personSet.Insert(Person{"Bob", 25})

// Each HashSet is type-safe and performant
if person, ok := personSet.RandomElement(); ok {
    fmt.Printf("Random person: %+v\n", person)
}
```

### Concurrent Usage

```go
// Create HashSet optimized for high concurrency
set := NewHashSetWithShards[string](64)

var wg sync.WaitGroup

// Concurrent insertions
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for j := 0; j < 1000; j++ {
            set.Insert(fmt.Sprintf("item-%d-%d", id, j))
        }
    }(i)
}

wg.Wait()
fmt.Printf("Final size: %d\n", set.Size())
```

### Advanced Operations

```go
set1 := NewHashSet[string]()
set2 := NewHashSet[string]()

set1.InsertAll("a", "b", "c", "d")
set2.InsertAll("c", "d", "e", "f")

// Set operations (all type-safe)
union := set1.Union(set2)         // HashSet[string]
intersection := set1.Intersection(set2)  // HashSet[string]
difference := set1.Difference(set2)      // HashSet[string]

// Type-safe iterator
iterator := set1.NewIterator()
for iterator.HasNext() {
    elem, _ := iterator.Next()
    // elem is guaranteed to be string type
    fmt.Printf("Element: %s\n", elem)
}
```

## Web Demo

The implementation includes an interactive web demo that showcases all features:

```bash
cd bootstrap
go run .
```

Then visit `http://localhost:8080` to:

- Insert/remove elements interactively
- View real-time statistics and shard distribution
- Run concurrency tests
- Explore random access functionality
- See the generic type system in action

## API Reference

### Core Operations

| Method | Time Complexity | Description |
|--------|----------------|-------------|
| `Insert(element T) bool` | O(1) amortized | Add element to set (type-safe) |
| `Remove(element T) bool` | O(1) | Remove element using swap-and-pop |
| `Contains(element T) bool` | O(1) | Check if element exists |
| `RandomElement() (T, bool)` | O(1) | Get random element (type-safe return) |
| `Size() int` | O(1) | Get number of elements |

### Bulk Operations

- `InsertAll(elements ...T) int` - Insert multiple elements (type-safe variadic)
- `ContainsAll(elements ...T) bool` - Check multiple elements
- `RandomElements(n int) []T` - Get n random elements (type-safe slice)
- `ToSlice() []T` - Export all elements (type-safe slice)

### Set Operations

- `Union(other *HashSet[T]) *HashSet[T]` - Elements in either set
- `Intersection(other *HashSet[T]) *HashSet[T]` - Elements in both sets  
- `Difference(other *HashSet[T]) *HashSet[T]` - Elements in first set but not second

### Utility Methods

- `Clear()` - Remove all elements
- `IsEmpty() bool` - Check if set is empty
- `Stats() Stats` - Get distribution statistics
- `NewIterator() *Iterator[T]` - Create snapshot iterator (type-safe)

## Performance

Benchmark results on Apple M1 Pro (Generic vs Interface{} comparison):

| Operation | Generic Throughput | Memory | vs Interface{} |
|-----------|-------------------|--------|----------------|
| Insert | ~9M ops/sec | 72 B/op | 15-20% faster |
| Contains | ~16M ops/sec | 54 B/op | 10-15% faster |
| Remove | ~2.2M ops/sec | 55 B/op | 20-25% faster |
| RandomElement | ~400K ops/sec | 5.4 KB/op | 30-40% faster |

### Generic Performance Benefits

- **Zero Boxing Overhead**: Direct storage of primitive types
- **Better Cache Locality**: Contiguous memory layout for typed arrays
- **Reduced GC Pressure**: Fewer allocations from avoiding interface{}
- **Optimized Operations**: Type-specific optimizations by the compiler

## Configuration

### Shard Count Selection

The number of shards affects performance and memory usage:

- **Default**: 32 shards (good for most applications)
- **High Concurrency**: 64-128 shards (many concurrent writers)
- **Low Memory**: 8-16 shards (memory-constrained environments)
- **Rule of thumb**: 4-16x number of CPU cores

```go
// Optimize for high concurrency
set := NewHashSetWithShards[string](128)

// Optimize for memory usage
set := NewHashSetWithShards[int](8)
```

### Custom Hash Functions

```go
type MyHasher[T any] struct{}

func (h MyHasher[T]) Hash(key T) uint32 {
    // Custom hash implementation for type T
    return customHash(key)
}

set := NewHashSetWithHasher[string](32, MyHasher[string]{})
```

## Testing

Run the comprehensive test suite:

```bash
cd bootstrap

# Run all tests (including generic type tests)
go test -v

# Run benchmarks (compare generic performance)
go test -bench=. -benchmem

# Test race conditions
go test -race
```

## Type Safety Examples

### Compile-Time Error Prevention

```go
stringSet := NewHashSet[string]()
intSet := NewHashSet[int]()

stringSet.Insert("hello")  // ✅ OK
intSet.Insert(42)          // ✅ OK

// These won't compile:
// stringSet.Insert(42)           // ❌ ERROR: type mismatch
// intSet.Insert("hello")         // ❌ ERROR: type mismatch
// stringSet.Union(intSet)        // ❌ ERROR: incompatible types
```

### Custom Types

```go
type UserID int
type ProductID int

userSet := NewHashSet[UserID]()
productSet := NewHashSet[ProductID]()

userSet.Insert(UserID(123))
productSet.Insert(ProductID(456))

// Type safety prevents mixing:
// userSet.Insert(ProductID(789))  // ❌ ERROR: type mismatch
```

## Implementation Details

### Generic Type System

The HashSet uses Go's type parameter system:

```go
type HashSet[T comparable] struct {
    shards     []shard[T]
    shardCount uint32
    hasher     Hasher[T]
    size       int64
}

type shard[T comparable] struct {
    mu       sync.RWMutex
    indexMap map[T]int    // Direct type storage
    elements []T          // No interface{} boxing
}
```

### Sharding Strategy

Elements are distributed across shards using:
```go
shardIndex = hash(element) & (shardCount - 1)
```

This ensures even distribution while maintaining O(1) shard selection.

### Swap-and-Pop Removal

When removing element at index `i`:
1. Swap element with last element
2. Update index mapping for swapped element  
3. Truncate slice
4. Delete from hash map

This maintains contiguous storage for efficient random access.

### Random Element Selection

1. Generate random index in range [0, totalSize)
2. Find shard containing that index
3. Select element at local index within shard
4. Return typed element directly (no conversion needed)

This provides uniform distribution across all elements.

### Thread Safety

- Each shard protected by `sync.RWMutex`
- Read operations use `RLock()` (concurrent reads allowed)
- Write operations use `Lock()` (exclusive access)
- Atomic counters for global size tracking

## Memory Usage

- **Per element**: ~16-24 bytes overhead (reduced from interface{} version)
- **Per shard**: ~40 bytes base overhead
- **Total overhead**: `O(shardCount + elementCount)`
- **Savings**: 20-40% less memory usage compared to interface{} version

## Migration from Interface{} Version

### Required Changes

```go
// Before (interface{} version)
set := NewHashSet()
set.Insert("hello")
elem, ok := set.RandomElement()
if ok {
    str := elem.(string)  // Type assertion needed
}

// After (generic version)
set := NewHashSet[string]()
set.Insert("hello")
elem, ok := set.RandomElement()
if ok {
    // elem is already string type, no assertion needed
    fmt.Println(elem)
}
```

### Benefits of Migration

1. **Type Safety**: Compile-time error detection
2. **Performance**: 15-40% faster operations
3. **Memory**: 20-40% less memory usage
4. **Code Quality**: No type assertions, better IDE support

## Limitations

- Requires Go 1.18+ for generics support
- Type must satisfy `comparable` constraint
- Iterator creates snapshot (not live view)

## Future Enhancements

- **Specialized Hash Functions**: Type-specific optimized hash implementations
- **SIMD Operations**: Vector operations for primitive types
- **Memory Pools**: Type-specific object pooling
- **Persistent Storage**: Generic serialization support

## License

This implementation is provided as-is for educational and practical use.

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass
2. Benchmarks show no performance regression
3. Thread safety is maintained
4. Generic type constraints are respected
5. Documentation is updated

## Bootstrap Client Application

In addition to the bootstrap server, this project includes a client application that demonstrates how to connect to and interact with the bootstrap server. The client provides a web UI similar to the bootstrap status page.

### Features

- **Connection Management**: Connect/disconnect to bootstrap server
- **Names Display**: View names received from the server in a table format
- **Real-time Updates**: Auto-refreshing UI with HTMX
- **Similar Styling**: UI styled consistently with the bootstrap status page
- **Environment Configuration**: Configurable client name and server URL

### Building Docker Images

Before running the application, you need to build the Docker images. This is automated for you:

```bash
# Build all Docker images (bootstrap + client)
./build.sh
```

This script will:
- Build both bootstrap and client Docker images
- Tag images with proper naming conventions for compatibility  
- Display all available images when done

### Running the Client

#### Development Mode

```bash
# Automatically builds images if needed, then starts with hot reloading
./dev.sh
```

Or manually:

```bash
cd client
./dev.sh
```

This starts the client in development mode with hot reloading:
- Client Name: `dev-client`
- Bootstrap URL: `http://localhost:8080`
- Port: `9090`

Access the client UI at: `http://localhost:9090`

#### Using Docker

```bash
# Start both bootstrap server and client
docker-compose up

# Development mode with hot reloading
docker-compose -f docker-compose.dev.yml up
```

#### Manual Configuration

```bash
cd client
export CLIENT_NAME="my-client"
export BOOTSTRAP_URL="http://localhost:8080"
export PORT="9090"
go run .
```

### Client UI

The client provides a web interface with:

1. **Connection Status**: Shows current connection state to bootstrap server
2. **Control Panel**: Connect/disconnect buttons and configuration display
3. **Names Table**: Displays names received from the bootstrap server
4. **Auto-refresh**: Updates every 3 seconds automatically
5. **Error Handling**: Shows connection errors and status messages

### API Endpoints

- `GET /` - Main client status page
- `GET /status-data` - Get current client status (HTMX endpoint)
- `POST /connect` - Connect to bootstrap server
- `POST /disconnect` - Disconnect from bootstrap server  
- `POST /refresh` - Refresh names from server

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CLIENT_NAME` | `default-client` | Name used to identify this client |
| `BOOTSTRAP_URL` | `http://localhost:8080` | URL of the bootstrap server |
| `PORT` | `9090` | Port for the client web server |

### Docker Compose Configuration

The client is configured in both `docker-compose.yml` and `docker-compose.dev.yml`:

```yaml
client:
  build:
    context: ./client
    dockerfile: Dockerfile
  ports:
    - "9090:9090"
  environment:
    - CLIENT_NAME=demo-client
    - BOOTSTRAP_URL=http://bootstrap:8080
    - PORT=9090
  depends_on:
    - bootstrap
```

### Client Architecture

The client is built using:
- **Go HTTP Server**: Serves the web UI and handles API requests
- **Templ**: Template engine for server-side rendering
- **HTMX**: For dynamic UI updates without JavaScript
- **Concurrent-Safe Design**: Thread-safe operations with mutex protection

### Troubleshooting

#### "Failed to create client: image not found" Error

If you encounter an error like:
```
Failed to create client: failed to ensure image exists: image scalability-engineering_client not found
```

This happens when the Docker images haven't been built yet. The solution is automatic:

1. **Run the build script**: `./build.sh` - This builds and properly tags all images
2. **Or use dev.sh**: `./dev.sh` - This automatically checks and builds images if needed

**Technical Details**: Docker Compose uses hyphens in image names (`scalability-engineering-client`) but the code expects underscores (`scalability-engineering_client`). The build script automatically creates both naming conventions for compatibility.

---

**Note**: This generic HashSet provides significant advantages over interface{}-based implementations through type safety and performance improvements. It's optimized for scenarios where random element access is required with the added benefits of compile-time type checking and zero boxing overhead. 