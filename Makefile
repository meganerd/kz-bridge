VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test lint clean

build:
	go build $(LDFLAGS) -o kz-bridge ./cmd/kz-bridge/

test:
	go test ./... -v -race

lint:
	golangci-lint run ./...

clean:
	rm -f kz-bridge
	rm -rf build/
