FROM --platform=linux/x86_64 node:24-alpine AS types

WORKDIR /types
COPY ./server/.npmrc /types/.npmrc
COPY ./server/package.json /types/package.json

# Configure npm to use GitHub token for private packages
RUN --mount=type=secret,id=GITHUB_TOKEN \
    GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)" npm install --omit=dev
RUN rm -f ~/.npmrc
RUN find node_modules -name "*.js" -not -path "*/package.json" -delete
RUN find node_modules -name "*.js.map" -delete

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
