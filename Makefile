BINARY := tp-mcp
VERSION := 1.0.0

.PHONY: build install test clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/tp-mcp/

install:
	go build -ldflags "-X main.version=$(VERSION)" -o /usr/local/bin/$(BINARY) ./cmd/tp-mcp/

test:
	go test ./...

clean:
	rm -f $(BINARY)
