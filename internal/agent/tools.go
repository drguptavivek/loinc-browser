package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// allowedTools is the whitelist of read-only MCP tools the agent may call. Keeping this narrow
// (rather than exposing every registered MCP tool) bounds what a prompt-injected tool result or a
// misbehaving model can reach: no write, ingest, or credential tool is ever on this list.
var allowedTools = map[string]bool{
	"loinc_search_terms":           true,
	"loinc_get_term":               true,
	"loinc_get_term_fit":           true,
	"loinc_get_term_relationships": true,
	"loinc_search_panels":          true,
	"loinc_get_panel_items":        true,
	"loinc_match_names":            true,
	"loinc_explain_concepts":       true,
}

// maxToolResultChars caps one tool result before it goes into the conversation, so a large
// candidate list can't blow the model's context on its own.
const maxToolResultChars = 6000

// ToolBridge connects the agent loop to the existing MCP tool implementations in-process (an
// in-memory client/server pair), rather than duplicating any tool logic here.
type ToolBridge struct {
	client *mcp.ClientSession
	server *mcp.ServerSession
	tools  []ToolDef
}

// NewToolBridge connects to mcpServer in-process and lists the whitelisted tools as OpenAI tool
// definitions.
func NewToolBridge(ctx context.Context, mcpServer *mcp.Server) (*ToolBridge, error) {
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect agent tool server: %w", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "loinc-agent", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		return nil, fmt.Errorf("connect agent tool client: %w", err)
	}
	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		_ = clientSession.Close()
		_ = serverSession.Close()
		return nil, fmt.Errorf("list agent tools: %w", err)
	}
	tools := make([]ToolDef, 0, len(allowedTools))
	for _, t := range result.Tools {
		if !allowedTools[t.Name] {
			continue
		}
		tools = append(tools, ToolDef{Name: t.Name, Description: t.Description, Parameters: t.InputSchema})
	}
	return &ToolBridge{client: clientSession, server: serverSession, tools: tools}, nil
}

// Tools returns the whitelisted tool definitions to offer the model.
func (b *ToolBridge) Tools() []ToolDef { return b.tools }

// CallTool runs one whitelisted tool call and returns its text result, truncated to
// maxToolResultChars. argsJSON is the model's raw (untrusted) arguments string.
func (b *ToolBridge) CallTool(ctx context.Context, name string, argsJSON string) (string, error) {
	if !allowedTools[name] {
		return "", fmt.Errorf("tool %q is not available to the agent", name)
	}
	var args map[string]any
	if s := strings.TrimSpace(argsJSON); s != "" {
		if err := json.Unmarshal([]byte(s), &args); err != nil {
			return "", fmt.Errorf("invalid arguments for %s: %w", name, err)
		}
	}
	result, err := b.client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			out.WriteString(text.Text)
		}
	}
	if out.Len() == 0 && result.StructuredContent != nil {
		if raw, err := json.Marshal(result.StructuredContent); err == nil {
			out.Write(raw)
		}
	}
	text := out.String()
	if len(text) > maxToolResultChars {
		text = text[:maxToolResultChars] + "…(truncated)"
	}
	return text, nil
}

// Close disconnects both ends of the in-process MCP session.
func (b *ToolBridge) Close() error {
	err := b.client.Close()
	if serverErr := b.server.Close(); err == nil {
		err = serverErr
	}
	return err
}
