# Stage 1: Build the Go application binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install ca-certificates and git
RUN apk add --no-cache git ca-certificates

# Copy go.mod and dependencies
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/main.go

# Stage 2: Production runtime
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy binary and assets
COPY --from=builder /app/server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/.env.example .env

EXPOSE 8080

CMD ["./server"]
