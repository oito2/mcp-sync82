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

// ExportMemoryTool implements export_memory: writes every memory file of
// a project out to plain .md files on disk, for browsing outside an MCP
// client or a git-diffable history. The export itself is done by
// ExportProject, which the "sync82 export" command also uses.
type ExportMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// exportMemoryArgs holds the decoded arguments of the export_memory tool;
// its JSON tags match the property names declared in InputSchema.
type exportMemoryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	OutputDir        string `json:"output_dir"`
	Overwrite        bool   `json:"overwrite,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Name returns the MCP tool name, "export_memory".
func (t *ExportMemoryTool) Name() string { return "export_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ExportMemoryTool) Description() string {
	return "Export a project's memory to plain Markdown files (one per kind, e.g. memory.md, progress.md) on the local filesystem, for browsing or git-diffing outside an MCP client. output_dir must be absolute (or start with ~/ or HOME/) and may be any directory the server process can write to — it is not confined to the project's own vault or workspace; it is created if missing. Existing files are never replaced unless overwrite is true, and a destination that is a symlink or special file is always refused. A kind with archived entries also gets a <kind>.archived.md file holding them."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property output_dir, an optional overwrite and the
// project-resolution properties.
func (t *ExportMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"output_dir":         map[string]any{"type": "string", "description": "Absolute directory (or starting with ~/ or HOME/) to write the exported .md files into. Required. Not confined to the vault or workspace. Created if missing."},
			"overwrite":          map[string]any{"type": "boolean", "description": "Replace .md files that already exist in output_dir. Without it, the export is refused when any destination file exists, and nothing is written."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"output_dir"},
	}
}

// Validate decodes raw into exportMemoryArgs and checks that output_dir is
// an absolute directory (after ~ or HOME expansion). It returns the
// arguments with OutputDir expanded and cleaned, or an error.
func (t *ExportMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args exportMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	dir, err := absoluteDirArg("output_dir", args.OutputDir)
	if err != nil {
		return nil, err
	}
	args.OutputDir = dir
	return args, nil
}

// Execute resolves the target project and exports its kinds to output_dir
// through exportProjectCore. It reports the number of files written;
// existing destination files without Overwrite yield an error result, and
// other failures are returned as errors.
func (t *ExportMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(exportMemoryArgs)
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

	count, err := exportProjectCore(ctx, s, rctx.Project, rctx.Subproject, args.OutputDir, args.Overwrite)
	if errors.Is(err, errExportWouldOverwrite) {
		return ToolResult{Text: fmt.Sprintf("Nothing exported for %s%s: %v", rctx.Label(), ContextNote(rctx), err), IsError: true}, nil
	}
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}
	if count == 0 {
		return ToolResult{Text: fmt.Sprintf("Nothing to export for %s%s: no files found.", rctx.Label(), ContextNote(rctx))}, nil
	}

	plural := pluralize(count, "file", "files")
	return ToolResult{Text: fmt.Sprintf("Exported %d %s from %s%s to %s", count, plural, rctx.Label(), ContextNote(rctx), args.OutputDir)}, nil
}
