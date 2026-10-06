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
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
)

// ReadMemoryTool implements read_memory: for an overwrite-style document,
// return its content; for an append-only entries collection, concatenate
// all non-archived entries in date order.
type ReadMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// readMemoryArgs holds the decoded arguments of the read_memory tool; its
// JSON tags match the property names declared in InputSchema.
type readMemoryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Filename         string `json:"filename"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Name returns the MCP tool name, "read_memory".
func (t *ReadMemoryTool) Name() string { return "read_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ReadMemoryTool) Description() string {
	return "Read a memory file's content. For an append-only kind (progress, decisions, or a custom append kind), returns every non-archived entry concatenated in date order."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property filename plus the project-resolution
// properties.
func (t *ReadMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"filename":           map[string]any{"type": "string", "description": "The file/kind to read (e.g. \"memory\", \"progress\", or a custom name)."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename"},
	}
}

// Validate decodes raw into readMemoryArgs and checks that filename is a
// valid kind name. It returns the arguments with Filename lower-cased, or an
// error.
func (t *ReadMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args readMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	kind, err := validateKind(args.Filename)
	if err != nil {
		return nil, err
	}
	args.Filename = kind
	return args, nil
}

// Execute resolves the target project and returns the kind's content with
// surrounding whitespace trimmed, or "(file is empty)" when nothing remains.
// It returns an error when the project or the kind does not exist.
func (t *ReadMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(readMemoryArgs)
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
	content, ok, err := s.ReadContent(ctx, rctx.Project, rctx.Subproject, args.Filename)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}
	if !ok {
		return ToolResult{}, fmt.Errorf("file not found: %s/%s", rctx.Label(), args.Filename)
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		trimmed = "(file is empty)"
	}
	return ToolResult{Text: trimmed}, nil
}
