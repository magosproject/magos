# Build stage
FROM golang:1.26.1-alpine AS builder

ARG github_token

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies with verification
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -installsuffix cgo \
    -ldflags="-w -s" \
    -o api cmd/api/main.go

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies and non-root user
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 1000 apiuser && \
    adduser -D -u 1000 -G apiuser apiuser

# Copy binary and docs from builder with correct ownership
COPY --from=builder --chown=apiuser:apiuser /build/api .
# TODO: set-up API docs, build/docs doesn't exist yet
# COPY --from=builder --chown=apiuser:apiuser /build/docs ./docs

# Use non-root user for security
USER apiuser

# Document the exposed port
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Set entrypoint to the API binary
ENTRYPOINT ["./api"]
CMD ["serve"]
