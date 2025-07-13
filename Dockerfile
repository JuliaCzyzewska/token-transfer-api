# Build
FROM golang:1.24 AS builder

WORKDIR /app

# Download Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Build the Go binary
RUN go build -o server .


# Run
FROM debian:bookworm-slim

WORKDIR /app

# Copy built binary and .env file from the builder stage
COPY --from=builder /app/server .
COPY .env .env

# Expose application port
EXPOSE 8080

# Start the server
CMD ["./server"]
