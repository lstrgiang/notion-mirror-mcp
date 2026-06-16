.PHONY: build install tidy lint

# FTS5 requires the sqlite_fts5 build tag (included in mattn/go-sqlite3's opt-in C file)
BUILD_TAGS := sqlite_fts5

build:
	go build -tags '$(BUILD_TAGS)' -o notion-mirror-mcp ./cmd/server

install:
	go install -tags '$(BUILD_TAGS)' ./cmd/server

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
