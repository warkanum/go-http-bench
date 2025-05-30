# HTTP Benchmark Tool

[![Go Version](https://img.shields.io/badge/Go-1.19+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/warkanum/go-http-bench)

A powerful, flexible HTTP benchmarking tool written in Go that supports variable replacement, file-based configuration, and multiple HTTP methods. Perfect for load testing APIs with dynamic data and tracking individual request performance.

## 🚀 Features

- **Multiple HTTP Methods**: GET, POST, PUT, PATCH, DELETE support
- **Variable Replacement**: Dynamic `[test_number]` and `[thread_number]` substitution
- **File-Based Configuration**: JSON configuration files for complex scenarios
- **POST Data Support**: Inline data or file-based payloads
- **Failure Dumping**: Automatic capture of unique API failure responses
- **Parallel Execution**: Configurable concurrent workers
- **Detailed Statistics**: Response times, percentiles, success rates
- **Custom Headers**: Support for authentication and custom headers
- **Query Parameters**: Dynamic URL parameter generation
- **Backward Compatible**: Works with original command-line interface

## 📦 Installation

### Option 1: Download Pre-built Binary
```bash
# Download from releases page
wget https://github.com/warkanum/go-http-bench/releases/latest/download/benchmark-linux-amd64
chmod +x benchmark-linux-amd64
mv benchmark-linux-amd64 /usr/local/bin/benchmark
```

### Option 2: Build from Source
```bash
# Clone the repository
git clone https://github.com/warkanum/go-http-bench.git
cd http-benchmark

# Build the binary
go build -o benchmark main.go

# Optional: Install globally
sudo mv benchmark /usr/local/bin/
```

### Option 3: Install with Go
```bash
go install github.com/warkanum/go-http-bench@latest
```

## 🏃 Quick Start

### Basic Usage
```bash
# Simple GET request benchmark
./benchmark -url "https://httpbin.org/get" -total 1000 -parallel 50

# POST request with JSON data
./benchmark \
    -url "https://httpbin.org/post" \
    -method POST \
    -post-data '{"message": "Hello World"}' \
    -total 500 \
    -parallel 20
```

### Using Configuration Files
```bash
# Run with JSON configuration
./benchmark -config config.json

# Override config file settings
./benchmark -config config.json -total 2000 -parallel 100
```

## 📋 Configuration

### Command Line Options

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `-config` | Configuration file path | - | `-config config.json` |
| `-url` | Target URL (required) | - | `-url "https://api.example.com"` |
| `-method` | HTTP method | `GET` | `-method POST` |
| `-total` | Total requests | `100` | `-total 1000` |
| `-parallel` | Parallel workers | `10` | `-parallel 50` |
| `-timeout` | Request timeout | `30s` | `-timeout 60s` |
| `-token` | Bearer token | - | `-token "abc123"` |
| `-headers` | Custom headers | - | `-headers "Accept:application/json"` |
| `-params` | Query parameters | - | `-params "page=1,limit=10"` |
| `-post-data` | Inline POST data | - | `-post-data '{"key":"value"}'` |
| `-post-file` | POST data file | - | `-post-file data.json` |
| `-content-type` | Content-Type header | `application/json` | `-content-type "text/xml"` |
| `-dump-failures` | Failure dump directory | - | `-dump-failures "./failures"` |

### JSON Configuration

Create a `config.json` file:

```json
{
    "url": "https://api.example.com/users",
    "method": "POST",
    "total_requests": 1000,
    "parallel_count": 50,
    "timeout": "30s",
    "auth_token": "your-bearer-token",
    "content_type": "application/json",
    "dump_failures_dir": "./failures",
    "headers": {
        "Accept": "application/json",
        "X-Request-ID": "bench-[test_number]-[thread_number]"
    },
    "parameters": {
        "source": "benchmark",
        "worker": "[thread_number]"
    },
    "post_data": "{\"id\": [test_number], \"worker\": [thread_number], \"timestamp\": \"2025-01-01T00:00:00Z\"}"
}
```

## 🔍 Failure Dumping

The tool automatically captures and saves unique API failure responses to help with debugging:

### Features
- **Automatic Detection**: Any HTTP status outside 200-299 is considered a failure
- **Unique Responses Only**: Uses MD5 hashing to save only distinct failure types
- **Detailed Reports**: Each dump includes status code, response body, timestamp, and hash
- **Organized Files**: Named as `failure_{number}_{timestamp}_status_{code}.txt`

### Example Usage
```bash
# Dump failures to a directory
./benchmark -url "https://api.example.com/test" -dump-failures "./debug" -total 1000

# JSON config with failure dumping
{
    "url": "https://api.example.com/endpoint",
    "dump_failures_dir": "./api_failures",
    "total_requests": 500
}
```

### Sample Failure Dump
```
HTTP Benchmark Failure Report
Generated: 2025-01-30 14:30:52
Hash: a1b2c3d4e5f6...

Status Code: 404
Error Message: HTTP 404 response

Response Body:
{
  "error": "Resource not found",
  "message": "The requested endpoint does not exist"
}
```

## 🔄 Variable Replacement

The tool supports dynamic variable replacement in URLs, headers, parameters, and POST data:

- `[test_number]` - Replaced with the request index (0-based)
- `[thread_number]` - Replaced with the worker thread ID

### Example Usage
```bash
./benchmark \
    -url "https://api.example.com/user/[test_number]" \
    -headers "X-Request-ID:req-[test_number]-[thread_number]" \
    -params "worker_id=[thread_number]" \
    -post-data '{"id": [test_number], "thread": [thread_number]}' \
    -total 100
```

**Result**: Each request gets unique values
- Request 0, Thread 0: `user/0`, `req-0-0`, `worker_id=0`, `{"id": 0, "thread": 0}`
- Request 5, Thread 2: `user/5`, `req-5-2`, `worker_id=2`, `{"id": 5, "thread": 2}`

## 📊 Example Scenarios

### API Load Testing
```json
{
    "url": "https://api.myservice.com/users",
    "method": "GET",
    "total_requests": 10000,
    "parallel_count": 100,
    "timeout": "30s",
    "auth_token": "your-api-token",
    "headers": {
        "Accept": "application/json",
        "X-Client-ID": "benchmark-client"
    },
    "parameters": {
        "page": "[test_number]",
        "limit": "50"
    }
}
```

### User Registration Simulation
```json
{
    "url": "https://api.myservice.com/register",
    "method": "POST",
    "total_requests": 1000,
    "parallel_count": 20,
    "timeout": "45s",
    "content_type": "application/json",
    "post_data": "{\"username\": \"user[test_number]\", \"email\": \"user[test_number]@example.com\", \"worker_id\": [thread_number]}"
}
```

### File Upload Testing
```bash
# Create POST data file
echo '{"file_name": "upload[test_number].txt", "worker": [thread_number]}' > upload_data.json

# Run benchmark
./benchmark \
    -url "https://api.myservice.com/upload" \
    -method POST \
    -post-file upload_data.json \
    -headers "Authorization:Bearer token123" \
    -total 500
```

## 📈 Sample Output

```
Starting HTTP benchmark...
URL: https://api.example.com/users
Method: GET
Total requests: 1000
Parallel requests: 50
Timeout: 30s
Failure dump directory: ./failures
Custom headers: 2
  Accept: application/json
  X-Request-ID: bench-[test_number]-[thread_number]
----------------------------------------

Failure dump summary: 3 unique failure types saved to ./failures

========== BENCHMARK RESULTS ==========
Total requests:      1000
Successful requests: 995
Failed requests:     5
Success rate:        99.50%
----------------------------------------
Total time:          15.234s
Requests per second: 65.34
----------------------------------------
Response times:
  Average:           743ms
  Minimum:           156ms
  Maximum:           2.1s
  50th percentile:   682ms
  95th percentile:   1.2s
  99th percentile:   1.8s
========================================
```

## 🏗️ Advanced Examples

### Multi-Stage API Testing
```bash
# Stage 1: Create users
./benchmark -config create_users.json

# Stage 2: Login users  
./benchmark -config login_users.json

# Stage 3: Fetch user data
./benchmark -config fetch_data.json
```

### Rate Limiting Testing
```bash
# Test different concurrency levels
for workers in 10 20 50 100; do
    echo "Testing with $workers workers..."
    ./benchmark -url "https://api.example.com/data" -parallel $workers -total 1000
    sleep 5
done
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup
```bash
# Clone your fork
git clone https://github.com/yourusername/http-benchmark.git
cd http-benchmark

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build
go build -o benchmark main.go
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🐛 Issues and Support

- 🐛 [Report a Bug](https://github.com/warkanum/go-http-bench/issues/new?template=bug_report.md)
- 💡 [Request a Feature](https://github.com/warkanum/go-http-bench/issues/new?template=feature_request.md)
- 💬 [Join Discussions](https://github.com/warkanum/go-http-bench/discussions)

## ⭐ Show Your Support

Give a ⭐️ if this project helped you!

