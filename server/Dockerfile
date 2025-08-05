FROM --platform=linux/x86_64 node:24-alpine AS types

WORKDIR /types
COPY ./server/.npmrc /types/.npmrc
COPY ./server/package.json /types/package.json

# Configure npm to use GitHub token for private packages
RUN --mount=type=secret,id=GITHUB_TOKEN \
    GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)" npm install --omit=dev
RUN rm -f ~/.npmrc

# Delete TypeScript compiler JS files but KEEP the .d.ts files (they provide core types)
RUN find node_modules -path "*/typescript/lib/*.js" -delete

# Delete other unnecessary files but keep JS files for bundling
RUN find node_modules -name "*.js.map" -delete && \
    find node_modules -name "*.ts" -not -name "*.d.ts" -delete && \
    find node_modules -name "*.md" -delete && \
    find node_modules -name "*.txt" -delete && \
    find node_modules -name "LICENSE*" -delete && \
    find node_modules -name "*.yml" -delete && \
    find node_modules -name "*.yaml" -delete && \
    find node_modules -name ".npmignore" -delete && \
    find node_modules -name ".gitignore" -delete && \
    find node_modules -type d -name "test" -exec rm -rf {} + 2>/dev/null || true && \
    find node_modules -type d -name "tests" -exec rm -rf {} + 2>/dev/null || true && \
    find node_modules -type d -name "__tests__" -exec rm -rf {} + 2>/dev/null || true && \
    find node_modules -type d -name "docs" -exec rm -rf {} + 2>/dev/null || true && \
    find node_modules -type d -name ".github" -exec rm -rf {} + 2>/dev/null || true

# Debug: Show what packages we have
RUN echo "=== Packages installed ==="
RUN ls -la node_modules/
RUN echo "=== @crayonnow contents ==="
RUN ls -la node_modules/@crayonnow/ || echo "No @crayonnow"
RUN echo "=== @crayonnow/core contents ==="
RUN ls -la node_modules/@crayonnow/core/ || echo "No @crayonnow/core"
RUN echo "=== @crayonnow/core package.json ==="
RUN cat node_modules/@crayonnow/core/package.json | grep -E '"main"|"exports"' || echo "No package.json"
RUN echo "=== react contents ==="
RUN ls -la node_modules/react/ || echo "No react"
RUN echo "=== typescript contents ==="
RUN ls -la node_modules/typescript/ || echo "No typescript"
RUN echo "=== File counts ==="
RUN find node_modules -type f -name "*.js" | wc -l
RUN find node_modules -type f -name "*.d.ts" | wc -l
RUN find node_modules -type f -name "package.json" | wc -l

FROM --platform=linux/x86_64 golang:1.24-bookworm AS build

WORKDIR /workspace

# Copy only what's needed for the Go module
COPY ./go.mod /workspace/go.mod
COPY ./go.sum /workspace/go.sum
COPY ./internal /workspace/internal
COPY ./server /workspace/server

# Set working directory to the server subdirectory
WORKDIR /workspace/server

# Copy node_modules from the types stage
COPY --from=types /types/node_modules /workspace/server/node_modules

RUN set -xe; \
    go build \
    -mod=readonly \
    -buildmode=pie \
    -ldflags "-linkmode external -extldflags -static-pie" \
    -tags netgo \
    -o /server .

FROM scratch

COPY --from=build /server /server
COPY --from=build /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/
COPY --from=build /lib64/ld-linux-x86-64.so.2 /lib64/
COPY --from=types /types/node_modules /node_modules

CMD ["/server"]
