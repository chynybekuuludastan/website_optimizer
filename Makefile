.PHONY: build run clean test swagger docker docker-compose help

# Build the application
build:
	go build -o bin/website-analyzer cmd/server/main.go

# Run the application
run:
	go run cmd/server/main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests
test:
	go test ./...

# Generate Swagger documentation
swagger:
	go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g cmd/server/main.go -o docs

# Build and run with Docker Compose
docker-compose:
	docker-compose up -d

# Build docker image
docker:
	docker build -t website-analyzer .

# Help command
help:
	@echo "Available commands:"
	@echo "  make build           - Build the application"
	@echo "  make run             - Run the application"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make test            - Run tests"
	@echo "  make swagger         - Generate Swagger documentation"
	@echo "  make docker          - Build Docker image"
	@echo "  make docker-compose  - Run with Docker Compose"