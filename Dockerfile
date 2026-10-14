# Multi-stage build for Syragen Telegram Bot
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install build dependencies and certificates
RUN apk add --no-cache ca-certificates

# Cache go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/bot ./cmd/bot

# Runtime stage
FROM alpine:3.21

# Install runtime timezone data and certificates
RUN apk add --no-cache ca-certificates tzdata

# Set Moscow timezone
ENV TZ=Europe/Moscow

WORKDIR /app

# Create directories
RUN mkdir -p /app/data /app/runtime/media

# Copy binary from builder
COPY --from=builder /bin/bot /app/bot

VOLUME ["/app/data", "/app/runtime"]

ENTRYPOINT ["/app/bot"]

