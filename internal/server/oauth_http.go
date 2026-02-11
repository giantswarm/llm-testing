package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	oauth "github.com/giantswarm/mcp-oauth"
	"github.com/giantswarm/mcp-oauth/providers/dex"
	oauthserver "github.com/giantswarm/mcp-oauth/server"
	"github.com/giantswarm/mcp-oauth/storage/memory"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const (
	// OAuthProviderDex is the Dex OIDC provider.
	OAuthProviderDex = "dex"

	defaultReadHeaderTimeout = 10 * time.Second
	defaultWriteTimeout      = 120 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultShutdownTimeout   = 10 * time.Second
)

// OAuthConfig holds configuration for the OAuth-enabled HTTP server.
type OAuthConfig struct {
	// BaseURL is the server's public base URL (e.g. https://llm-testing.example.com).
	BaseURL string

	// Provider is the OAuth provider name.
	// Currently only "dex" is supported. This field is reserved for future
	// provider implementations (e.g. "okta", "auth0") and is recorded in
	// server metadata for operational visibility.
	Provider string

	// DexIssuerURL is the Dex OIDC issuer URL.
	DexIssuerURL string

	// DexClientID is the Dex OAuth client ID.
	DexClientID string

	// DexClientSecret is the Dex OAuth client secret.
	DexClientSecret string
}

// OAuthHTTPServer wraps an MCP server with OAuth 2.1 authentication.
type OAuthHTTPServer struct {
	mcpServer    *mcpserver.MCPServer
	oauthServer  *oauth.Server
	oauthHandler *oauth.Handler
	httpServer   *http.Server
	mcpEndpoint  string
}

// NewOAuthHTTPServer creates a new OAuth-enabled HTTP server for MCP.
func NewOAuthHTTPServer(mcpSrv *mcpserver.MCPServer, mcpEndpoint string, cfg OAuthConfig) (*OAuthHTTPServer, error) {
	if cfg.Provider != "" && cfg.Provider != OAuthProviderDex {
		return nil, fmt.Errorf("unsupported OAuth provider %q (supported: %s)", cfg.Provider, OAuthProviderDex)
	}

	if err := validateHTTPSRequirement(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("OAuth base URL validation failed: %w", err)
	}

	// Create Dex provider.
	callbackURL := cfg.BaseURL + "/oauth/callback"
	dexProvider, err := dex.NewProvider(&dex.Config{
		IssuerURL:    cfg.DexIssuerURL,
		ClientID:     cfg.DexClientID,
		ClientSecret: cfg.DexClientSecret,
		RedirectURL:  callbackURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Dex provider: %w", err)
	}

	// Create in-memory storage (sufficient for single-instance deployment).
	store := memory.New()

	logger := slog.Default()

	// Create OAuth server.
	oauthSrv, err := oauth.NewServer(
		dexProvider,
		store,
		store,
		store,
		&oauthserver.Config{
			Issuer:                    cfg.BaseURL,
			AllowRefreshTokenRotation: true,
			MaxClientsPerIP:           10,
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth server: %w", err)
	}

	oauthHandler := oauth.NewHandler(oauthSrv, logger)

	return &OAuthHTTPServer{
		mcpServer:    mcpSrv,
		oauthServer:  oauthSrv,
		oauthHandler: oauthHandler,
		mcpEndpoint:  mcpEndpoint,
	}, nil
}

// Start starts the OAuth-enabled HTTP server.
func (s *OAuthHTTPServer) Start(addr string) error {
	mux := http.NewServeMux()

	// Register OAuth endpoints.
	s.oauthHandler.RegisterAuthorizationServerMetadataRoutes(mux)
	s.oauthHandler.RegisterProtectedResourceMetadataRoutes(mux, s.mcpEndpoint)
	mux.HandleFunc("/oauth/authorize", s.oauthHandler.ServeAuthorization)
	mux.HandleFunc("/oauth/token", s.oauthHandler.ServeToken)
	mux.HandleFunc("/oauth/callback", s.oauthHandler.ServeCallback)
	mux.HandleFunc("/oauth/register", s.oauthHandler.ServeClientRegistration)
	mux.HandleFunc("/oauth/revoke", s.oauthHandler.ServeTokenRevocation)
	mux.HandleFunc("/oauth/introspect", s.oauthHandler.ServeTokenIntrospection)

	// Register MCP endpoint behind OAuth token validation.
	mcpHandler := mcpserver.NewStreamableHTTPServer(s.mcpServer,
		mcpserver.WithEndpointPath(s.mcpEndpoint),
	)
	mux.Handle(s.mcpEndpoint, s.oauthHandler.ValidateToken(mcpHandler))

	// Health check (unauthenticated).
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *OAuthHTTPServer) Shutdown(ctx context.Context) error {
	if s.oauthServer != nil {
		if err := s.oauthServer.Shutdown(ctx); err != nil {
			slog.Error("failed to shutdown OAuth server", "error", err)
		}
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// validateHTTPSRequirement ensures OAuth 2.1 HTTPS compliance.
// Allows HTTP only for loopback addresses (localhost, 127.0.0.1, ::1).
func validateHTTPSRequirement(baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("base URL cannot be empty")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	if u.Scheme == "http" {
		host := u.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return fmt.Errorf("OAuth 2.1 requires HTTPS for production (got: %s). Use HTTPS or localhost for development", baseURL)
		}
	} else if u.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s (must be http for localhost or https)", u.Scheme)
	}

	return nil
}
