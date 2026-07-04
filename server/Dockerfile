FROM golang:1.26-bookworm AS build

# Accept provenance as build arguments
ARG GIT_COMMIT=unknown
ARG SERVER_VERSION=0.0.0-dev

WORKDIR /workspace

# First, copy the parent module that's referenced by replace directive
# Only copy go.mod/go.sum and minimal source needed for module resolution
COPY ./go.mod /workspace/go.mod
COPY ./go.sum /workspace/go.sum
# Create empty internal directory structure so the module is valid
RUN mkdir -p /workspace/internal

# Now copy server's go.mod and go.sum
COPY ./server/go.mod /workspace/server/go.mod
COPY ./server/go.sum /workspace/server/go.sum

# Download dependencies - this will cache external deps even with replace directive
WORKDIR /workspace/server
RUN go mod download -x

# Now copy the actual source code
WORKDIR /workspace
COPY ./internal /workspace/internal
COPY ./server /workspace/server

# Set working directory back to the server subdirectory
WORKDIR /workspace/server

RUN set -xe; \
    go build \
    -mod=readonly \
    -buildmode=pie \
    -ldflags "-linkmode external -extldflags -static-pie -X main.gitCommit=${GIT_COMMIT} -X main.serverVersion=${SERVER_VERSION}" \
    -tags netgo \
    -o /server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl libgcc libstdc++ unzip && \
    ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then BUN_ARCH="x64"; else BUN_ARCH="aarch64"; fi && \
    curl -fsSL "https://github.com/oven-sh/bun/releases/latest/download/bun-linux-${BUN_ARCH}-musl.zip" -o /tmp/bun.zip && \
    unzip /tmp/bun.zip -d /tmp && \
    mv /tmp/bun-linux-${BUN_ARCH}-musl/bun /usr/local/bin/bun && \
    rm -rf /tmp/bun.zip /tmp/bun-linux-*

COPY --from=build /server /server

# Environment variables for S3 configuration
# These will be provided at runtime:
# AWS_ACCESS_KEY_ID
# AWS_SECRET_ACCESS_KEY
# AWS_REGION
# AWS_ENDPOINT_URL_S3 (optional, for S3-compatible services)
# S3_BUCKET (required)
# CACHE_SIZE (optional, defaults to 32MB)

CMD ["/server"]
