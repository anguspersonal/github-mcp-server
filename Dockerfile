FROM golang:1.25.4-alpine AS build
ARG VERSION="dev"

WORKDIR /build

# Install git for version info
# Railway requires cache mount IDs to be prefixed with s/<SERVICE_ID>-
# IMPORTANT: Replace YOUR_RAILWAY_SERVICE_ID below with your actual Railway Service ID
# Get it: Railway dashboard → Your service → Press Ctrl+K (Cmd+K on Mac) → "Copy Service ID"
# Example: If your Service ID is "abc123-def456-ghi789", replace YOUR_RAILWAY_SERVICE_ID with: abc123-def456-ghi789
# Note: Railway requires hardcoding the Service ID - build arguments don't work for cache mounts
# See: https://station.railway.com/questions/error-cache-mount-id-is-not-prefixed-wi-12a50b55
# Format: id=s/<SERVICE_ID>-<cache-name>
RUN --mount=type=cache,id=s/266e71f8-400f-4d39-aa57-515a3ff2f083D-apk-cache,target=/var/cache/apk \
    apk add git

# Build mini-mcp-http only
# Railway requires cache mount IDs to be prefixed with s/<SERVICE_ID>-
# IMPORTANT: Replace YOUR_RAILWAY_SERVICE_ID below with your actual Railway Service ID (same as above)
# Format: id=s/<SERVICE_ID>-<cache-name>
RUN --mount=type=cache,id=s/266e71f8-400f-4d39-aa57-515a3ff2f083-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=s/266e71f8-400f-4d39-aa57-515a3ff2f083-go-build,target=/root/.cache/go-build \
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
