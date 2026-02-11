.PHONY: fmt build run test lint install clean backup-credentials

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
DB_PATH ?= $(HOME)/.config/mysis/mysis.db
CREDENTIALS_BACKUP ?= credentials-backup-$(shell date +%Y%m%d-%H%M%S).sql

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

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest; \
	fi
	$$(go env GOPATH)/bin/golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out

backup-credentials:
	@echo "Backing up credentials from $(DB_PATH)..."
	@if [ ! -f "$(DB_PATH)" ]; then \
		echo "Error: Database not found at $(DB_PATH)"; \
		exit 1; \
	fi
	@sqlite3 "$(DB_PATH)" "SELECT 'INSERT INTO session_credentials VALUES(' || quote(session_id) || ',' || quote(username) || ',' || quote(password) || ',' || quote(created_at) || ',' || quote(updated_at) || ');' FROM session_credentials;" > "$(CREDENTIALS_BACKUP)"
	@echo "Credentials backed up to $(CREDENTIALS_BACKUP)"
	@echo ""
	@echo "Backup contains $(shell wc -l < $(CREDENTIALS_BACKUP)) credential(s)"
	@echo ""
	@echo "To restore (merge into existing table):"
	@echo "  sqlite3 $(DB_PATH) < $(CREDENTIALS_BACKUP)"
