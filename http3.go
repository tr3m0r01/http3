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
        resp, err := client.Get("https://cloudflare-quic.com/")
        if err != nil {
            fmt.Println("Error:", err)
            continue
        }
        fmt.Println("Status:", resp.Status)
        resp.Body.Close()
    }
}

