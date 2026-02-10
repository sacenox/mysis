.PHONY: fmt build run test test-integration install clean db-reset-accounts check-upstream-version mcp_client

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
DB_PATH ?= $(HOME)/.mysis/mysis.db
ACCOUNTS_EXPORT ?= accounts-backup.sql

fmt:
	go fmt ./...

build:
	go build $(LDFLAGS) -o bin/mysis ./cmd/mysis

run: build
	./bin/mysis

install: build
	mkdir -p $(HOME)/.mysis/bin
	cp bin/mysis $(HOME)/.mysis/bin/mysis

test:
	go test -race -timeout=60s -coverprofile=coverage.out ./internal/**
	go tool cover -func=coverage.out
	@echo ""
	@echo "package coverage:"
	@go tool cover -func=coverage.out | grep "^total:"

clean:
	rm -rf bin/ coverage.out
