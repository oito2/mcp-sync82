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

// DeleteMemoryTool implements delete_memory: delete a custom
// document/entries kind. The six standard kinds are protected and cannot
// be deleted.
type DeleteMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// deleteMemoryArgs holds the decoded arguments of the delete_memory tool;
// its JSON tags match the property names declared in InputSchema.
type deleteMemoryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Filename         string `json:"filename"`
	Confirm          bool   `json:"confirm"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Name returns the MCP tool name, "delete_memory".
func (t *DeleteMemoryTool) Name() string { return "delete_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *DeleteMemoryTool) Description() string {
	return "Permanently delete a custom memory file. Requires confirm: true — ask the user before calling this with confirm: true. The six standard files (memory, architecture, stack, decisions, progress, next_steps) cannot be deleted this way — use write_memory to clear their content instead. The project must be given via project or workspace_root, not taken from the last session."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties filename and confirm plus the project-
// resolution properties.
func (t *DeleteMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root; unlike other tools, the last used project is refused."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"filename":           map[string]any{"type": "string", "description": "The custom file/kind to delete."},
			"confirm":            map[string]any{"type": "boolean", "description": "Must be true to confirm permanent deletion. Ask the user before setting this."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename", "confirm"},
	}
}

// Validate decodes raw into deleteMemoryArgs. It returns the arguments with
// Filename lower-cased, or an error when filename is not a valid kind name,
// names one of the standard kinds, or confirm is not true.
func (t *DeleteMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args deleteMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	kind, err := validateKind(args.Filename)
	if err != nil {
		return nil, err
	}
	args.Filename = kind
	if isStandardKind(kind) {
		return nil, fmt.Errorf("cannot delete standard file %q; use write_memory to clear its content instead", kind)
	}
	if !args.Confirm {
		return nil, fmt.Errorf(`"confirm" must be true to confirm permanent deletion — ask the user before proceeding`)
	}
	return args, nil
}

// Execute resolves the target project and deletes the custom kind. It
// refuses, with an error result, to act on a project that was only taken
// from the last session; a missing project or kind and other store failures
// are returned as errors.
func (t *DeleteMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(deleteMemoryArgs)
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
	if refused := refuseRememberedTarget(rctx, "delete "+args.Filename); refused != nil {
		return *refused, nil
	}
	if err := s.DeleteKind(ctx, rctx.Project, rctx.Subproject, args.Filename); err != nil {
		return ToolResult{}, err
	}

	return ToolResult{Text: fmt.Sprintf("Deleted: %s/%s%s", rctx.Label(), args.Filename, ContextNote(rctx))}, nil
}
