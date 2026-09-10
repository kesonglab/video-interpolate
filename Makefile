.PHONY: build test lint vet run clean

BINARY := vif
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
  -X github.com/kesonglab/video-interpolate/internal/version.Version=$(VERSION) \
  -X github.com/kesonglab/video-interpolate/internal/version.Commit=$(COMMIT) \
  -X github.com/kesonglab/video-interpolate/internal/version.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/vif

test:
	go test ./...

run: build
	./bin/$(BINARY)

clean:
	rm -rf bin/

vet:
	go vet ./...