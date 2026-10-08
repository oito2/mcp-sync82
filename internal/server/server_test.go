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

package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/oito2/mcp-sync82/internal/server"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// fakeTool is a minimal tools.Tool test double. It lets these tests exercise
// the server dispatch logic (both error paths, schema and description wiring,
// panic recovery) in isolation from any real tool, against the real SDK.
type fakeTool struct {
	name        string
	description string
	schema      map[string]any

	validateErr    error // non-nil makes Validate fail
	execResult     tools.ToolResult
	execErr        error // non-nil makes Execute fail
	panicOnExecute any   // non-nil makes Execute panic with this value

	validateCalledWith json.RawMessage
	executeCalledWith  any
}

// Name, Description and InputSchema return the configured tool name,
// description and input schema.
func (f *fakeTool) Name() string                { return f.name }
func (f *fakeTool) Description() string         { return f.description }
func (f *fakeTool) InputSchema() map[string]any { return f.schema }

// Validate records raw, returns validateErr when set, and otherwise returns
// raw decoded as a map.
func (f *fakeTool) Validate(raw json.RawMessage) (any, error) {
	f.validateCalledWith = raw
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	return parsed, nil
}

// Execute records args, then panics with panicOnExecute when set, fails with
// execErr when set, and otherwise returns execResult.
func (f *fakeTool) Execute(_ context.Context, args any) (tools.ToolResult, error) {
	f.executeCalledWith = args
	if f.panicOnExecute != nil {
		panic(f.panicOnExecute)
	}
	if f.execErr != nil {
		return tools.ToolResult{}, f.execErr
	}
	return f.execResult, nil
}

// connect starts a real sync82 server with registeredTools and the
// tools.Prompts prompts, connected to a real SDK client; see connectWith.
func connect(t *testing.T, registeredTools []tools.Tool) *mcp.ClientSession {
	t.Helper()
	return connectWith(t, server.Options{Tools: registeredTools, Prompts: tools.Prompts})
}

// connectWith starts a real sync82 server built from opts and a real SDK
// client over an in-memory transport pair, and returns the client session.
// The tests therefore exercise the actual wire behavior. Sessions are
// closed on test cleanup; connection failures fail the test.
func connectWith(t *testing.T, opts server.Options) *mcp.ClientSession {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	s := server.New("sync82-test", "v0.0.0-test", logger, opts)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	return clientSession
}

// TestListTools_ReflectsRegisteredTools verifies that listed tools carry the registered name, description and
// schema.
func TestListTools_ReflectsRegisteredTools(t *testing.T) {
	tool := &fakeTool{
		name:        "example_tool",
		description: "an example tool for testing",
		schema:      map[string]any{"type": "object", "properties": map[string]any{}},
	}
	cs := connect(t, []tools.Tool{tool})

	result, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 registered tool, got %d", len(result.Tools))
	}
	got := result.Tools[0]
	if got.Name != tool.name {
		t.Errorf("Name = %q, want %q", got.Name, tool.name)
	}
	if got.Description != tool.description {
		t.Errorf("Description = %q, want %q", got.Description, tool.description)
	}
}

// TestListTools_EmptyRegistry verifies that a server without tools lists none.
func TestListTools_EmptyRegistry(t *testing.T) {
	cs := connect(t, nil)

	result, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(result.Tools))
	}
}

// TestCallTool_ValidationFailure_IsNormalResultNotProtocolError verifies that a Validate failure comes back as an error result and the
// connection stays usable.
func TestCallTool_ValidationFailure_IsNormalResultNotProtocolError(t *testing.T) {
	tool := &fakeTool{
		name:        "needs_project",
		schema:      map[string]any{"type": "object"},
		validateErr: errors.New(`missing required field "project"`),
	}
	cs := connect(t, []tools.Tool{tool})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tool.name,
		Arguments: map[string]any{},
	})
	// A validation failure must not surface as a Go/transport error — the
	// connection must survive it, and the calling agent must see the
	// message in the result content instead.
	if err != nil {
		t.Fatalf("CallTool returned a transport error for a validation failure: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for a validation failure")
	}
	text := textOf(t, result)
	if !strings.Contains(text, `missing required field "project"`) {
		t.Fatalf("result text = %q, want it to contain the validation message", text)
	}

	// The connection must still be usable afterwards.
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("connection did not survive a validation failure: %v", err)
	}
}

// callToolError calls the named tool and returns the text of its result. It
// fails the test unless the call succeeded at the protocol level with isError
// set.
func callToolError(t *testing.T, cs *mcp.ClientSession, name string) string {
	t.Helper()
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool returned a protocol error %v, want an isError result", err)
	}
	if !result.IsError {
		t.Fatalf("result.IsError = false, want true")
	}
	if len(result.Content) != 1 {
		t.Fatalf("result has %d content items, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("result content is %T, want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}

// TestCallTool_ExecutionFailure_IsToolErrorResult verifies that an error from Execute (such as `project not found`) comes
// back as a tool error result the agent can read, not as a JSON-RPC protocol
// error, which some clients never show to the model.
func TestCallTool_ExecutionFailure_IsToolErrorResult(t *testing.T) {
	tool := &fakeTool{
		name:    "flaky",
		schema:  map[string]any{"type": "object"},
		execErr: errors.New(`project not found: "foo". Use create_project first.`),
	}
	cs := connect(t, []tools.Tool{tool})

	if text := callToolError(t, cs, tool.name); !strings.Contains(text, `project not found: "foo"`) {
		t.Fatalf("result text = %q, want the execution error's message", text)
	}

	// The connection must still be usable afterwards — one tool's
	// execution failure must not take down the session.
	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("connection did not survive an execution failure: %v", err)
	}
}

// TestCallTool_ExecutePanic_IsToolErrorNotConnectionCrash verifies that a panic inside Execute produces a generic error result
// without leaking the panic value, and that the session survives.
func TestCallTool_ExecutePanic_IsToolErrorNotConnectionCrash(t *testing.T) {
	tool := &fakeTool{
		name:           "panics",
		schema:         map[string]any{"type": "object"},
		panicOnExecute: "simulated nil map write",
	}
	cs := connect(t, []tools.Tool{tool})

	// The raw panic value must not leak to the calling agent — only a
	// generic message.
	if text := callToolError(t, cs, tool.name); strings.Contains(text, "simulated nil map write") {
		t.Errorf("result text = %q, want the raw panic value not to leak to the client", text)
	}

	if _, err := cs.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("connection did not survive a tool panic: %v", err)
	}
}

// TestCallTool_ExecutionFailure_StoreOpenFailure_IsMasked verifies that a store.ErrOpenFailed error is replaced by a generic
// message that hides vault file details.
func TestCallTool_ExecutionFailure_StoreOpenFailure_IsMasked(t *testing.T) {
	// A store.Open failure carries low-level detail about the vault file;
	// the agent gets a generic message instead.
	underlying := fmt.Errorf("%w: open vault %s: %w", store.ErrOpenFailed, "/home/carlos/.sync82/knowledge.db", errors.New("permission denied"))
	tool := &fakeTool{
		name:    "flaky-store",
		schema:  map[string]any{"type": "object"},
		execErr: underlying,
	}
	cs := connect(t, []tools.Tool{tool})

	text := callToolError(t, cs, tool.name)
	if strings.Contains(text, "/home/carlos/.sync82/knowledge.db") || strings.Contains(text, "permission denied") {
		t.Fatalf("result text = %q, must not contain the vault open failure's detail", text)
	}
}

// TestCallTool_ExecutionFailure_NewerSchema_SaysToUpgrade verifies that a
// vault refused for its newer schema is reported with a message telling
// the agent to upgrade sync82, without the vault file's path.
func TestCallTool_ExecutionFailure_NewerSchema_SaysToUpgrade(t *testing.T) {
	underlying := fmt.Errorf("%w: migrate vault %s: vault schema version 4 is newer than this sync82 supports (3): %w",
		store.ErrOpenFailed, "/home/carlos/.sync82/knowledge.db", store.ErrSchemaTooNew)
	tool := &fakeTool{
		name:    "old-binary",
		schema:  map[string]any{"type": "object"},
		execErr: underlying,
	}
	cs := connect(t, []tools.Tool{tool})

	text := callToolError(t, cs, tool.name)
	if !strings.Contains(text, "upgrade sync82") || strings.Contains(text, "/home/carlos/.sync82/knowledge.db") {
		t.Fatalf("result text = %q, want the upgrade hint without the vault path", text)
	}
}

// TestCallTool_Success verifies that a successful call returns the tool text and structured
// content.
func TestCallTool_Success(t *testing.T) {
	tool := &fakeTool{
		name:       "echo",
		schema:     map[string]any{"type": "object"},
		execResult: tools.ToolResult{Text: "hello from sync82"},
	}
	cs := connect(t, []tools.Tool{tool})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tool.name,
		Arguments: map[string]any{"anything": "goes"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false, got true (text: %s)", textOf(t, result))
	}
	if got := textOf(t, result); got != "hello from sync82" {
		t.Fatalf("result text = %q, want %q", got, "hello from sync82")
	}

	parsed, ok := tool.executeCalledWith.(map[string]any)
	if !ok {
		t.Fatalf("expected Execute to receive the value Validate returned, got %T", tool.executeCalledWith)
	}
	if parsed["anything"] != "goes" {
		t.Fatalf("Execute did not receive the validated arguments: %+v", parsed)
	}
}

// TestCallTool_BusinessIsError_IsNormalResultNotProtocolError verifies that ToolResult.IsError is passed through as an error result
// without a protocol error.
func TestCallTool_BusinessIsError_IsNormalResultNotProtocolError(t *testing.T) {
	// check_project_health-style outcome: nothing crashed, but the tool
	// itself reports a business-level failure via ToolResult.IsError.
	tool := &fakeTool{
		name:       "check_health",
		schema:     map[string]any{"type": "object"},
		execResult: tools.ToolResult{Text: "UNHEALTHY: missing files", IsError: true},
	}
	cs := connect(t, []tools.Tool{tool})

	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: tool.name,
	})
	if err != nil {
		t.Fatalf("CallTool returned a transport error for a business-level IsError result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true to be preserved from the tool's ToolResult")
	}
}

// textOf returns the text of the first content item of result. It fails the
// test when the result has no text content.
func textOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("expected exactly 1 content block, got %d", len(result.Content))
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// TestServerInfo_AdvertisesIcon verifies that the server info advertises the embedded icon.
func TestServerInfo_AdvertisesIcon(t *testing.T) {
	cs := connect(t, nil)
	info := cs.InitializeResult().ServerInfo
	if len(info.Icons) != 1 || !strings.HasPrefix(info.Icons[0].Source, "data:image/png;base64,") || info.Icons[0].Sizes[0] != "64x64" {
		t.Fatalf("serverInfo icons = %+v, want the embedded 64x64 PNG", info.Icons)
	}
}

// TestNew_ServesTheGivenPrompts verifies that the server lists and renders
// exactly the prompts passed in Options, and offers no resources or
// completion without Options.Resources.
func TestNew_ServesTheGivenPrompts(t *testing.T) {
	cs := connectWith(t, server.Options{Prompts: []tools.PromptDefinition{{
		Name:      "fake_prompt",
		Arguments: []tools.PromptArgument{{Name: "who", Required: true}},
		Render:    func(args map[string]string) (string, error) { return "hello " + args["who"], nil },
	}}})
	ctx := context.Background()

	list, err := cs.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if len(list.Prompts) != 1 || list.Prompts[0].Name != "fake_prompt" {
		t.Fatalf("prompts = %+v, want only fake_prompt", list.Prompts)
	}
	got, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "fake_prompt", Arguments: map[string]string{"who": "world"}})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if text := got.Messages[0].Content.(*mcp.TextContent).Text; text != "hello world" {
		t.Errorf("rendered prompt = %q, want %q", text, "hello world")
	}
	if caps := cs.InitializeResult().Capabilities; caps.Resources != nil || caps.Completions != nil {
		t.Errorf("capabilities = %+v, want no resources or completions without Options.Resources", caps)
	}
}

// fakeOutputTool is a fakeTool that declares an output schema.
type fakeOutputTool struct {
	*fakeTool
}

// OutputSchema returns a fixed object schema.
func (f fakeOutputTool) OutputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"n": map[string]any{"type": "integer"}}, "required": []string{"n"}}
}

// TestOutputSchema_AdvertisedAndEnforced verifies that tools/list carries
// a tool's output schema, that its structured content is returned, and
// that a successful result without structured content is turned into an
// error result, since clients reject it.
func TestOutputSchema_AdvertisedAndEnforced(t *testing.T) {
	tool := fakeOutputTool{&fakeTool{
		name:       "out_tool",
		schema:     map[string]any{"type": "object"},
		execResult: tools.ToolResult{Text: "n is 3", Structured: map[string]any{"n": 3}},
	}}
	cs := connect(t, []tools.Tool{tool})
	ctx := context.Background()

	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if list.Tools[0].OutputSchema == nil {
		t.Fatal("tools/list has no outputSchema for a tool that declares one")
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "out_tool"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || res.StructuredContent == nil {
		t.Errorf("result = %+v, want a successful result with structured content", res)
	}

	tool.execResult = tools.ToolResult{Text: "please name a project"}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "out_tool"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Error("a result without structured content from a tool with an output schema should be an error result")
	}
}
