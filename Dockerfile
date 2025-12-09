FROM golang:1.25.4-alpine AS build
ARG VERSION="dev"
# Railway Service ID for cache mount prefixing
# Set this as an environment variable in Railway (Settings → Variables)
# Railway will make it available as a build argument during Docker build
ARG RAILWAY_SERVICE_ID=""

WORKDIR /build

# Install git for version info
# Railway requires cache mount IDs to be prefixed with s/<SERVICE_ID>-
# If RAILWAY_SERVICE_ID is not set, cache mounts will still work but won't be shared across services
RUN --mount=type=cache,id=s/${RAILWAY_SERVICE_ID}-apk-cache,target=/var/cache/apk \
    apk add git

# Build mini-mcp-http only
# Railway requires cache mount IDs to be prefixed with s/<SERVICE_ID>-
RUN --mount=type=cache,id=s/${RAILWAY_SERVICE_ID}-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=s/${RAILWAY_SERVICE_ID}-go-build,target=/root/.cache/go-build \
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
