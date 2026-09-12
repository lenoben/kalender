# Stage 1: Build the Go application binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and ca-certificates for downloading dependencies if needed
RUN apk add --no-cache git ca-certificates

# Copy go.mod and go.sum (if present)
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build lightweight static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/main.go

# Stage 2: Minimal production runtime
FROM alpine:latest
s
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy binary and assets from builder
COPY --from=builder /app/server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/.env.example .env

# Expose HTTP port
EXPOSE 10000

# Run the binary
CMD ["./server"]
