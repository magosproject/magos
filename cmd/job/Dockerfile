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
    go build -ldflags="-s -w" -o job cmd/job/main.go

FROM alpine:3.19
RUN apk add --no-cache git ca-certificates

WORKDIR /
COPY --from=builder /workspace/job /usr/local/bin/job

ENTRYPOINT ["/usr/local/bin/job"]
