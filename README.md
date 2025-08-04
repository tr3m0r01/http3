# GoFlood - Optimized HTTP Flood Tool

An optimized HTTP flood testing tool with advanced worker pool architecture for maximum CPU and network utilization.

## Features

- **Worker Pool Architecture**: Efficient concurrent request handling with proper worker management
- **CPU Optimization**: Automatically uses all available CPU cores with `runtime.GOMAXPROCS`
- **Network Optimization**: 
  - Connection pooling and reuse
  - Optimized buffer sizes
  - Custom dialer settings
  - Keep-alive connections
- **Legitimate Traffic Patterns**:
  - Removed botnet signatures
  - Realistic browser fingerprints
  - Proper header ordering
  - Referrer headers from legitimate sources
- **Real-time Statistics**: Live monitoring of requests/sec, success rate, and data transfer
- **Stability Improvements**:
  - Proper error handling
  - Graceful shutdown
  - Connection recycling
  - Context-based cancellation

## Installation

```bash
go mod download
go build -o goflood goflood.go
```

## Usage

```bash
./goflood <url> <rate> <seconds> <threads> [--cached]
```

### Parameters:
- `url`: Target URL
- `rate`: Requests per worker per cycle
- `seconds`: Duration of the test
- `threads`: Number of worker threads
- `--cached`: Optional flag for cache-busting mode

### Example:
```bash
./goflood https://example.com 100 60 50
./goflood https://example.com/api 200 30 100 --cached
```

## Improvements Made

1. **Worker Pool Implementation**: Replaced simple goroutines with a proper worker pool pattern for better resource management
2. **CPU Utilization**: Uses all available CPU cores automatically
3. **Network Optimization**: Implemented connection pooling, optimized buffers, and proper keep-alive settings
4. **Removed Botnet Signatures**: All user agents and headers now mimic legitimate browser traffic
5. **Statistics Tracking**: Real-time monitoring of performance metrics
6. **Error Handling**: Proper error counting and graceful degradation
7. **Memory Efficiency**: Reuses clients and connections to reduce memory allocation

## Performance Tips

1. Increase system file descriptor limits:
   ```bash
   ulimit -n 65535
   ```

2. Tune kernel parameters for better network performance:
   ```bash
   echo 'net.ipv4.tcp_tw_reuse = 1' >> /etc/sysctl.conf
   echo 'net.ipv4.ip_local_port_range = 1024 65535' >> /etc/sysctl.conf
   sysctl -p
   ```

3. Use appropriate thread count based on your CPU cores and network capacity

## Disclaimer

This tool is for testing and educational purposes only. Only use it against systems you own or have explicit permission to test.