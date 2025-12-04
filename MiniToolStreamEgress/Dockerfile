# Build stage
FROM golang:1.25.2-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/minitoolstream-egress \
    ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/minitoolstream-egress .

# Copy default config (optional)
# COPY --from=builder /app/configs/config.yaml /etc/minitoolstream/config.yaml

# Change ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose gRPC port
EXPOSE 50052

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD grpc_health_probe -addr=:50052 || exit 1

# Run the application
ENTRYPOINT ["/app/minitoolstream-egress"]
# CMD ["-config", "/etc/minitoolstream/config.yaml"]
