package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/github/github-mcp-server/internal/ghmcp"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func extractBearer(auth string) string {
    if auth == "" {
        return ""
    }
    parts := strings.SplitN(auth, " ", 2)
    if len(parts) != 2 {
        return ""
    }
    if strings.ToLower(parts[0]) != "bearer" {
        return ""
    }
    return strings.TrimSpace(parts[1])
}

// writerAdapter wraps http.ResponseWriter into an io.WriteCloser for mcp IOTransport.
type writerAdapter struct {
    w       http.ResponseWriter
    flusher http.Flusher
    closed  chan struct{}
}

func newWriterAdapter(w http.ResponseWriter) *writerAdapter {
    f, _ := w.(http.Flusher)
    return &writerAdapter{w: w, flusher: f, closed: make(chan struct{})}
}

func (wa *writerAdapter) Write(p []byte) (int, error) {
    n, err := wa.w.Write(p)
    if wa.flusher != nil {
        wa.flusher.Flush()
    }
    return n, err
}

func (wa *writerAdapter) Close() error {
    select {
    case <-wa.closed:
        return nil
    default:
        close(wa.closed)
        return nil
    }
}

func main() {
    var (
        listenAddr = flag.String("listen", ":8080", "address to listen on")
        host       = flag.String("host", "", "GitHub host (e.g. https://github.com)")
        version    = flag.String("version", "mini-mcp-http", "server version string")
        tokenMapEnv = flag.String("token-map-env", "GITHUB_MCP_TOKEN_MAP", "env var that contains JSON mapping of MCP token -> GitHub token")
    )
    flag.Parse()

    ts, err := ghmcp.NewEnvTokenStoreFromEnv(*tokenMapEnv)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load token map from env %s: %v\n", *tokenMapEnv, err)
        os.Exit(2)
    }

    // Create server with no global token (per-connection tokens expected)
    cfg := ghmcp.MCPServerConfig{
        Version: *version,
        Host:    *host,
        Token:   "",
        TokenStore: ts,
        Logger:  nil,
    }

    ghServer, err := ghmcp.NewMCPServer(cfg)
    if err != nil {
        log.Fatalf("failed to create MCP server: %v", err)
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
        // Only allow POST
        if r.Method != http.MethodPost {
            http.Error(w, "only POST supported", http.StatusMethodNotAllowed)
            return
        }

        auth := r.Header.Get("Authorization")
        mcpToken := extractBearer(auth)
        if mcpToken == "" {
            http.Error(w, "missing Authorization Bearer token", http.StatusUnauthorized)
            return
        }

        // Resolve GitHub token via TokenStore
        ghToken := ""
        if ts != nil {
            if t, ok := ts.GetGitHubToken(mcpToken); ok {
                ghToken = t
            } else {
                http.Error(w, "invalid token", http.StatusUnauthorized)
                return
            }
        } else {
            // If no TokenStore configured, treat incoming bearer as a direct GitHub token
            ghToken = mcpToken
        }

        // Prepare response headers for a streaming connection
        w.Header().Set("Content-Type", "application/octet-stream")
        w.Header().Set("Transfer-Encoding", "chunked")
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusOK)

        // Build a context with the resolved GitHub token
        ctx := ghmcp.ContextWithGitHubToken(r.Context(), ghToken)
        // Ensure GitHub errors are enabled in context
        ctx = r.Context()
        ctx = ghmcp.ContextWithGitHubToken(ctx, ghToken)

        wa := newWriterAdapter(w)
        transport := &mcp.IOTransport{Reader: r.Body, Writer: wa}

        // Run the MCP server for this connection; this call will block until the
        // client disconnects or the server returns an error.
        if err := ghServer.Run(ctx, transport); err != nil {
            log.Printf("mcp run error: %v", err)
        }

        // Close writer adapter
        _ = wa.Close()
    })

    srv := &http.Server{
        Addr:    *listenAddr,
        Handler: mux,
        ReadTimeout:  5 * time.Minute,
        WriteTimeout: 0,
        IdleTimeout:  5 * time.Minute,
    }

    log.Printf("mini-mcp-http listening on %s", *listenAddr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("listen error: %v", err)
    }
}
