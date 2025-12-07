package main

import (
    "flag"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/github/github-mcp-server/internal/ghmcp"
    "github.com/github/github-mcp-server/pkg/github"
)

func main() {
    var (
        host               = flag.String("host", "", "GitHub host (e.g. https://github.com or your enterprise host)")
        token              = flag.String("token", "", "GitHub personal access token (or set GITHUB_PERSONAL_ACCESS_TOKEN)")
        toolsets           = flag.String("toolsets", "", "Comma-separated list of toolset IDs to enable (default: default toolset)")
        tools              = flag.String("tools", "", "Comma-separated list of specific tools to enable")
        dynamicToolsets    = flag.Bool("dynamic-toolsets", false, "Enable dynamic tool discovery")
        readOnly           = flag.Bool("read-only", false, "Run server in read-only mode")
        logFile            = flag.String("log-file", "", "Path to log file (default: stderr)")
        contentWindowSize  = flag.Int("content-window-size", 5000, "Content window size for large responses")
        lockdownMode       = flag.Bool("lockdown-mode", false, "Enable lockdown mode")
        repoAccessCacheTTL = flag.Duration("repo-access-cache-ttl", 5*time.Minute, "Repo access cache TTL (e.g. 1m, 0s to disable)")
    )
    flag.Parse()

    // token can be provided by env var
    tok := *token
    if tok == "" {
        tok = os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")
    }
    if tok == "" {
        fmt.Fprintln(os.Stderr, "GITHUB_PERSONAL_ACCESS_TOKEN not set; pass --token or set the env var")
        os.Exit(2)
    }

    var enabledToolsets []string
    if *toolsets != "" {
        for _, t := range strings.Split(*toolsets, ",") {
            if s := strings.TrimSpace(t); s != "" {
                enabledToolsets = append(enabledToolsets, s)
            }
        }
    }

    var enabledTools []string
    if *tools != "" {
        for _, t := range strings.Split(*tools, ",") {
            if s := strings.TrimSpace(t); s != "" {
                enabledTools = append(enabledTools, s)
            }
        }
    }

    // Default toolset when none specified
    if len(enabledToolsets) == 0 && len(enabledTools) == 0 {
        enabledToolsets = []string{github.ToolsetMetadataDefault.ID}
    }

    cfg := ghmcp.StdioServerConfig{
        Version:              "mini-mcp",
        Host:                 *host,
        Token:                tok,
        EnabledToolsets:      enabledToolsets,
        EnabledTools:         enabledTools,
        DynamicToolsets:      *dynamicToolsets,
        ReadOnly:             *readOnly,
        LogFilePath:          *logFile,
        ContentWindowSize:    *contentWindowSize,
        LockdownMode:         *lockdownMode,
        RepoAccessCacheTTL:   repoAccessCacheTTL,
    }

    if err := ghmcp.RunStdioServer(cfg); err != nil {
        fmt.Fprintf(os.Stderr, "error running mini-mcp: %v\n", err)
        os.Exit(1)
    }
}
