# GoFlood Improvements - Stable Thread Management & Rate Control

## Key Improvements Made

### 1. **Proper Rate Limiting**
- Implemented `golang.org/x/time/rate` package for precise rate limiting
- Global rate limiter ensures requests don't exceed specified rate across all threads
- Token bucket algorithm provides smooth and consistent request distribution

### 2. **Thread Management & Synchronization**
- **Worker Pool Pattern**: Structured worker pool with proper lifecycle management
- **Context-based Control**: Uses Go contexts for graceful shutdown and cancellation
- **WaitGroup Synchronization**: Ensures all workers complete before program exits
- **Thread-safe Operations**: Mutex protection for shared resources (HTTP clients)

### 3. **Graceful Shutdown**
- Signal handling for SIGINT/SIGTERM (Ctrl+C)
- Workers stop cleanly when interrupted
- Final statistics displayed on shutdown
- All connections properly closed

### 4. **Resource Management**
- **Client Lifecycle**: HTTP clients refreshed every 30 seconds to avoid stale connections
- **Connection Pooling**: Reuses clients for multiple requests (session simulation)
- **Proper Cleanup**: All resources (connections, goroutines) cleaned up on exit
- **Memory Efficiency**: Response bodies properly drained and closed

### 5. **Error Handling & Stability**
- Robust error handling at every level
- Workers continue on individual request failures
- Request timeouts (10 seconds) prevent hanging
- Client creation failures handled gracefully

### 6. **Monitoring & Statistics**
- Real-time statistics every 5 seconds
- Tracks total requests, success, failures, and RPS
- Per-worker statistics available
- Atomic operations for thread-safe counters

### 7. **Performance Optimizations**
- Parallel request processing within rate limits
- Efficient request distribution across workers
- Minimal lock contention
- Optimized for high concurrency

## Usage Remains the Same
```bash
./goflood <url> <rate> <secs> <threads> [--cached]
```

## Architecture Overview

```
Main Process
    ├── Worker Pool Manager
    │   ├── Rate Limiter (global)
    │   └── Context (cancellation)
    │
    ├── Worker 1
    │   ├── HTTP Client (refreshed periodically)
    │   ├── Request Loop
    │   └── Statistics
    │
    ├── Worker 2
    │   └── ...
    │
    └── Monitor (statistics reporter)
```

## Benefits

1. **Stable Performance**: Consistent request rate without spikes or drops
2. **Resource Efficiency**: Proper cleanup prevents memory leaks
3. **Graceful Behavior**: Clean shutdown without hanging processes
4. **Observable**: Real-time monitoring of performance
5. **Maintainable**: Clean, structured code with clear separation of concerns