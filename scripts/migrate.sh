#!/bin/bash

# scripts/migrate.sh
# Script to manage database migrations

# Check if the action is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 [action]"
    echo "Actions:"
    echo "  up      - Apply all pending migrations"
    echo "  down    - Rollback the last batch of migrations"
    echo "  reset   - Rollback all migrations and re-apply them"
    echo "  status  - Show migration status"
    exit 1
fi

# Go to the root directory of the project
cd "$(dirname "$0")/.."

# Check if .env file exists
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    echo "Please create a .env file with your database settings"
    exit 1
fi

# Set GOPATH if not already set
export GOPATH=${GOPATH:-$HOME/go}

# Build the migration tool
echo "Building migration tool..."
go build -o bin/migrate cmd/migrate/main.go

# Run the migration tool with the specified action
case "$1" in
    up|migrate)
        echo "Running migrations..."
        ./bin/migrate --migrate
        ;;
    down|rollback)
        echo "Rolling back the last batch of migrations..."
        ./bin/migrate --rollback
        ;;
    reset)
        echo "Resetting all migrations..."
        ./bin/migrate --reset
        ;;
    status)
        echo "Showing migration status..."
        ./bin/migrate --status
        ;;
    *)
        echo "Unknown action: $1"
        echo "Available actions: up, down, reset, status"
        exit 1
        ;;
esac

# Check if migration was successful
if [ $? -eq 0 ]; then
    echo "Migration action completed successfully"
else
    echo "Migration action failed"
    exit 1
fi