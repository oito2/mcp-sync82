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
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// ArchiveMemoryTool implements archive_memory: marks old dated entries in
// progress or decisions as archived through store.ArchiveEntries. Entries
// with no date header are never archived, regardless of age.
type ArchiveMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// archiveMemoryArgs holds the decoded arguments of the archive_memory tool;
// its JSON tags match the property names declared in InputSchema.
type archiveMemoryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Filename         string `json:"filename"`
	KeepDays         int    `json:"keep_days,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// maxKeepDays is the largest accepted keep_days (100 years). It keeps the
// cutoff date computation within a valid range.
const maxKeepDays = 36500

// Name returns the MCP tool name, "archive_memory".
func (t *ArchiveMemoryTool) Name() string { return "archive_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ArchiveMemoryTool) Description() string {
	return "Archive old dated entries from progress.md or decisions.md, keeping only the last N days active. Entries without a date header are never archived. The project must be given via project or workspace_root, not taken from the last session."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property filename (progress or decisions), an optional
// keep_days and the project-resolution properties.
func (t *ArchiveMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root; unlike other tools, the last used project is refused."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"filename":           map[string]any{"type": "string", "enum": []string{"progress", "decisions"}, "description": "Which append-only file to archive."},
			"keep_days":          map[string]any{"type": "integer", "minimum": 1, "maximum": maxKeepDays, "description": "Entries older than this many days are archived (default 90, maximum 36500)."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename"},
	}
}

// Validate decodes raw into archiveMemoryArgs. It returns the arguments,
// with Filename trimmed and lower-cased and KeepDays defaulting to 90, or an
// error when filename is not an append-only standard kind or keep_days is
// outside 1 to maxKeepDays.
func (t *ArchiveMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args archiveMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if kind := NormalizeName(args.Filename); isAppendOnlyKind(kind) {
		args.Filename = kind
	} else {
		return nil, fmt.Errorf(`"filename" must be one of "progress" or "decisions", got %q`, args.Filename)
	}
	if args.KeepDays == 0 {
		args.KeepDays = 90
	} else if args.KeepDays < 1 || args.KeepDays > maxKeepDays {
		return nil, fmt.Errorf(`"keep_days" must be between 1 and %d`, maxKeepDays)
	}
	return args, nil
}

// Execute resolves the target project and archives the dated entries older
// than KeepDays days (UTC), then reports how many were archived and kept. It
// refuses, with an error result, to act on a project that was only taken
// from the last session; store failures are returned as errors.
func (t *ArchiveMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(archiveMemoryArgs)
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

	if refused := refuseRememberedTarget(rctx, "archive "+args.Filename); refused != nil {
		return *refused, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -args.KeepDays).Format("2006-01-02")
	result, err := s.ArchiveEntries(ctx, rctx.Project, rctx.Subproject, args.Filename, cutoff)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}

	label := rctx.Label()
	if result.Archived == 0 {
		plural := pluralize(result.Kept, "entry", "entries")
		return ToolResult{Text: fmt.Sprintf(
			"Nothing to archive in %s/%s: all %d %s are within the last %d days.",
			label, args.Filename, result.Kept, plural, args.KeepDays,
		)}, nil
	}

	archivedPlural := pluralize(result.Archived, "entry", "entries")
	keptSuffix := ""
	if result.NoDate > 0 {
		keptSuffix = fmt.Sprintf(", %d undated", result.NoDate)
	}

	text := fmt.Sprintf(
		"Archived %d %s from %s/%s%s\n  → moved to archive (%d archived)\n  → %s (%d kept%s)",
		result.Archived, archivedPlural, label, args.Filename, ContextNote(rctx),
		result.Archived, args.Filename, result.Kept, keptSuffix,
	)
	return ToolResult{Text: text}, nil
}
