package main

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strconv"
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

var startTime = time.Now()

// Logger provides structured JSON logging for Railway
type Logger struct {
    writer io.Writer
}

func NewLogger(w io.Writer) *Logger {
    return &Logger{writer: w}
}

func (l *Logger) log(level, component, message string, fields map[string]interface{}) {
    entry := map[string]interface{}{
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "level":     level,
        "component": component,
        "message":   message,
    }
    for k, v := range fields {
        entry[k] = v
    }
    json.NewEncoder(l.writer).Encode(entry)
}

func (l *Logger) Info(component, message string, fields map[string]interface{}) {
    l.log("info", component, message, fields)
}

func (l *Logger) Error(component, message string, fields map[string]interface{}) {
    l.log("error", component, message, fields)
}

func (l *Logger) Warn(component, message string, fields map[string]interface{}) {
    l.log("warn", component, message, fields)
}

func (l *Logger) Debug(component, message string, fields map[string]interface{}) {
    l.log("debug", component, message, fields)
}

func main() {
    var (
        listenAddr = flag.String("listen", "", "address to listen on (overrides PORT env var)")
        host       = flag.String("host", "", "GitHub host (e.g. https://github.com)")
        version    = flag.String("version", "mini-mcp-http", "server version string")
        tokenMapEnv = flag.String("token-map-env", "GITHUB_MCP_TOKEN_MAP", "env var that contains JSON mapping of MCP token -> GitHub token")
    )
    flag.Parse()

    logger := NewLogger(os.Stdout)

    // Determine listen address: flag > PORT env var > default :8080
    addr := *listenAddr
    if addr == "" {
        port := os.Getenv("PORT")
        if port == "" {
            port = "8080"
        }
        addr = ":" + port
    }
    *listenAddr = addr

    logger.Info("server", "Starting mini-mcp-http server", map[string]interface{}{
        "version":     *version,
        "listen_addr": addr,
        "host":        *host,
    })

    envStore, err := ghmcp.NewEnvTokenStoreFromEnv(*tokenMapEnv)
    if err != nil {
        logger.Error("config", "Failed to load token map from environment", map[string]interface{}{
            "env_var": *tokenMapEnv,
            "error":   err.Error(),
        })
        fmt.Fprintf(os.Stderr, "failed to load token map from env %s: %v\n", *tokenMapEnv, err)
        os.Exit(2)
    }

    // If GitHub App credentials are present, prefer InstallationTokenStore which
    // will mint short-lived installation tokens for the mapped installation IDs.
    var ts ghmcp.TokenStore
    appIDStr := os.Getenv("GITHUB_APP_ID")
    pkb64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_B64")
    if appIDStr != "" && pkb64 != "" {
        logger.Info("config", "GitHub App credentials detected, using InstallationTokenStore", map[string]interface{}{
            "app_id": appIDStr,
        })
        appID, err := strconv.ParseInt(appIDStr, 10, 64)
        if err != nil {
            logger.Error("config", "Invalid GITHUB_APP_ID", map[string]interface{}{
                "app_id": appIDStr,
                "error":  err.Error(),
            })
            fmt.Fprintf(os.Stderr, "invalid GITHUB_APP_ID: %v\n", err)
            os.Exit(2)
        }
        pkBytes, err := base64.StdEncoding.DecodeString(pkb64)
        if err != nil {
            // try raw PEM
            pkBytes = []byte(pkb64)
        }
        mapping := envStore.Mapping()
        apiBase := "https://api.github.com/"
        instStore, err := ghmcp.NewInstallationTokenStore(appID, pkBytes, mapping, apiBase)
        if err != nil {
            logger.Error("config", "Failed to initialize installation token store", map[string]interface{}{
                "error": err.Error(),
            })
            fmt.Fprintf(os.Stderr, "failed to initialize installation token store: %v\n", err)
            os.Exit(2)
        }
        ts = instStore
        logger.Info("config", "InstallationTokenStore initialized successfully", nil)
    } else {
        logger.Info("config", "Using EnvTokenStore for token resolution", nil)
        ts = envStore
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
        logger.Error("server", "Failed to create MCP server", map[string]interface{}{
            "error": err.Error(),
        })
        log.Fatalf("failed to create MCP server: %v", err)
    }

    logger.Info("server", "MCP server created successfully", nil)

    mux := http.NewServeMux()
    
    // Health check endpoint
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        uptime := time.Since(startTime).Seconds()
        
        // Check GitHub API reachability
        githubReachable := true
        apiURL := "https://api.github.com/"
        if *host != "" {
            // Use custom host if specified
            apiURL = strings.TrimSuffix(*host, "/") + "/"
            if !strings.HasPrefix(apiURL, "http") {
                apiURL = "https://" + apiURL
            }
        }
        
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()
        
        req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
        if err == nil {
            resp, err := http.DefaultClient.Do(req)
            if err != nil || resp.StatusCode >= 500 {
                githubReachable = false
            }
            if resp != nil {
                resp.Body.Close()
            }
        } else {
            githubReachable = false
        }
        
        health := map[string]interface{}{
            "status":                "healthy",
            "version":               *version,
            "uptime_seconds":        uptime,
            "github_api_reachable":  githubReachable,
        }
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(health)
    })
    
    mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
        requestID := fmt.Sprintf("%d", time.Now().UnixNano())
        
        logger.Info("http", "MCP connection request received", map[string]interface{}{
            "request_id":  requestID,
            "method":      r.Method,
            "remote_addr": r.RemoteAddr,
        })

        // Only allow POST
        if r.Method != http.MethodPost {
            logger.Warn("http", "Invalid HTTP method", map[string]interface{}{
                "request_id": requestID,
                "method":     r.Method,
            })
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusMethodNotAllowed)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error": map[string]interface{}{
                    "type":    "method_not_allowed",
                    "message": "Only POST method is supported for MCP endpoint",
                    "details": map[string]string{
                        "method_received": r.Method,
                        "method_required": "POST",
                    },
                },
            })
            return
        }

        auth := r.Header.Get("Authorization")
        mcpToken := extractBearer(auth)
        if mcpToken == "" {
            logger.Warn("http", "Missing Authorization header", map[string]interface{}{
                "request_id": requestID,
            })
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "error": map[string]interface{}{
                    "type":    "missing_authorization",
                    "message": "Authorization header with Bearer token is required",
                    "details": map[string]string{
                        "header_format": "Authorization: Bearer <token>",
                    },
                },
            })
            return
        }

        // Resolve GitHub token via TokenStore
        ghToken := ""
        if ts != nil {
            if t, ok := ts.GetGitHubToken(mcpToken); ok {
                ghToken = t
                logger.Debug("token", "Token resolved successfully", map[string]interface{}{
                    "request_id": requestID,
                })
            } else {
                logger.Warn("token", "Invalid MCP token", map[string]interface{}{
                    "request_id": requestID,
                })
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusUnauthorized)
                json.NewEncoder(w).Encode(map[string]interface{}{
                    "error": map[string]interface{}{
                        "type":    "invalid_token",
                        "message": "The provided MCP token is not valid or not found in token mapping",
                        "details": map[string]string{
                            "hint": "Verify that your token is correctly configured in GITHUB_MCP_TOKEN_MAP",
                        },
                    },
                })
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

        logger.Info("http", "MCP connection established", map[string]interface{}{
            "request_id": requestID,
        })

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
            logger.Error("mcp", "MCP run error", map[string]interface{}{
                "request_id": requestID,
                "error":      err.Error(),
            })
            log.Printf("mcp run error: %v", err)
        }

        logger.Info("http", "MCP connection closed", map[string]interface{}{
            "request_id": requestID,
        })

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

    logger.Info("server", "Server listening", map[string]interface{}{
        "addr": *listenAddr,
    })
    log.Printf("mini-mcp-http listening on %s", *listenAddr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("server", "Server listen error", map[string]interface{}{
            "error": err.Error(),
        })
        log.Fatalf("listen error: %v", err)
    }
}
