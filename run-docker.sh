#!/bin/bash

# Start the Docker containers
echo "Starting PostgreSQL and Redis with Docker Compose..."
docker-compose up -d postgres redis

# Wait for services to be ready
echo "Waiting for PostgreSQL to be ready..."
until docker exec website-analyzer-postgres pg_isready -U postgres > /dev/null 2>&1; do
  echo -n "."
  sleep 1
done
echo " PostgreSQL is ready!"

echo "Waiting for Redis to be ready..."
until docker exec website-analyzer-redis redis-cli ping > /dev/null 2>&1; do
  echo -n "."
  sleep 1
done
echo " Redis is ready!"

# Update .env file with Docker container addresses
cat > .env <<EOF
PORT=8080
ENVIRONMENT=development
POSTGRES_URI=postgres://postgres:postgres@localhost:5432/website_analyzer?sslmode=disable
REDIS_URI=redis://localhost:6379/0
JWT_SECRET=your-secret-key-change-this-in-production
JWT_EXPIRATION_HOURS=24
OPENAI_API_KEY=your-openai-api-key
ANALYSIS_TIMEOUT=60
EOF

echo "Docker containers are running and .env is configured!"
echo "You can now run the application with: go run cmd/server/main.go"
echo ""
echo "To build and run the application in Docker:"
echo "  docker-compose up -d"
echo ""
echo "To stop the containers:"
echo "  docker-compose down"