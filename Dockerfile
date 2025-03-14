# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o website-analyzer ./cmd/server/main.go

# Final stage
FROM alpine:3.17

WORKDIR /app

# Install certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/website-analyzer .

# Create a non-root user and switch to it
RUN adduser -D -g '' appuser
USER appuser

# Command to run the executable
ENTRYPOINT ["./website-analyzer"]