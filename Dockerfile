# Build the manager binary
FROM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go.mod/sum first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager cmd/main.go

# Use alpine as base image for shell support (required for Vault agent injection)
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 65532 nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
