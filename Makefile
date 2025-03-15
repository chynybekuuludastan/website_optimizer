.PHONY: build run clean test swagger docker docker-compose help migrate migrate-up migrate-down migrate-reset migrate-status db-setup db-seed dev-tools lint fmt build-all

# Build the application
build:
	go build -o bin/website-analyzer cmd/server/main.go

# Build all executables
build-all:
	go build -o bin/website-analyzer cmd/server/main.go
	go build -o bin/migrate cmd/migrate/main.go

# Run the application
run:
	go run cmd/server/main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Generate Swagger documentation
swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/server/main.go -o docs

# Migration commands
migrate-up:
	@mkdir -p bin
	@if [ ! -f bin/migrate ]; then go build -o bin/migrate cmd/migrate/main.go; fi
	bin/migrate --migrate

migrate-down:
	@mkdir -p bin
	@if [ ! -f bin/migrate ]; then go build -o bin/migrate cmd/migrate/main.go; fi
	bin/migrate --rollback

migrate-reset:
	@mkdir -p bin
	@if [ ! -f bin/migrate ]; then go build -o bin/migrate cmd/migrate/main.go; fi
	bin/migrate --reset

migrate-status:
	@mkdir -p bin
	@if [ ! -f bin/migrate ]; then go build -o bin/migrate cmd/migrate/main.go; fi
	bin/migrate --status

# Database setup and seed
db-setup: migrate-up

db-seed:
	@echo "Seeding database with default data..."
	go run cmd/server/main.go --seed-only

# Development tools
dev-tools:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	goimports -w .

# Docker commands
docker:
	docker build -t website-analyzer .

# Start only database services
docker-db:
	docker-compose up -d postgres redis

# Build and run with Docker Compose
docker-compose:
	docker-compose up -d

# Stop Docker Compose
docker-compose-stop:
	docker-compose down

# Run migrations in Docker
docker-migrate:
	docker-compose run --rm app bin/migrate --migrate

# Development environment setup
dev-setup: dev-tools docker-db db-setup

# Help command
help:
	@echo "Website Analyzer - Development Commands"
	@echo ""
	@echo "Build and Run:"
	@echo "  make build           - Build the application"
	@echo "  make build-all       - Build all executables (app and migration tool)"
	@echo "  make run             - Run the application"
	@echo "  make clean           - Clean build artifacts"
	@echo ""
	@echo "Testing and Quality:"
	@echo "  make test            - Run tests"
	@echo "  make test-coverage   - Run tests with coverage report"
	@echo "  make lint            - Run linter"
	@echo "  make fmt             - Format code"
	@echo "  make swagger         - Generate Swagger documentation"
	@echo ""
	@echo "Database Management:"
	@echo "  make migrate-up      - Run all pending migrations"
	@echo "  make migrate-down    - Rollback the last batch of migrations"
	@echo "  make migrate-reset   - Reset all migrations (rollback and re-apply)"
	@echo "  make migrate-status  - Show migration status"
	@echo "  make db-setup        - Setup database (run migrations)"
	@echo "  make db-seed         - Seed database with initial data"
	@echo ""
	@echo "Docker:"
	@echo "  make docker          - Build Docker image"
	@echo "  make docker-db       - Start database services with Docker Compose"
	@echo "  make docker-compose  - Start all services with Docker Compose"
	@echo "  make docker-compose-stop - Stop all Docker Compose services"
	@echo "  make docker-migrate  - Run migrations in Docker environment"
	@echo ""
	@echo "Development Environment:"
	@echo "  make dev-tools       - Install development tools"
	@echo "  make dev-setup       - Setup complete development environment"