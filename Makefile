.PHONY: build run test test-coverage test-integration clean help dev-up dev-down dev-reset

# Build the application
build:
	@mkdir -p build
	@go build -o build/cito ./server
	@echo "Built binary: build/cito"

# Build and run the application
run: dev-up
	@mkdir -p build
	@go build -o build/cito ./server
	@. ./env.sh && ./build/cito

# Run all unit tests
test:
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@mkdir -p build
	@go test -v -coverprofile=build/coverage.out ./...
	@go tool cover -func=build/coverage.out | grep total

# View coverage report in browser
coverage-html:
	@mkdir -p build
	@go test -coverprofile=build/coverage.out ./...
	@go tool cover -html=build/coverage.out

# Run integration tests (requires Docker)
test-integration:
	@mkdir -p build
	@go test -c -tags=integration -o build/integration_tests.test ./integration_tests
	@build/integration_tests.test -test.v -test.timeout=5m 2>&1 | tee build/integration_test.log
	@echo "\nIntegration test log saved to build/integration_test.log"

# Run all tests (unit + integration)
test-all: test test-integration

# Clean build artifacts
clean:
	@rm -rf build/
	@echo "Cleaned build folder"

# Start dev database
dev-up:
	@docker compose up -d
	@echo "PostgreSQL running on localhost:5442"

# Stop dev database
dev-down:
	@docker compose down

# Reset dev database (destroys all data)
dev-reset:
	@docker compose down -v
	@docker compose up -d
	@echo "Database reset"

# Show help
help:
	@echo "Available targets:"
	@echo "  build            - Build the application to build/cito"
	@echo "  test             - Run unit tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  coverage-html    - View coverage report in browser"
	@echo "  test-integration - Run integration tests (requires Docker)"
	@echo "  test-all         - Run all tests (unit + integration)"
	@echo "  clean            - Remove build artifacts"
	@echo "  dev-up           - Start dev PostgreSQL container"
	@echo "  dev-down         - Stop dev PostgreSQL container"
	@echo "  dev-reset        - Reset dev database (destroys data)"
	@echo "  help             - Show this help message"
