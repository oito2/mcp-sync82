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

package tools

import (
	"context"
	"encoding/json"
)

// ToolResult is a tool's business-level outcome — the payload of a normal
// JSON-RPC response. Text is the message shown to the calling agent.
// IsError marks a failure the agent should see and act on (e.g.
// check_project_health reporting an unhealthy project).
type ToolResult struct {
	Text    string
	IsError bool
	// Structured, when set, is also returned as the result's structured
	// content (a value that marshals to a JSON object).
	Structured any
}

// Tool is the interface every one of sync82's tools implements, so the
// server can register and dispatch all of them the same way.
//
// Validate checks and decodes the arguments; Execute performs the call.
// The server (internal/server) turns an error from either one into a
// tool result with IsError: true, so the calling agent always sees the
// message — an Execute error that wraps store.ErrOpenFailed is replaced
// by a generic message there, since it carries low-level detail about the
// vault file.
//
// Validate's returned args value is opaque to the server; only the same
// Tool's own Execute knows its concrete type. This type erasure is what
// lets internal/server hold a single []Tool of heterogeneous
// implementations — each tool is trusted to keep Validate and Execute in
// sync with each other, an invariant enforced by construction (both
// methods live on the same concrete type) rather than by the type system.
type Tool interface {
	// Name returns the tool's unique MCP name.
	Name() string
	// Description returns the text that tells the calling agent what the
	// tool does.
	Description() string

	// InputSchema must never be nil and must marshal to a JSON object
	// with "type": "object" — the SDK's Server.AddTool panics at
	// registration time otherwise (a deliberate fail-fast check so a
	// missing schema is caught at startup, not the first time an agent
	// sends bad input). A tool with no arguments still needs
	// map[string]any{"type": "object"}.
	InputSchema() map[string]any

	// Validate parses raw JSON-RPC arguments and checks them against the
	// tool's business rules (required fields, string/int bounds, enum
	// values, and so on). A non-nil error here is always a validation
	// failure, never a system error, and its message must list every
	// failing field so the calling agent can correct all of them in one
	// retry rather than discovering them one at a time.
	Validate(raw json.RawMessage) (args any, err error)

	// Execute runs the tool's business logic against the value Validate
	// returned. A non-nil error here is always an execution failure.
	Execute(ctx context.Context, args any) (ToolResult, error)
}
