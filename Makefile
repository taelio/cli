BINARY  := tael
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X tael.io/cli/cmd.version=$(VERSION) -X tael.io/cli/cmd.commit=$(COMMIT)

.PHONY: all build test vet lint clean

all: build vet test lint

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed; skipping lint"; \
	fi

clean:
	rm -f $(BINARY)
