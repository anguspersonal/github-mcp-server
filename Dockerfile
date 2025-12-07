FROM golang:1.25.4-alpine AS build
ARG VERSION="dev"

WORKDIR /build

# Install git for version info
RUN --mount=type=cache,id=apk-cache,target=/var/cache/apk \
    apk add git

# Build mini-mcp-http only
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bin/mini-mcp-http \
    cmd/mini-mcp-http/main.go

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /server
COPY --from=build /bin/mini-mcp-http .

ENTRYPOINT ["/server/mini-mcp-http"]
CMD ["-listen", ":8080"]
