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
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/oito2/mcp-sync82/internal/store"
)

// LoadProjectContextTool implements load_project_context: concatenate
// every non-blank document/entries kind into one context block, current
// state first, skipping blank kinds. "Blank" means the kind's content
// trims to the empty string, either because it holds only whitespace or
// because filteredContent left nothing after its since/max_entries
// filtering.
//
// "since" and "max_entries" optionally narrow the dated history of an
// entries-backed kind (progress, decisions, or a custom append kind), so a
// long-running project doesn't load every past entry each time. An
// overwrite-style document (memory, architecture, stack, next_steps, or a
// custom write-mode kind) represents current state, not history, so the
// filter never applies to one — see filteredContent. The response is cut
// at max_bytes.
type LoadProjectContextTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// loadProjectContextArgs holds the decoded arguments of the
// load_project_context tool; its JSON tags match the property names declared
// in InputSchema.
type loadProjectContextArgs struct {
	Project          string   `json:"project,omitempty"`
	Subproject       string   `json:"subproject,omitempty"`
	Path             string   `json:"path,omitempty"`
	WorkspaceRoot    string   `json:"workspace_root,omitempty"`
	SearchParentDirs bool     `json:"search_parent_dirs,omitempty"`
	Files            []string `json:"files,omitempty"`
	Since            string   `json:"since,omitempty"`
	MaxEntries       int      `json:"max_entries,omitempty"`
	MaxBytes         int      `json:"max_bytes,omitempty"`
}

// Size bounds, in bytes, of a load_project_context response: the default
// when max_bytes is omitted, and the minimum and maximum accepted.
const (
	defaultContextBytes = 200 << 10
	minContextBytes     = 1 << 10
	maxContextBytes     = 50 << 20
)

// currentStateKinds are the kinds placed first, in this order, so a
// size-bounded response keeps the project's current state before its
// history.
var currentStateKinds = []string{"memory", "architecture", "stack", "next_steps"}

// Name returns the MCP tool name, "load_project_context".
func (t *LoadProjectContextTool) Name() string { return "load_project_context" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *LoadProjectContextTool) Description() string {
	return `Load a project's entire memory (every non-blank file) concatenated into one context block, ready to paste into a new session. For a long-running project, "since" and/or "max_entries" limit how much of progress/decisions' dated history is included — overwrite-style files (memory, architecture, stack, next_steps) are always included in full, since they represent current state, not history. Those four come first; the response is cut at max_bytes (default 200 KB) with a note saying so.`
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties files, since, max_entries and max_bytes plus
// the project-resolution properties.
func (t *LoadProjectContextTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"files":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Only load these specific files/kinds, instead of everything."},
			"since":              map[string]any{"type": "string", "description": "Only include dated entries (progress, decisions, custom append kinds) on or after this date (\"YYYY-MM-DD\"). Undated entries are always included. Overwrite-style files are unaffected."},
			"max_entries":        map[string]any{"type": "integer", "description": "Only include the most recent N dated entries per append-only kind. Overwrite-style files are unaffected."},
			"max_bytes":          map[string]any{"type": "integer", "minimum": minContextBytes, "maximum": maxContextBytes, "description": "Cut the response at this many bytes (default 204800, i.e. 200 KB), with a note when it is cut."},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
	}
}

// Validate decodes raw into loadProjectContextArgs. It lower-cases and
// checks each file name and the since date, and defaults MaxBytes to
// defaultContextBytes. It returns the arguments, or an error when a file
// name or since is invalid, max_entries is negative or max_bytes is out of
// range.
func (t *LoadProjectContextTool) Validate(raw json.RawMessage) (any, error) {
	var args loadProjectContextArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	for i, f := range args.Files {
		kind, err := validateKind(f)
		if err != nil {
			return nil, err
		}
		args.Files[i] = kind
	}
	if args.Since != "" {
		if _, err := time.Parse("2006-01-02", args.Since); err != nil {
			return nil, fmt.Errorf(`"since" must be a valid date in the format "YYYY-MM-DD", got %q`, args.Since)
		}
	}
	if args.MaxEntries < 0 {
		return nil, fmt.Errorf(`"max_entries" must be >= 0`)
	}
	if args.MaxBytes == 0 {
		args.MaxBytes = defaultContextBytes
	} else if args.MaxBytes < minContextBytes || args.MaxBytes > maxContextBytes {
		return nil, fmt.Errorf(`"max_bytes" must be between %d and %d`, minContextBytes, maxContextBytes)
	}
	return args, nil
}

// Execute resolves the target project and concatenates the non-blank content
// of its kinds (all of them, or only the requested files) into one block,
// current-state kinds first, applying since and max_entries to dated
// history. The block is cut at MaxBytes. Store failures are returned as
// errors.
func (t *LoadProjectContextTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(loadProjectContextArgs)
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
	kinds, err := s.ListKinds(ctx, rctx.Project, rctx.Subproject, false)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}

	if len(args.Files) > 0 {
		wanted := make(map[string]bool, len(args.Files))
		for _, f := range args.Files {
			wanted[f] = true
		}
		filtered := make([]string, 0, len(kinds))
		for _, k := range kinds {
			if wanted[k] {
				filtered = append(filtered, k)
			}
		}
		kinds = filtered
	}
	kinds = currentStateFirst(kinds)

	var parts []string
	for _, k := range kinds {
		content, ok, err := filteredContent(ctx, s, rctx.Project, rctx.Subproject, k, args.Since, args.MaxEntries)
		if err != nil {
			return ToolResult{}, err
		}
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			continue
		}
		parts = append(parts, "## "+k+"\n\n"+trimmed)
	}

	if len(parts) == 0 {
		return ToolResult{Text: fmt.Sprintf("# Context: %s\n\n(no content yet)", rctx.Label())}, nil
	}
	text := fmt.Sprintf("# Context: %s\n\n%s", rctx.Label(), strings.Join(parts, "\n\n---\n\n"))
	return ToolResult{Text: truncateContext(text, args.MaxBytes)}, nil
}

// currentStateFirst returns a copy of kinds with the currentStateKinds
// present in it first, in that order, followed by the other kinds in their
// original order.
func currentStateFirst(kinds []string) []string {
	present := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		present[k] = true
	}
	out := make([]string, 0, len(kinds))
	for _, k := range currentStateKinds {
		if present[k] {
			out = append(out, k)
		}
	}
	for _, k := range kinds {
		if !slices.Contains(currentStateKinds, k) {
			out = append(out, k)
		}
	}
	return out
}

// truncateContext returns text unchanged when it fits in maxBytes;
// otherwise it cuts it at the last line break within maxBytes (or at a
// UTF-8 character boundary when there is none) and appends a note saying
// how much was kept and how to narrow the request.
func truncateContext(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	if nl := strings.LastIndexByte(text[:cut], '\n'); nl > 0 {
		cut = nl
	}
	return text[:cut] + fmt.Sprintf("\n\n[context truncated at %d of %d bytes — narrow it with \"since\", \"max_entries\" or \"files\", or raise \"max_bytes\"]", cut, len(text))
}

// filteredContent returns the content of kind in the given project and
// subproject of s, and whether the kind has any. When since and maxEntries
// are both zero it behaves like store.ReadContent. Otherwise an
// overwrite-style document is still returned in full, since it has no
// dated history, while an entries-backed kind is narrowed through
// Store.ReadEntriesSince: since (a "YYYY-MM-DD" date) keeps dated entries
// on or after that date and always keeps undated ones, and maxEntries
// (when positive) keeps only the most recent N of what remains, in
// ascending date and insertion order. Store failures are returned as
// errors.
func filteredContent(ctx context.Context, s *store.Store, project, subproject, kind, since string, maxEntries int) (content string, ok bool, err error) {
	if since == "" && maxEntries == 0 {
		return s.ReadContent(ctx, project, subproject, kind)
	}

	docContent, found, err := s.ReadDocument(ctx, project, subproject, kind)
	if err != nil {
		return "", false, err
	}
	if found {
		return docContent, true, nil
	}

	entries, err := s.ReadEntriesSince(ctx, project, subproject, kind, since, maxEntries)
	if err != nil {
		return "", false, err
	}
	if len(entries) == 0 {
		return "", false, nil
	}

	bodies := make([]string, len(entries))
	for i, e := range entries {
		bodies[i] = e.Body
	}
	return strings.Join(bodies, "\n\n"), true, nil
}
