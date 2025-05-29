package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type BenchmarkConfig struct {
	URL           string
	AuthToken     string
	TotalRequests int
	ParallelCount int
	Timeout       time.Duration
	Headers       map[string]string
	Parameters    map[string]string
}

type BenchmarkResult struct {
	TotalRequests   int
	SuccessfulReqs  int
	FailedReqs      int
	TotalDuration   time.Duration
	AvgResponseTime time.Duration
	MinResponseTime time.Duration
	MaxResponseTime time.Duration
	RequestsPerSec  float64
	ResponseTimes   []time.Duration
}

func main() {
	var config BenchmarkConfig
	var headersFlag, paramsFlag string

	flag.StringVar(&config.URL, "url", "", "Target URL to benchmark (required)")
	flag.StringVar(&config.AuthToken, "token", "", "Authorization token (Bearer token)")
	flag.IntVar(&config.TotalRequests, "total", 100, "Total number of requests to make")
	flag.IntVar(&config.ParallelCount, "parallel", 10, "Number of parallel requests")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Request timeout")
	flag.StringVar(&headersFlag, "headers", "", "Custom headers (format: 'key1:value1,key2:value2')")
	flag.StringVar(&paramsFlag, "params", "", "Query parameters (format: 'key1=value1,key2=value2')")
	flag.Parse()

	if config.URL == "" {
		fmt.Println("Error: URL is required")
		flag.Usage()
		return
	}

	// Parse headers
	config.Headers = parseKeyValuePairs(headersFlag, ":")

	// Parse parameters
	config.Parameters = parseKeyValuePairs(paramsFlag, "=")

	fmt.Printf("Starting HTTP benchmark...\n")
	fmt.Printf("URL: %s\n", config.URL)
	fmt.Printf("Total requests: %d\n", config.TotalRequests)
	fmt.Printf("Parallel requests: %d\n", config.ParallelCount)
	fmt.Printf("Timeout: %v\n", config.Timeout)

	if len(config.Headers) > 0 {
		fmt.Printf("Custom headers: %d\n", len(config.Headers))
		for k, v := range config.Headers {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if len(config.Parameters) > 0 {
		fmt.Printf("Query parameters: %d\n", len(config.Parameters))
		for k, v := range config.Parameters {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}

	fmt.Println("----------------------------------------")

	result := runBenchmark(config)
	printResults(result)
}

func runBenchmark(config BenchmarkConfig) BenchmarkResult {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Channel to distribute work
	workChan := make(chan int, config.TotalRequests)
	resultsChan := make(chan RequestResult, config.TotalRequests)

	// Fill work channel
	for i := 0; i < config.TotalRequests; i++ {
		workChan <- i
	}
	close(workChan)

	// Start workers
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < config.ParallelCount; i++ {
		wg.Add(1)
		go worker(client, config, workChan, resultsChan, &wg)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []RequestResult
	for result := range resultsChan {
		results = append(results, result)
	}

	totalDuration := time.Since(startTime)
	return calculateBenchmarkResult(results, totalDuration)
}

type RequestResult struct {
	Success      bool
	ResponseTime time.Duration
	StatusCode   int
	Error        error
}

func worker(client *http.Client, config BenchmarkConfig, workChan <-chan int, resultsChan chan<- RequestResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for range workChan {
		result := makeRequest(client, config)
		resultsChan <- result
	}
}

func makeRequest(client *http.Client, config BenchmarkConfig) RequestResult {
	// Build URL with query parameters
	targetURL := config.URL
	if len(config.Parameters) > 0 {
		u, err := url.Parse(config.URL)
		if err != nil {
			return RequestResult{
				Success: false,
				Error:   fmt.Errorf("invalid URL: %w", err),
			}
		}

		query := u.Query()
		for key, value := range config.Parameters {
			query.Set(key, value)
		}
		u.RawQuery = query.Encode()
		targetURL = u.String()
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", targetURL, nil)
	if err != nil {
		return RequestResult{
			Success: false,
			Error:   err,
		}
	}

	// Add auth token if provided
	if config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.AuthToken)
	}

	// Add custom headers
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	// Add common headers (only if not overridden by custom headers)
	if _, exists := config.Headers["User-Agent"]; !exists {
		req.Header.Set("User-Agent", "HTTP-Benchmark-Client/1.0")
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		return RequestResult{
			Success:      false,
			ResponseTime: responseTime,
			Error:        err,
		}
	}
	defer resp.Body.Close()

	// Read and discard response body to ensure complete request
	_, _ = io.Copy(io.Discard, resp.Body)

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return RequestResult{
		Success:      success,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
	}
}

func calculateBenchmarkResult(results []RequestResult, totalDuration time.Duration) BenchmarkResult {
	var successCount, failedCount int
	var responseTimes []time.Duration
	var totalResponseTime time.Duration
	minTime := time.Hour // Initialize with large value
	maxTime := time.Duration(0)

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failedCount++
		}

		responseTimes = append(responseTimes, result.ResponseTime)
		totalResponseTime += result.ResponseTime

		if result.ResponseTime < minTime {
			minTime = result.ResponseTime
		}
		if result.ResponseTime > maxTime {
			maxTime = result.ResponseTime
		}
	}

	avgResponseTime := time.Duration(0)
	if len(results) > 0 {
		avgResponseTime = totalResponseTime / time.Duration(len(results))
	}

	requestsPerSec := float64(len(results)) / totalDuration.Seconds()

	return BenchmarkResult{
		TotalRequests:   len(results),
		SuccessfulReqs:  successCount,
		FailedReqs:      failedCount,
		TotalDuration:   totalDuration,
		AvgResponseTime: avgResponseTime,
		MinResponseTime: minTime,
		MaxResponseTime: maxTime,
		RequestsPerSec:  requestsPerSec,
		ResponseTimes:   responseTimes,
	}
}

func printResults(result BenchmarkResult) {
	fmt.Println("\n========== BENCHMARK RESULTS ==========")
	fmt.Printf("Total requests:      %d\n", result.TotalRequests)
	fmt.Printf("Successful requests: %d\n", result.SuccessfulReqs)
	fmt.Printf("Failed requests:     %d\n", result.FailedReqs)
	fmt.Printf("Success rate:        %.2f%%\n", float64(result.SuccessfulReqs)/float64(result.TotalRequests)*100)
	fmt.Println("----------------------------------------")
	fmt.Printf("Total time:          %v\n", result.TotalDuration)
	fmt.Printf("Requests per second: %.2f\n", result.RequestsPerSec)
	fmt.Println("----------------------------------------")
	fmt.Printf("Response times:\n")
	fmt.Printf("  Average:           %v\n", result.AvgResponseTime)
	fmt.Printf("  Minimum:           %v\n", result.MinResponseTime)
	fmt.Printf("  Maximum:           %v\n", result.MaxResponseTime)

	// Calculate percentiles
	if len(result.ResponseTimes) > 0 {
		percentiles := calculatePercentiles(result.ResponseTimes)
		fmt.Printf("  50th percentile:   %v\n", percentiles[50])
		fmt.Printf("  95th percentile:   %v\n", percentiles[95])
		fmt.Printf("  99th percentile:   %v\n", percentiles[99])
	}
	fmt.Println("========================================")
}

func calculatePercentiles(times []time.Duration) map[int]time.Duration {
	// Simple percentile calculation
	sorted := make([]time.Duration, len(times))
	copy(sorted, times)

	// Basic bubble sort for simplicity
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	percentiles := make(map[int]time.Duration)
	percentiles[50] = sorted[len(sorted)*50/100]
	percentiles[95] = sorted[len(sorted)*95/100]
	percentiles[99] = sorted[len(sorted)*99/100]

	return percentiles
}

// parseKeyValuePairs parses a string of key-value pairs separated by commas
// Format: "key1<separator>value1,key2<separator>value2"
func parseKeyValuePairs(input, separator string) map[string]string {
	result := make(map[string]string)

	if input == "" {
		return result
	}

	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), separator, 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				result[key] = value
			}
		}
	}

	return result
}
