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
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// Options lists what New registers on the server. Tools are registered
// with their toolAnnotations and Prompts as MCP prompts. When Resources is
// not nil, ResourceTemplates are served through it, and the arguments of
// Prompts and ResourceTemplates are completed from it; without it, neither
// resources nor completion are offered.
type Options struct {
	Tools             []tools.Tool
	Prompts           []tools.PromptDefinition
	ResourceTemplates []tools.ResourceTemplate
	Resources         *tools.Resources
}

// New builds the sync82 MCP server with everything opts lists.
// name/version identify the server to connecting clients; logger receives
// every diagnostic message and must write only to stderr — stdout is the
// JSON-RPC channel over the stdio transport. A tool call that creates,
// renames or deletes a project tells clients the resource list changed,
// when resources are served.
func New(name, version string, logger *slog.Logger, opts Options) *mcp.Server {
	serverOpts := &mcp.ServerOptions{Logger: slog.New(cancelAsInfo{logger.Handler()})}
	if opts.Resources != nil {
		serverOpts.CompletionHandler = completionHandler(opts, logger)
	}
	s := mcp.NewServer(&mcp.Implementation{Name: name, Version: version, Icons: serverIcons()}, serverOpts)

	onProjectsChanged := func() {}
	if opts.Resources != nil && len(opts.ResourceTemplates) > 0 {
		onProjectsChanged = addResources(s, opts.ResourceTemplates, opts.Resources, logger)
	}
	addPrompts(s, opts.Prompts, logger)
	for _, t := range opts.Tools {
		schema := t.InputSchema()
		// Arguments a tool doesn't define are rejected by its Validate;
		// the schema says so too, so clients don't send them.
		if _, set := schema["additionalProperties"]; !set {
			schema["additionalProperties"] = false
		}
		tool := &mcp.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: schema,
			Annotations: toolAnnotations(t.Name()),
		}
		if o, ok := t.(tools.OutputSchemaTool); ok {
			tool.OutputSchema = o.OutputSchema()
		}
		s.AddTool(tool, adapt(t, logger, onProjectsChanged))
	}

	return s
}

// cancelAsInfo is the handler of the logger the SDK receives. It logs at
// INFO an ERROR record whose "error" attribute is context.Canceled — the
// SDK reports a server stopped by SIGINT/SIGTERM that way, and that is a
// clean shutdown — and passes every other record to the wrapped handler
// unchanged.
type cancelAsInfo struct {
	slog.Handler
}

// Handle passes r to the wrapped handler, at INFO when it is an ERROR
// record caused by context.Canceled.
func (h cancelAsInfo) Handle(ctx context.Context, r slog.Record) error {
	if r.Level == slog.LevelError {
		r.Attrs(func(a slog.Attr) bool {
			if err, ok := a.Value.Any().(error); ok && a.Key == "error" && errors.Is(err, context.Canceled) {
				r.Level = slog.LevelInfo
				return false
			}
			return true
		})
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns the handler with attrs added, still logging
// cancellations at INFO.
func (h cancelAsInfo) WithAttrs(attrs []slog.Attr) slog.Handler {
	return cancelAsInfo{h.Handler.WithAttrs(attrs)}
}

// WithGroup returns the handler with the group opened, still logging
// cancellations at INFO.
func (h cancelAsInfo) WithGroup(name string) slog.Handler {
	return cancelAsInfo{h.Handler.WithGroup(name)}
}

// toolAnnotations returns the MCP annotations of the tool called name, from
// tools.HintsFor, or nil when it has no hints. Every annotation is set
// explicitly, since the protocol defaults destructiveHint and openWorldHint
// to true.
func toolAnnotations(name string) *mcp.ToolAnnotations {
	h, ok := tools.HintsFor(name)
	if !ok {
		return nil
	}
	destructive, openWorld := h.Destructive && !h.ReadOnly, false
	return &mcp.ToolAnnotations{
		Title:           h.Title,
		ReadOnlyHint:    h.ReadOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  h.Idempotent || h.ReadOnly,
		OpenWorldHint:   &openWorld,
	}
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
//
// A tool with an output schema must return structured content with every
// successful result, and clients reject one without it. Its results that
// carry none, such as the request to name a project, are therefore marked
// isError: the agent still reads the text and acts on it.
func adapt(t tools.Tool, logger *slog.Logger, onProjectsChanged func()) mcp.ToolHandler {
	hints, _ := tools.HintsFor(t.Name())
	_, hasOutputSchema := t.(tools.OutputSchemaTool)
	return func(ctx context.Context, req *mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		// The MCP SDK runs each request in its own goroutine and recovers
		// from no panic: an unrecovered panic here (a bad type assertion, a
		// nil map write on an edge case no test happens to cover) would
		// crash the whole process, ending every client's session.
		defer func() {
			if r := recover(); r != nil {
				logger.Error("tool panicked", "tool", t.Name(), "panic", r, "stack", string(debug.Stack()))
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
		if hints.ChangesProjects && !toolResult.IsError && !toolResult.ProjectsUnchanged {
			onProjectsChanged()
		}
		if hasOutputSchema && toolResult.Structured == nil {
			toolResult.IsError = true
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

// clientMessage logs err in full server-side, naming the tool or MCP
// method that failed (handler), and returns the message the calling agent
// sees. Store errors are already worded for the agent (e.g. `project not
// found: "foo"`), so err.Error() is used as-is, with one exception: a
// store.ErrOpenFailed error carries low-level detail about opening the
// vault file (paths, SQLite errors) and is replaced by a generic message,
// or by store.ErrSchemaTooNew's message when the vault was upgraded by a
// newer sync82 — the full error is still in the log.
func clientMessage(logger *slog.Logger, handler string, err error) string {
	logger.Error("request failed", "handler", handler, "error", err)
	if errors.Is(err, store.ErrSchemaTooNew) {
		return "could not open the vault database: " + store.ErrSchemaTooNew.Error()
	}
	if errors.Is(err, store.ErrOpenFailed) {
		return "internal error: could not open the vault database"
	}
	return err.Error()
}

// internalError returns the JSON-RPC internal error an MCP method other
// than a tool call answers with when err occurs, worded by clientMessage.
func internalError(logger *slog.Logger, method string, err error) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: clientMessage(logger, method, err)}
}

// recoverAsInternal, deferred by an MCP method handler with named error
// result *err, turns a panic into a logged internal error answer instead
// of a crash of the whole process (the SDK recovers from no panic).
func recoverAsInternal(logger *slog.Logger, method string, err *error) {
	if r := recover(); r != nil {
		logger.Error("handler panicked", "handler", method, "panic", r, "stack", string(debug.Stack()))
		*err = &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error: the request failed unexpectedly"}
	}
}
