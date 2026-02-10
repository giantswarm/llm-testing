package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/llm-testing/internal/server"
)

// RegisterTools registers all MCP tools with the server.
func RegisterTools(s *mcpserver.MCPServer, sc *server.ServerContext) error {
	if err := registerTestSuiteTools(s, sc); err != nil {
		return err
	}
	if err := registerModelTools(s, sc); err != nil {
		return err
	}
	return nil
}
