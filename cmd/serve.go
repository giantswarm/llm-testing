package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/giantswarm/llm-testing/internal/kserve"
	"github.com/giantswarm/llm-testing/internal/llm"
	mcptools "github.com/giantswarm/llm-testing/internal/mcp"
	"github.com/giantswarm/llm-testing/internal/server"
)

const (
	transportStdio          = "stdio"
	transportStreamableHTTP = "streamable-http"
)

func newServeCmd() *cobra.Command {
	var (
		transport    string
		httpAddr     string
		httpEndpoint string
		inCluster    bool
		outputDir    string
		suitesDir    string
		debug        bool

		// OAuth options (simplified from mcp-kubernetes).
		enableOAuth    bool
		oauthBaseURL   string
		oauthProvider  string
		dexIssuerURL   string
		dexClientID    string
		dexClientSecret string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start the MCP server to expose LLM testing tools via the Model Context Protocol.

Supports multiple transport types:
  - stdio: Standard input/output (default, for IDE integration)
  - streamable-http: HTTP with streaming support (for remote access)

When using streamable-http transport, OAuth 2.1 authentication can be enabled.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				})))
			}

			namespace, _ := cmd.Flags().GetString("namespace")
			kubeconfig, _ := cmd.Flags().GetString("kubeconfig")

			// Build server context.
			sc := &server.ServerContext{
				Namespace: namespace,
				OutputDir: outputDir,
				SuitesDir: suitesDir,
			}

			// Create KServe manager if in-cluster or kubeconfig is available.
			ksManager, err := kserve.NewManager(namespace, kubeconfig, inCluster)
			if err != nil {
				slog.Warn("KServe manager not available", "error", err)
			} else {
				sc.KServeManager = ksManager
			}

			// Create default LLM client (for scoring; test runs may use different endpoints).
			sc.LLMClient = llm.NewOpenAIClient()

			// Create MCP server.
			mcpSrv := mcpserver.NewMCPServer("llm-testing", rootCmd.Version,
				mcpserver.WithToolCapabilities(true),
			)

			if err := mcptools.RegisterTools(mcpSrv, sc); err != nil {
				return fmt.Errorf("failed to register MCP tools: %w", err)
			}

			// Set up graceful shutdown.
			shutdownCtx, cancel := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer cancel()

			switch transport {
			case transportStdio:
				return runStdioServer(mcpSrv)
			case transportStreamableHTTP:
				fmt.Printf("Starting llm-testing MCP server with %s transport...\n", transport)
				if enableOAuth {
					return runOAuthHTTPServer(mcpSrv, httpAddr, httpEndpoint, shutdownCtx, oauthConfig{
						baseURL:         oauthBaseURL,
						provider:        oauthProvider,
						dexIssuerURL:    dexIssuerURL,
						dexClientID:     dexClientID,
						dexClientSecret: dexClientSecret,
					})
				}
				return runHTTPServer(mcpSrv, httpAddr, httpEndpoint, shutdownCtx)
			default:
				return fmt.Errorf("unsupported transport: %s (supported: stdio, streamable-http)", transport)
			}
		},
	}

	cmd.Flags().StringVar(&transport, "transport", transportStdio, "Transport type: stdio or streamable-http")
	cmd.Flags().StringVar(&httpAddr, "http-addr", ":8080", "HTTP server address (for streamable-http)")
	cmd.Flags().StringVar(&httpEndpoint, "http-endpoint", "/mcp", "HTTP endpoint path (for streamable-http)")
	cmd.Flags().BoolVar(&inCluster, "in-cluster", false, "Use in-cluster Kubernetes authentication")
	cmd.Flags().StringVar(&outputDir, "output-dir", "results", "Directory for test results")
	cmd.Flags().StringVar(&suitesDir, "suites-dir", "", "External test suites directory (optional)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	// OAuth flags.
	cmd.Flags().BoolVar(&enableOAuth, "enable-oauth", false, "Enable OAuth 2.1 authentication (for HTTP transport)")
	cmd.Flags().StringVar(&oauthBaseURL, "oauth-base-url", "", "OAuth base URL (e.g. https://llm-testing.example.com)")
	cmd.Flags().StringVar(&oauthProvider, "oauth-provider", "dex", "OAuth provider: dex")
	cmd.Flags().StringVar(&dexIssuerURL, "dex-issuer-url", "", "Dex OIDC issuer URL")
	cmd.Flags().StringVar(&dexClientID, "dex-client-id", "", "Dex OAuth client ID")
	cmd.Flags().StringVar(&dexClientSecret, "dex-client-secret", "", "Dex OAuth client secret")

	return cmd
}

func runStdioServer(mcpSrv *mcpserver.MCPServer) error {
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		if err := mcpserver.ServeStdio(mcpSrv); err != nil {
			serverDone <- err
		}
	}()

	err := <-serverDone
	if err != nil {
		return fmt.Errorf("server stopped with error: %w", err)
	}
	return nil
}

func runHTTPServer(mcpSrv *mcpserver.MCPServer, addr, endpoint string, ctx context.Context) error {
	mcpHandler := mcpserver.NewStreamableHTTPServer(mcpSrv,
		mcpserver.WithEndpointPath(endpoint),
	)

	mux := http.NewServeMux()
	mux.Handle(endpoint, mcpHandler)

	// Health check.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	fmt.Printf("  HTTP endpoint: %s\n", endpoint)
	fmt.Printf("  Health: /healthz\n")

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverDone <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Shutdown signal received, stopping HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("error shutting down: %w", err)
		}
	case err := <-serverDone:
		if err != nil {
			return fmt.Errorf("HTTP server error: %w", err)
		}
	}

	fmt.Println("HTTP server stopped")
	return nil
}

type oauthConfig struct {
	baseURL         string
	provider        string
	dexIssuerURL    string
	dexClientID     string
	dexClientSecret string
}

func runOAuthHTTPServer(mcpSrv *mcpserver.MCPServer, addr, endpoint string, ctx context.Context, cfg oauthConfig) error {
	// Load credentials from env vars if not set via flags.
	if cfg.dexIssuerURL == "" {
		cfg.dexIssuerURL = os.Getenv("DEX_ISSUER_URL")
	}
	if cfg.dexClientID == "" {
		cfg.dexClientID = os.Getenv("DEX_CLIENT_ID")
	}
	if cfg.dexClientSecret == "" {
		cfg.dexClientSecret = os.Getenv("DEX_CLIENT_SECRET")
	}

	if cfg.baseURL == "" {
		return fmt.Errorf("--oauth-base-url is required when --enable-oauth is set")
	}
	if cfg.dexIssuerURL == "" {
		return fmt.Errorf("dex issuer URL is required (--dex-issuer-url or DEX_ISSUER_URL)")
	}
	if cfg.dexClientID == "" {
		return fmt.Errorf("dex client ID is required (--dex-client-id or DEX_CLIENT_ID)")
	}
	if cfg.dexClientSecret == "" {
		return fmt.Errorf("dex client secret is required (--dex-client-secret or DEX_CLIENT_SECRET)")
	}

	oauthSrv, err := server.NewOAuthHTTPServer(mcpSrv, endpoint, server.OAuthConfig{
		BaseURL:         cfg.baseURL,
		Provider:        cfg.provider,
		DexIssuerURL:    cfg.dexIssuerURL,
		DexClientID:     cfg.dexClientID,
		DexClientSecret: cfg.dexClientSecret,
	})
	if err != nil {
		return fmt.Errorf("failed to create OAuth HTTP server: %w", err)
	}

	fmt.Printf("OAuth-enabled HTTP server starting on %s\n", addr)
	fmt.Printf("  Base URL: %s\n", cfg.baseURL)
	fmt.Printf("  Provider: %s\n", cfg.provider)
	fmt.Printf("  MCP endpoint: %s (requires OAuth Bearer token)\n", endpoint)
	fmt.Printf("  Health: /healthz\n")
	fmt.Printf("  OAuth endpoints:\n")
	fmt.Printf("    - Authorization Server Metadata: /.well-known/oauth-authorization-server\n")
	fmt.Printf("    - Protected Resource Metadata: /.well-known/oauth-protected-resource\n")
	fmt.Printf("    - Client Registration: /oauth/register\n")
	fmt.Printf("    - Authorization: /oauth/authorize\n")
	fmt.Printf("    - Token: /oauth/token\n")
	fmt.Printf("    - Callback: /oauth/callback\n")

	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		if err := oauthSrv.Start(addr); err != nil && err != http.ErrServerClosed {
			serverDone <- err
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Shutdown signal received, stopping OAuth HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := oauthSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("error shutting down OAuth HTTP server: %w", err)
		}
	case err := <-serverDone:
		if err != nil {
			return fmt.Errorf("OAuth HTTP server error: %w", err)
		}
	}

	fmt.Println("OAuth HTTP server stopped")
	return nil
}
