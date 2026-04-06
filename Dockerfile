# Build the manager binary
FROM golang:1.26.1-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager cmd/main.go

# Use alpine as base image for shell support (required for Vault agent injection)
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 65532 nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
