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


# Install PostgreSQL client tools (pg_isready)
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

# Copy wait script
COPY wait-for-postgres.sh /usr/local/bin/wait-for-postgres.sh
RUN chmod +x /usr/local/bin/wait-for-postgres.sh

# Copy built binary file from the builder stage
COPY --from=builder /app/server .

# Expose application port
EXPOSE 8080


# Start the server via wait script
CMD ["wait-for-postgres.sh", "db", "./server"]
