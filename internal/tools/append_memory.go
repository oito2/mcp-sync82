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
	"errors"
	"fmt"

	"github.com/oito2/mcp-sync82/internal/store"
)

// AppendMemoryTool implements append_memory: add a new entry to an
// append-only kind. A "## YYYY-MM-DD" date header is required only for the
// two standard append-only kinds (progress, decisions); a custom kind
// never requires one. Validation and execution delegate to
// validateAppendInput and appendMemoryCore, shared with
// update_project_memory.
type AppendMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// appendMemoryArgs holds the decoded arguments of the append_memory tool;
// its JSON tags match the property names declared in InputSchema.
type appendMemoryArgs struct {
	targetArgs
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// Name returns the MCP tool name, "append_memory".
func (t *AppendMemoryTool) Name() string { return "append_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *AppendMemoryTool) Description() string {
	return `Append a new dated entry to an append-only memory file (progress, decisions, or a custom kind). IMPORTANT: progress and decisions require a "## YYYY-MM-DD" date header somewhere in content. Overwrite-style files (memory, architecture, stack, next_steps, or a custom kind created with write_memory) are rejected — use write_memory for them.`
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties filename and content plus the project-
// resolution properties.
func (t *AppendMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{}, map[string]any{
			"filename": map[string]any{"type": "string", "description": "The file/kind to append to (e.g. \"progress\", \"decisions\", or a custom name)."},
			"content":  map[string]any{"type": "string", "description": "The content to append. For \"progress\"/\"decisions\", must contain a \"## YYYY-MM-DD\" header."},
		}),
		"required": []string{"filename", "content"},
	}
}

// Validate decodes raw into appendMemoryArgs and checks filename and content
// with validateAppendInput. It returns the arguments with Filename
// normalized to its lower-case kind name, or an error describing the first
// violated rule.
func (t *AppendMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args appendMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	// Entry id marker lines are removed before the checks, so content
	// holding only markers is refused here as empty.
	args.Content = stripEntryMarkers(args.Content)
	kind, err := validateAppendInput(args.Filename, args.Content)
	if err != nil {
		return nil, err
	}
	args.Filename = kind
	return args, nil
}

// Execute resolves the target project and appends the content as a new entry
// of the kind through appendMemoryCore. Appending to an overwrite-style kind
// returns an error result (IsError) that points to write_memory; an
// unresolved project returns the instructional result; other failures are
// returned as errors.
func (t *AppendMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(appendMemoryArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, args.contextArgs())
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	if err := appendMemoryCore(ctx, s, rctx.Project, rctx.Subproject, args.Filename, args.Content); err != nil {
		if errors.Is(err, errAppendToDocumentKind) {
			return ToolResult{Text: err.Error(), IsError: true}, nil
		}
		return ToolResult{}, err
	}

	return ToolResult{Text: fmt.Sprintf("Appended to: %s/%s%s", rctx.Label(), args.Filename, ContextNote(rctx))}, nil
}
