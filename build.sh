#!/bin/bash

echo "Building optimized goflood..."
go build -o goflood goflood.go
echo "Build complete! Run with: ./goflood <url> <rate> <secs> <threads> [--cached]"