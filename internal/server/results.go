package server

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/preaverage/reddit-mcp/internal/reddit"
)

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func jsonResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return textResult(fmt.Sprint(v))
	}

	return textResult(string(b))
}

func result[T any](r reddit.Result[T], err error, raw bool) (*mcp.CallToolResult, error) {
	if err != nil {
		return nil, err
	}

	if raw {
		return textResult(string(r.Raw)), nil
	}

	return jsonResult(r.Data), nil
}
