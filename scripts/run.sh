#!/bin/bash

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go and try again."
    exit 1
fi

# Check if PostgreSQL is running
if ! pg_isready -h localhost -p 5432 &> /dev/null; then
    echo "PostgreSQL is not running. Please start PostgreSQL and try again."
    exit 1
fi

# Check if Redis is running
if ! redis-cli ping &> /dev/null; then
    echo "Redis is not running. Please start Redis and try again."
    exit 1
fi

# Create database if it doesn't exist
PGPASSWORD=postgres psql -h localhost -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'website_analyzer'" | grep -q 1 || PGPASSWORD=postgres psql -h localhost -U postgres -c "CREATE DATABASE website_analyzer"

# Download dependencies
echo "Downloading dependencies..."
go mod tidy

# Build and run the application
echo "Building and starting the server..."
go run cmd/server/main.go