BINARY := go-http-bench
BIN_DIR := bin

.PHONY: all build test vet fmt clean

all: vet test build

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -f $(BIN_DIR)/$(BINARY)
