package main

import (
    "fmt"
    "net/http"
    "time"

    "github.com/quic-go/quic-go/http3"
)

func main() {
    client := http.Client{
        Transport: &http3.RoundTripper{},
        Timeout:   5 * time.Second,
    }
    
    for i := 0; i < 10; i++ {
        req, err := http.NewRequest("GET", "https://cloudflare-quic.com/", nil)
        if err != nil {
            fmt.Println("Error creating request:", err)
            continue
        }
        
        // Add real browser headers to bypass Cloudflare
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
        if err != nil {
            fmt.Println("Error:", err)
            continue
        }
        fmt.Println("Status:", resp.Status)
        resp.Body.Close()
    }
}

