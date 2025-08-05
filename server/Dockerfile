FROM --platform=linux/x86_64 node:24-alpine AS types

WORKDIR /types
COPY ./package.json /types/package.json

# Configure npm to use GitHub token for private packages
RUN --mount=type=secret,id=GITHUB_TOKEN \
    if [ -f /run/secrets/GITHUB_TOKEN ]; then \
        echo "//npm.pkg.github.com/:_authToken=$(cat /run/secrets/GITHUB_TOKEN)" > ~/.npmrc && \
        echo "@crayonnow:registry=https://npm.pkg.github.com" >> ~/.npmrc; \
    fi && \
    npm install --production=false && \
    rm -f ~/.npmrc

FROM --platform=linux/x86_64 golang:1.24-bookworm AS build

WORKDIR /src

COPY ./server.go /src/server.go
COPY ./go.mod /src/go.mod
COPY ./go.sum /src/go.sum
COPY ./vendor /src/vendor
COPY --from=types /types/node_modules /src/node_modules

RUN set -xe; \
    go build \
      -mod=vendor \
      -buildmode=pie \
      -ldflags "-linkmode external -extldflags -static-pie" \
      -tags netgo \
      -o /server . \
    ;

FROM scratch

COPY --from=build /server /server
COPY --from=build /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/
COPY --from=build /lib64/ld-linux-x86-64.so.2 /lib64/
COPY --from=types /types/node_modules /node_modules

CMD ["/server"]
