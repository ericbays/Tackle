.PHONY: dev test test-integration lint build migrate-up migrate-down

BINARY_NAME = tackle
BINARY_PATH = bin/$(BINARY_NAME)

# Scoped Go package patterns.
GO_PKGS = ./cmd/... ./internal/... ./pkg/...

# Start Docker Compose (database) and backend dev server.
dev:
	docker compose up -d
	@echo "PostgreSQL is up. Starting backend..."
	go run ./cmd/tackle/

# Run all tests.
test:
	go test -race -count=1 $(GO_PKGS)

# Run integration tests against a real PostgreSQL database.
# Requires TEST_DATABASE_URL (or DATABASE_URL) and all migrations applied.
# Example: make test-integration DATABASE_URL=postgres://user:pass@localhost:5432/tackle_test
test-integration:
	TEST_DATABASE_URL=$(DATABASE_URL) go test -tags=integration -v -count=1 ./internal/tests/integration/...

# Run all linters.
lint:
	go vet $(GO_PKGS)

# Build production artifacts.
build:
	@mkdir -p bin
	go build -o $(BINARY_PATH) ./cmd/tackle/
	@echo "Build complete: $(BINARY_PATH)"

# Run database migrations forward.
migrate-up: build
	./$(BINARY_PATH) -migrate-up

# Roll back the last database migration.
migrate-down: build
	./$(BINARY_PATH) -migrate-down
