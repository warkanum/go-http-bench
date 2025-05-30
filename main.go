package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type BenchmarkConfig struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"` // GET, POST, PUT, etc.
	AuthToken       string            `json:"auth_token"`
	TotalRequests   int               `json:"total_requests"`
	ParallelCount   int               `json:"parallel_count"`
	Timeout         time.Duration     `json:"-"`       // Not directly unmarshaled
	TimeoutStr      string            `json:"timeout"` // String representation for JSON
	Headers         map[string]string `json:"headers"`
	Parameters      map[string]string `json:"parameters"`
	PostDataFile    string            `json:"post_data_file"`    // File containing POST data
	PostData        string            `json:"post_data"`         // Direct POST data
	ContentType     string            `json:"content_type"`      // Content-Type for POST requests
	DumpFailuresDir string            `json:"dump_failures_dir"` // Directory to dump failure responses
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

type RequestResult struct {
	Success      bool
	ResponseTime time.Duration
	StatusCode   int
	Error        error
	ResponseBody string // Added to capture response body for failures
}

type FailureDumper struct {
	enabled      bool
	dumpDir      string
	seenHashes   map[string]bool
	mutex        sync.Mutex
	failureCount int
}

func NewFailureDumper(dumpDir string) *FailureDumper {
	if dumpDir == "" {
		return &FailureDumper{enabled: false}
	}

	// Create dump directory if it doesn't exist
	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		fmt.Printf("Warning: Could not create dump directory %s: %v\n", dumpDir, err)
		return &FailureDumper{enabled: false}
	}

	return &FailureDumper{
		enabled:    true,
		dumpDir:    dumpDir,
		seenHashes: make(map[string]bool),
	}
}

func (fd *FailureDumper) DumpFailure(statusCode int, responseBody, errorMsg string) {
	if !fd.enabled {
		return
	}

	// Create a unique identifier for this failure type
	failureContent := fmt.Sprintf("Status: %d\nError: %s\nResponse: %s", statusCode, errorMsg, responseBody)
	hash := md5.Sum([]byte(failureContent))
	hashStr := hex.EncodeToString(hash[:])

	fd.mutex.Lock()
	defer fd.mutex.Unlock()

	// Check if we've already seen this failure type
	if fd.seenHashes[hashStr] {
		return
	}

	// Mark this failure type as seen
	fd.seenHashes[hashStr] = true
	fd.failureCount++

	// Create filename with timestamp and failure number
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("failure_%d_%s_status_%d.txt", fd.failureCount, timestamp, statusCode)
	filepath := filepath.Join(fd.dumpDir, filename)

	// Prepare failure details
	failureDetails := fmt.Sprintf(`HTTP Benchmark Failure Report
Generated: %s
Hash: %s

Status Code: %d
Error Message: %s

Response Headers: (captured in request)
Response Body:
%s

----------------------------------------
This is a unique failure response that hasn't been seen before in this benchmark run.
`, time.Now().Format("2006-01-02 15:04:05"), hashStr, statusCode, errorMsg, responseBody)

	// Write to file
	if err := os.WriteFile(filepath, []byte(failureDetails), 0644); err != nil {
		fmt.Printf("Warning: Could not write failure dump to %s: %v\n", filepath, err)
	}
}

func main() {
	var config BenchmarkConfig
	var headersFlag, paramsFlag, configFile string

	// Command line flags (for backward compatibility)
	flag.StringVar(&configFile, "config", "", "Configuration file (JSON format)")
	flag.StringVar(&config.URL, "url", "", "Target URL to benchmark")
	flag.StringVar(&config.Method, "method", "GET", "HTTP method (GET, POST, PUT, etc.)")
	flag.StringVar(&config.AuthToken, "token", "", "Authorization token (Bearer token)")
	flag.IntVar(&config.TotalRequests, "total", 100, "Total number of requests to make")
	flag.IntVar(&config.ParallelCount, "parallel", 10, "Number of parallel requests")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Request timeout")
	flag.StringVar(&headersFlag, "headers", "", "Custom headers (format: 'key1:value1,key2:value2')")
	flag.StringVar(&paramsFlag, "params", "", "Query parameters (format: 'key1=value1,key2=value2')")
	flag.StringVar(&config.DumpFailuresDir, "dump-failures", "", "Directory to dump failure responses (only unique failures)")
	flag.StringVar(&config.PostDataFile, "post-file", "", "File containing POST data")
	flag.StringVar(&config.PostData, "post-data", "", "POST data as string")
	flag.StringVar(&config.ContentType, "content-type", "application/json", "Content-Type for POST requests")
	flag.Parse()

	// Load config from file if specified
	if configFile != "" {
		fileConfig, err := loadConfigFromFile(configFile)
		if err != nil {
			fmt.Printf("Error loading config file: %v\n", err)
			return
		}
		config = fileConfig
	}

	// Command line flags override config file values
	if headersFlag != "" {
		if config.Headers == nil {
			config.Headers = make(map[string]string)
		}
		headersParsed := parseKeyValuePairs(headersFlag, ":")
		for k, v := range headersParsed {
			config.Headers[k] = v
		}
	}

	if paramsFlag != "" {
		if config.Parameters == nil {
			config.Parameters = make(map[string]string)
		}
		paramsParsed := parseKeyValuePairs(paramsFlag, "=")
		for k, v := range paramsParsed {
			config.Parameters[k] = v
		}
	}

	// Validate required fields
	if config.URL == "" {
		fmt.Println("Error: URL is required (use -url flag or config file)")
		flag.Usage()
		return
	}

	// Set defaults
	if config.Method == "" {
		config.Method = "GET"
	}
	if config.TotalRequests == 0 {
		config.TotalRequests = 100
	}
	if config.ParallelCount == 0 {
		config.ParallelCount = 10
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.ContentType == "" {
		config.ContentType = "application/json"
	}

	// Load POST data from file if specified
	if config.PostDataFile != "" && config.PostData == "" {
		data, err := os.ReadFile(config.PostDataFile)
		if err != nil {
			fmt.Printf("Error reading POST data file: %v\n", err)
			return
		}
		config.PostData = string(data)
	}

	printConfig(config)
	fmt.Println("----------------------------------------")

	result := runBenchmark(config)
	printResults(result)
}

func loadConfigFromFile(filename string) (BenchmarkConfig, error) {
	var config BenchmarkConfig

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	// Convert timeout string to time.Duration
	if config.TimeoutStr != "" {
		config.Timeout, err = time.ParseDuration(config.TimeoutStr)
		if err != nil {
			return config, fmt.Errorf("invalid timeout format '%s': %w", config.TimeoutStr, err)
		}
	}

	return config, nil
}

func printConfig(config BenchmarkConfig) {
	fmt.Printf("Starting HTTP benchmark...\n")
	fmt.Printf("URL: %s\n", config.URL)
	fmt.Printf("Method: %s\n", config.Method)
	fmt.Printf("Total requests: %d\n", config.TotalRequests)
	fmt.Printf("Parallel requests: %d\n", config.ParallelCount)
	fmt.Printf("Timeout: %v\n", config.Timeout)

	if config.AuthToken != "" {
		fmt.Printf("Auth token: [PROVIDED]\n")
	}

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

	if config.PostData != "" || config.PostDataFile != "" {
		fmt.Printf("POST data: [PROVIDED]\n")
		if config.PostDataFile != "" {
			fmt.Printf("POST data file: %s\n", config.PostDataFile)
		}
		fmt.Printf("Content-Type: %s\n", config.ContentType)
	}

	if config.DumpFailuresDir != "" {
		fmt.Printf("Failure dump directory: %s\n", config.DumpFailuresDir)
	}
}

func runBenchmark(config BenchmarkConfig) BenchmarkResult {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// Create failure dumper
	failureDumper := NewFailureDumper(config.DumpFailuresDir)

	// Channel to distribute work
	workChan := make(chan WorkItem, config.TotalRequests)
	resultsChan := make(chan RequestResult, config.TotalRequests)

	// Fill work channel with test numbers
	for i := 0; i < config.TotalRequests; i++ {
		workChan <- WorkItem{TestNumber: i}
	}
	close(workChan)

	// Start workers
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < config.ParallelCount; i++ {
		wg.Add(1)
		go worker(client, config, i, workChan, resultsChan, failureDumper, &wg)
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

	// Print failure dump summary
	if failureDumper.enabled {
		fmt.Printf("\nFailure dump summary: %d unique failure types saved to %s\n",
			failureDumper.failureCount, failureDumper.dumpDir)
	}

	return calculateBenchmarkResult(results, totalDuration)
}

type WorkItem struct {
	TestNumber int
}

func worker(client *http.Client, config BenchmarkConfig, threadNumber int, workChan <-chan WorkItem, resultsChan chan<- RequestResult, failureDumper *FailureDumper, wg *sync.WaitGroup) {
	defer wg.Done()

	for workItem := range workChan {
		result := makeRequest(client, config, workItem.TestNumber, threadNumber, failureDumper)
		resultsChan <- result
	}
}

func makeRequest(client *http.Client, config BenchmarkConfig, testNumber, threadNumber int, failureDumper *FailureDumper) RequestResult {
	// Replace variables in URL
	targetURL := replaceVariables(config.URL, testNumber, threadNumber)

	// Build URL with query parameters
	if len(config.Parameters) > 0 {
		u, err := url.Parse(targetURL)
		if err != nil {
			errorMsg := fmt.Sprintf("invalid URL: %v", err)
			failureDumper.DumpFailure(0, "", errorMsg)
			return RequestResult{
				Success: false,
				Error:   fmt.Errorf("invalid URL: %w", err),
			}
		}

		query := u.Query()
		for key, value := range config.Parameters {
			replacedKey := replaceVariables(key, testNumber, threadNumber)
			replacedValue := replaceVariables(value, testNumber, threadNumber)
			query.Set(replacedKey, replacedValue)
		}
		u.RawQuery = query.Encode()
		targetURL = u.String()
	}

	// Prepare request body for POST/PUT requests
	var requestBody io.Reader
	if config.PostData != "" && (config.Method == "POST" || config.Method == "PUT" || config.Method == "PATCH") {
		bodyData := replaceVariables(config.PostData, testNumber, threadNumber)
		requestBody = strings.NewReader(bodyData)
	}

	req, err := http.NewRequestWithContext(context.Background(), config.Method, targetURL, requestBody)
	if err != nil {
		errorMsg := fmt.Sprintf("request creation failed: %v", err)
		failureDumper.DumpFailure(0, "", errorMsg)
		return RequestResult{
			Success: false,
			Error:   err,
		}
	}

	// Add auth token if provided
	if config.AuthToken != "" {
		authToken := replaceVariables(config.AuthToken, testNumber, threadNumber)
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	// Add custom headers with variable replacement
	for key, value := range config.Headers {
		replacedKey := replaceVariables(key, testNumber, threadNumber)
		replacedValue := replaceVariables(value, testNumber, threadNumber)
		req.Header.Set(replacedKey, replacedValue)
	}

	// Add Content-Type for POST requests (only if not overridden by custom headers)
	if (config.Method == "POST" || config.Method == "PUT" || config.Method == "PATCH") && config.PostData != "" {
		if _, exists := config.Headers["Content-Type"]; !exists {
			req.Header.Set("Content-Type", config.ContentType)
		}
	}

	// Add common headers (only if not overridden by custom headers)
	if _, exists := config.Headers["User-Agent"]; !exists {
		req.Header.Set("User-Agent", "HTTP-Benchmark-Client/1.0")
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		errorMsg := fmt.Sprintf("request failed: %v", err)
		failureDumper.DumpFailure(0, "", errorMsg)
		return RequestResult{
			Success:      false,
			ResponseTime: responseTime,
			Error:        err,
		}
	}
	defer resp.Body.Close()

	// Read response body for potential failure dumping
	responseBody, readErr := io.ReadAll(resp.Body)
	responseBodyStr := ""
	if readErr == nil {
		responseBodyStr = string(responseBody)
	} else {
		responseBodyStr = fmt.Sprintf("Error reading response body: %v", readErr)
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Dump failure if request was not successful
	if !success {
		errorMsg := fmt.Sprintf("HTTP %d response", resp.StatusCode)
		failureDumper.DumpFailure(resp.StatusCode, responseBodyStr, errorMsg)
	}

	return RequestResult{
		Success:      success,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
		ResponseBody: responseBodyStr,
	}
}

// replaceVariables replaces [test_number] and [thread_number] with actual values
func replaceVariables(input string, testNumber, threadNumber int) string {
	result := input
	result = strings.ReplaceAll(result, "[test_number]", strconv.Itoa(testNumber))
	result = strings.ReplaceAll(result, "[thread_number]", strconv.Itoa(threadNumber))
	return result
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
