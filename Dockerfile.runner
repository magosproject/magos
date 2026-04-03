# Build the manager binary
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/job/main.go cmd/job/main.go
COPY internal/ internal/
COPY api/ api/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o job cmd/job/main.go

# Use alpine as base image for the final stage to have minimal utilities like git and certificates
FROM alpine:3.19
RUN apk add --no-cache git ca-certificates

WORKDIR /
COPY --from=builder /workspace/job /usr/local/bin/job

ENTRYPOINT ["/usr/local/bin/job"]
