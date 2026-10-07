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
	"fmt"

	"github.com/oito2/mcp-sync82/internal/store"
)

// WriteMemoryTool implements write_memory: overwrite a memory file's
// entire content. It works on any kind, including the append-only ones
// (progress, decisions). Overwriting an append-only kind splits the
// incoming text into dated sections and replaces the whole entries
// collection; overwriting a document kind replaces it in place, keeping no
// history. Validation and execution delegate to validateWriteInput and
// writeMemoryCore, shared with update_project_memory.
type WriteMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// writeMemoryArgs holds the decoded arguments of the write_memory tool; its
// JSON tags match the property names declared in InputSchema.
type writeMemoryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Filename         string `json:"filename"`
	Content          string `json:"content"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Name returns the MCP tool name, "write_memory".
func (t *WriteMemoryTool) Name() string { return "write_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *WriteMemoryTool) Description() string {
	return "Overwrite a memory file's entire content. Destructive: for append-only kinds (progress, decisions, or a custom kind created with append_memory), this replaces every non-archived entry, not just the latest one — use append_memory to add without losing prior entries. Archived entries are kept. The project must be given via project or workspace_root, not taken from the last session."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties filename and content plus the project-
// resolution properties.
func (t *WriteMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root; unlike other tools, the last used project is refused."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"filename":           map[string]any{"type": "string", "description": "The file/kind to overwrite (e.g. \"memory\", \"progress\", or a custom name)."},
			"content":            map[string]any{"type": "string", "description": "The new full content."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename", "content"},
	}
}

// Validate decodes raw into writeMemoryArgs, removes entry id marker lines
// from content and checks filename and content with validateWriteInput. It
// returns the arguments with Filename normalized to its lower-case kind
// name, or an error listing every violated rule.
func (t *WriteMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args writeMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	args.Content = stripEntryMarkers(args.Content)
	kind, err := validateWriteInput(args.Filename, args.Content)
	if err != nil {
		return nil, err
	}
	args.Filename = kind
	return args, nil
}

// Execute resolves the target project and replaces the kind's content
// through writeMemoryCore. It refuses, with an error result, to overwrite
// a project that was only taken from the last session. An unresolved
// project returns the instructional result; store failures, including a
// missing project, are returned as errors.
func (t *WriteMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(writeMemoryArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, ContextArgs{
		Project: args.Project, Subproject: args.Subproject, Path: args.Path,
		WorkspaceRoot: args.WorkspaceRoot, SearchParentDirs: args.SearchParentDirs,
	})
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	if refused := refuseRememberedTarget(rctx, "overwrite "+args.Filename); refused != nil {
		return *refused, nil
	}
	if err := writeMemoryCore(ctx, s, rctx.Project, rctx.Subproject, args.Filename, args.Content); err != nil {
		return ToolResult{}, err
	}

	return ToolResult{Text: fmt.Sprintf("Written: %s/%s%s", rctx.Label(), args.Filename, ContextNote(rctx))}, nil
}
