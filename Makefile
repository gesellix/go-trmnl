BINARY := trmnld
BUILD_DIR := build

.PHONY: all build run test lint vet fmt tidy vuln clean

all: lint test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) ./cmd/trmnld

run:
	go run ./cmd/trmnld

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out
