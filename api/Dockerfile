# Build stage
FROM golang:1.26.1-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Copy root module (needed for replace directive in api/go.mod)
COPY go.mod go.sum ./
COPY types/ types/

# Copy go mod files and download dependencies
COPY api/go.mod api/go.sum api/
RUN cd api && go mod download && go mod verify

# Copy source code
COPY api/ api/

# Build the application with optimizations into a dedicated output path.
RUN mkdir -p /out && cd api && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build \
    -a \
    -installsuffix cgo \
    -ldflags="-w -s" \
    -o /out/api cmd/api/main.go

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install runtime dependencies and non-root user
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 1000 apiuser && \
    adduser -D -u 1000 -G apiuser apiuser

# Copy binary from builder with correct ownership
COPY --from=builder --chown=apiuser:apiuser /out/api /app/api

# Use non-root user for security
USER apiuser

# Document the exposed port
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Set entrypoint to the API binary
ENTRYPOINT ["/app/api"]
CMD ["serve"]
