// Copyright (C) 2026  OITO2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// New builds the sync82 MCP server and registers every tool in
// registeredTools. name/version identify the server to connecting
// clients; logger receives every diagnostic message and must write only
// to stderr — stdout is the JSON-RPC channel over the stdio transport.
func New(name, version string, logger *slog.Logger, registeredTools []tools.Tool) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: name, Version: version, Icons: serverIcons()}, &mcp.ServerOptions{
		Logger: logger,
	})

	for _, t := range registeredTools {
		schema := t.InputSchema()
		// Arguments a tool doesn't define are rejected by its Validate;
		// the schema says so too, so clients don't send them.
		if _, set := schema["additionalProperties"]; !set {
			schema["additionalProperties"] = false
		}
		s.AddTool(&mcp.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: schema,
		}, adapt(t, logger))
	}

	return s
}

// iconPNG is the 64×64 server icon advertised in serverInfo. It is
// embedded as a data URI because a stdio server has no URL to serve it
// from.
//
//go:embed icon.png
var iconPNG []byte

// serverIcons returns the icons advertised in serverInfo.
func serverIcons() []mcp.Icon {
	return []mcp.Icon{{
		Source:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconPNG),
		MIMEType: "image/png",
		Sizes:    []string{"64x64"},
	}}
}

// adapt wraps a tools.Tool into the SDK's low-level mcp.ToolHandler.
// This deliberately bypasses the SDK's generic AddTool[In, Out] (which
// infers a JSON schema from Go struct tags and auto-validates against
// it) — the schema and validation here are hand-written per tool, so
// every tool's Validate step controls its own human-readable,
// multi-field error messages instead of a generic reflection-based one.
//
// Every failure — invalid arguments, an error from Execute, even a panic
// — comes back as a tool result with isError set, never as a JSON-RPC
// protocol error: protocol errors are for malformed requests and unknown
// tools, and some clients don't show them to the model at all, while a
// tool error result lets the calling agent read the message and correct
// its call.
func adapt(t tools.Tool, logger *slog.Logger) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		// The MCP SDK runs each request in its own goroutine — an
		// unrecovered panic here (a bad type assertion, a nil map write on
		// an edge case no test happens to cover) would otherwise take that
		// goroutine down without ever sending a response, leaving the
		// calling client hanging instead of seeing a clean error.
		defer func() {
			if r := recover(); r != nil {
				logger.Error("tool panicked", "tool", t.Name(), "panic", r)
				result, err = errorResult("internal error: tool execution failed unexpectedly"), nil
			}
		}()

		args, verr := t.Validate(req.Params.Arguments)
		if verr != nil {
			return errorResult(verr.Error()), nil
		}

		toolResult, eerr := t.Execute(ctx, args)
		if eerr != nil {
			return errorResult(clientMessage(logger, t.Name(), eerr)), nil
		}

		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: toolResult.Text}},
			StructuredContent: toolResult.Structured,
			IsError:           toolResult.IsError,
		}, nil
	}
}

// errorResult builds a tool result flagged as an error, with msg as its
// only text content.
func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}
}

// clientMessage logs err in full server-side and returns the message the
// calling agent sees. Store errors are already worded for the agent
// (e.g. `project not found: "foo"`), so err.Error() is used as-is, with
// one exception: a store.ErrOpenFailed error carries low-level detail
// about opening the vault file (paths, SQLite errors) and is replaced by
// a generic message — the full error is still in the log.
func clientMessage(logger *slog.Logger, toolName string, err error) string {
	logger.Error("tool execution failed", "tool", toolName, "error", err)
	if errors.Is(err, store.ErrOpenFailed) {
		return "internal error: could not open the vault database"
	}
	return err.Error()
}
