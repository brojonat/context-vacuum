.PHONY: test
test:
	go test ./... -v -race -cover

.PHONY: test-integration
test-integration:
	go test ./... -v -tags=integration

.PHONY: lint
lint:
	golangci-lint run

.PHONY: build
build:
	go build -o bin/context-vacuum ./cmd/context-vacuum

.PHONY: build-all
build-all:
	GOOS=darwin GOARCH=amd64 go build -o bin/context-vacuum-darwin-amd64 ./cmd/context-vacuum
	GOOS=darwin GOARCH=arm64 go build -o bin/context-vacuum-darwin-arm64 ./cmd/context-vacuum
	GOOS=linux GOARCH=amd64 go build -o bin/context-vacuum-linux-amd64 ./cmd/context-vacuum
	GOOS=windows GOARCH=amd64 go build -o bin/context-vacuum-windows-amd64.exe ./cmd/context-vacuum

.PHONY: dev
dev:
	air

.PHONY: sqlc-generate
sqlc-generate:
	sqlc generate

.PHONY: sqlc-verify
sqlc-verify:
	sqlc verify

.PHONY: install
install:
	go install ./cmd/context-vacuum

.PHONY: clean
clean:
	rm -rf bin/
	rm -f ~/.context-vacuum/cache.db

.PHONY: pre-commit
pre-commit: test lint
