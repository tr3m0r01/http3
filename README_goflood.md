# GoFlood - Optimized High-Performance HTTP Flood Tool

## Overview
This is an optimized version of goflood that has been completely rewritten for maximum performance using raw connections without any proxy support.

## Key Improvements

### 1. **Removed All Proxy Dependencies**
- Eliminated all proxy-related code and dependencies
- Removed `github.com/bogdanfinn/tls-client` and related packages
- Now uses only Go standard library for maximum compatibility

### 2. **Raw Connection Implementation**
- Direct TCP connections with optimized settings
- Custom `DialContext` with TCP no-delay for lower latency
- Keep-alive connections for better performance
- Connection pooling using `sync.Pool` for efficient reuse

### 3. **Performance Optimizations**
- **Connection Pooling**: Reuses HTTP clients through `sync.Pool`
- **Batch Processing**: Prepares requests in batches before sending
- **Concurrent Requests**: Uses goroutines for parallel request execution
- **Optimized TLS**: Modern cipher suites and session caching
- **HTTP/2 Support**: Enabled by default for better performance
- **Pre-generated Paths**: For cached mode, paths are pre-generated to reduce overhead

### 4. **Network Optimizations**
- TCP No-Delay enabled for lower latency
- Keep-alive connections with 30-second intervals
- Large connection pool (1000 idle connections)
- No connection limits per host
- Optimized timeouts for fast failure detection

### 5. **Better Resource Management**
- Efficient memory usage with request buffer reuse
- Proper connection cleanup
- Atomic counters for thread-safe statistics
- Real-time stats reporting without screen flickering

## Usage

```bash
./goflood <url> <rate> <secs> <threads> [--cached]
```

### Parameters:
- `url`: Target URL to flood
- `rate`: Requests per second per thread
- `secs`: Duration of the attack in seconds
- `threads`: Number of concurrent threads
- `--cached`: Optional flag for cache-busting mode

### Examples:
```bash
# Basic flood - 100 req/s, 60 seconds, 10 threads
./goflood https://example.com 100 60 10

# Cache-busting mode
./goflood https://example.com 50 30 5 --cached
```

## Building

```bash
# Using the build script
./build.sh

# Or manually
go build -o goflood goflood.go
```

## Technical Details

### Connection Settings:
- **Dial Timeout**: 5 seconds
- **Keep-Alive**: 30 seconds
- **Response Timeout**: 10 seconds
- **TLS Version**: 1.2-1.3
- **HTTP Version**: HTTP/2 preferred, falls back to HTTP/1.1

### Cipher Suites:
- TLS_AES_128_GCM_SHA256
- TLS_AES_256_GCM_SHA384
- TLS_CHACHA20_POLY1305_SHA256
- ECDHE cipher suites for forward secrecy

### Headers:
- Realistic browser headers
- Random User-Agent rotation
- Accept-Language variation
- Chrome/Firefox specific headers

## Performance Tips

1. **Adjust Rate**: Start with lower rates and increase gradually
2. **Thread Count**: Use threads = CPU cores * 2 for optimal performance
3. **Cached Mode**: Use for testing CDN and cache behavior
4. **Monitor Resources**: Watch CPU and memory usage during tests

## Comparison with Original

| Feature | Original | Optimized |
|---------|----------|-----------|
| Proxy Support | Required | Removed |
| Dependencies | Multiple external | Standard library only |
| Connection Type | Through proxy | Direct/Raw |
| Performance | Limited by proxy | Maximum throughput |
| TLS Handling | Custom library | Native Go TLS |
| Memory Usage | Higher | Optimized |
| Connection Reuse | Limited | Pooled |

## Notes

- This tool is for testing and educational purposes only
- Always ensure you have permission to test against target servers
- Monitor your network and system resources during use
- The tool now focuses on raw performance without proxy overhead