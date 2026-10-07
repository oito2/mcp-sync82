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
// all non-archived entries in date order, each preceded by an entry id
// marker line when with_ids is set.
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
	WithIDs          bool   `json:"with_ids,omitempty"`
}

// Name returns the MCP tool name, "read_memory".
func (t *ReadMemoryTool) Name() string { return "read_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ReadMemoryTool) Description() string {
	return `Read a memory file's content. For an append-only kind (progress, decisions, or a custom append kind), returns every non-archived entry concatenated in date order. With with_ids: true, each entry is preceded by a "<!-- entry:N -->" line giving the id that edit_entry takes; these lines are never stored if the content is written back.`
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
			"with_ids":           map[string]any{"type": "boolean", "description": "Put a \"<!-- entry:N -->\" line before each entry of an append-only kind, with the id edit_entry takes. No effect on overwrite-style files."},
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
// With WithIDs, an entries-backed kind is returned entry by entry, each
// preceded by its entryMarker line. It returns an error when the project or
// the kind does not exist.
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
	content, ok, err := readMemoryContent(ctx, s, rctx.Project, rctx.Subproject, args.Filename, args.WithIDs)
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

// readMemoryContent returns the readable content of kind like
// store.ReadContent does. When withIDs is set and kind is stored as
// entries, every entry body is preceded by its entryMarker line. ok is
// false when the kind has no content. Store errors are returned unchanged.
func readMemoryContent(ctx context.Context, s *store.Store, project, subproject, kind string, withIDs bool) (content string, ok bool, err error) {
	if !withIDs {
		return s.ReadContent(ctx, project, subproject, kind)
	}
	mode, err := s.KindMode(ctx, project, subproject, kind)
	if err != nil {
		return "", false, err
	}
	if mode != store.KindStorageEntries {
		return s.ReadContent(ctx, project, subproject, kind)
	}
	entries, err := s.ReadEntries(ctx, project, subproject, kind, false)
	if err != nil {
		return "", false, err
	}
	if len(entries) == 0 {
		return "", false, nil
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = entryMarker(e.ID) + "\n" + e.Body
	}
	return strings.Join(parts, "\n\n"), true, nil
}
