# Build the manager binary
FROM golang:1.26.1-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY types/ types/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o job cmd/job/main.go

FROM golang:1.26.1-alpine AS bgit-builder
ARG TARGETOS
ARG TARGETARCH
ARG BGIT_REF=1.3.6

RUN apk add --no-cache git

WORKDIR /workspace/bgit
RUN git clone --depth=1 --branch ${BGIT_REF} https://github.com/bucketgit/bgit.git .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X main.version=${BGIT_REF}" -o /out/bgit .

FROM alpine:3.19
RUN apk add --no-cache git ca-certificates curl

# Install Kyverno CLI for JSON policy validation
RUN curl -sSL https://github.com/kyverno/kyverno/releases/download/v1.17.0/kyverno-cli_v1.17.0_linux_x86_64.tar.gz | tar -xz && \
    mv kyverno /usr/local/bin/

WORKDIR /
COPY --from=builder /workspace/job /usr/local/bin/job
COPY --from=bgit-builder /out/bgit /usr/local/bin/bgit
RUN ln -s /usr/local/bin/bgit /usr/local/bin/git-remote-bgit

ENTRYPOINT ["/usr/local/bin/job"]
