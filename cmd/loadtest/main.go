// Command loadtest measures the latency and throughput of a running
// instance of the platform by sending it real HTTP requests over the
// network — as opposed to Go micro-benchmarks, which call handlers
// in-process and never touch a socket. It exists specifically to back
// the performance-evaluation report: latency is measured by timing
// each endpoint's response, and throughput is measured by counting how
// many requests a repeated, concurrent local run can process.
//
// Usage:
//
//	go run ./cmd/server &
//	go run ./cmd/loadtest -endpoint /report
//	go run ./cmd/loadtest -endpoint /login -method POST \
//	    -body '{"username":"admin","password":"changeme"}' -content-type application/json
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "base URL of a running platform instance")
	endpoint := flag.String("endpoint", "/report", "endpoint path to test")
	method := flag.String("method", "GET", "HTTP method")
	body := flag.String("body", "", "request body (for POST)")
	contentType := flag.String("content-type", "", "Content-Type header (e.g. application/json)")
	n := flag.Int("n", 50, "number of sequential requests for the latency phase")
	workers := flag.Int("c", 20, "number of concurrent workers for the throughput phase")
	duration := flag.Duration("duration", 3*time.Second, "duration of the throughput phase")
	flag.Parse()

	url := *base + *endpoint
	client := &http.Client{Timeout: 10 * time.Second}
	newRequest := func() (*http.Request, error) {
		var reader io.Reader
		if *body != "" {
			reader = bytes.NewReader([]byte(*body))
		}
		req, err := http.NewRequest(*method, url, reader)
		if err != nil {
			return nil, err
		}
		if *contentType != "" {
			req.Header.Set("Content-Type", *contentType)
		}
		return req, nil
	}

	fmt.Printf("Target: %s %s\n\n", *method, url)

	if err := runLatencyPhase(client, newRequest, *n); err != nil {
		log.Fatalf("latency phase: %v", err)
	}
	fmt.Println()
	if err := runThroughputPhase(client, newRequest, *workers, *duration); err != nil {
		log.Fatalf("throughput phase: %v", err)
	}
}

// runLatencyPhase sends n sequential requests to the target endpoint and
// reports the response-time distribution, directly answering "how long
// does a single call to this endpoint take?".
func runLatencyPhase(client *http.Client, newRequest func() (*http.Request, error), n int) error {
	durations := make([]time.Duration, 0, n)
	for i := range n {
		req, err := newRequest()
		if err != nil {
			return err
		}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("request %d: %w", i, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		durations = append(durations, time.Since(start))
	}

	slices.Sort(durations)
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	mean := total / time.Duration(len(durations))
	p50 := durations[len(durations)*50/100]
	p95 := durations[min(len(durations)*95/100, len(durations)-1)]

	fmt.Printf("Latency (sequential, n=%d):\n", n)
	fmt.Printf("  min=%s  mean=%s  p50=%s  p95=%s  max=%s\n",
		durations[0], mean, p50, p95, durations[len(durations)-1])
	return nil
}

// runThroughputPhase runs `workers` goroutines issuing requests as fast
// as the server allows for `duration`, then reports how many requests
// were completed per second — a real, repeated local load test.
func runThroughputPhase(client *http.Client, newRequest func() (*http.Request, error), workers int, duration time.Duration) error {
	var completed int64
	var failed int64
	var wg sync.WaitGroup
	deadline := time.Now().Add(duration)

	for range workers {
		wg.Go(func() {
			for time.Now().Before(deadline) {
				req, err := newRequest()
				if err != nil {
					atomic.AddInt64(&failed, 1)
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddInt64(&failed, 1)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				atomic.AddInt64(&completed, 1)
			}
		})
	}

	start := time.Now()
	wg.Wait()
	elapsed := time.Since(start)

	rps := float64(completed) / elapsed.Seconds()
	fmt.Printf("Throughput (concurrent, workers=%d, duration=%s):\n", workers, duration)
	fmt.Printf("  requests=%d  failed=%d  elapsed=%s  req/s=%.1f\n",
		completed, failed, elapsed.Round(time.Millisecond), rps)

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d requests failed\n", failed)
	}
	return nil
}
