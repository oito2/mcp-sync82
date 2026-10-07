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
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// The values of edit_entry's "action" argument.
const (
	editActionReplace   = "replace"
	editActionDelete    = "delete"
	editActionSupersede = "supersede"
)

// EditEntryTool implements edit_entry: replace, delete or supersede one
// entry of an append-only kind, identified by the id read_memory shows
// with with_ids. Like the other destructive tools, it refuses a project
// that was only taken from the last session.
type EditEntryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// editEntryArgs holds the decoded arguments of the edit_entry tool; its
// JSON tags match the property names declared in InputSchema.
type editEntryArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Filename         string `json:"filename"`
	EntryID          int64  `json:"entry_id"`
	Action           string `json:"action"`
	Content          string `json:"content,omitempty"`
	Confirm          bool   `json:"confirm,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Name returns the MCP tool name, "edit_entry".
func (t *EditEntryTool) Name() string { return "edit_entry" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *EditEntryTool) Description() string {
	return `Change one entry of an append-only memory file (progress, decisions, or a custom append kind), identified by entry_id — get the ids from read_memory with with_ids: true, or from search_memory with format "json". action "replace" rewrites the entry in place (its position is kept; for progress/decisions content must contain a "## YYYY-MM-DD" header, which also sets the entry's date). action "supersede" appends content as a new entry and adds a "> Superseded by entry N on YYYY-MM-DD." line to the old one, keeping the history auditable — prefer it when a decision changed rather than was wrong. action "delete" removes the entry permanently and requires confirm: true — ask the user first. The project must be given via project or workspace_root, not taken from the last session.`
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required properties filename, entry_id and action, the optional
// content and confirm, plus the project-resolution properties.
func (t *EditEntryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root; unlike other tools, the last used project is refused."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"filename":           map[string]any{"type": "string", "description": "The append-only file/kind the entry belongs to (e.g. \"progress\", \"decisions\", or a custom name)."},
			"entry_id":           map[string]any{"type": "integer", "minimum": 1, "description": "The entry's id, as shown by read_memory with with_ids: true (\"<!-- entry:N -->\") or by search_memory's JSON output (entry_id)."},
			"action":             map[string]any{"type": "string", "enum": []string{editActionReplace, editActionSupersede, editActionDelete}, "description": "\"replace\": rewrite the entry in place. \"supersede\": append content as a new entry and mark the old one as superseded. \"delete\": remove the entry (requires confirm: true)."},
			"content":            map[string]any{"type": "string", "description": "The new entry text, required for replace and supersede. For \"progress\"/\"decisions\", must contain a \"## YYYY-MM-DD\" header."},
			"confirm":            map[string]any{"type": "boolean", "description": "Must be true for action delete. Ask the user before setting this."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename", "entry_id", "action"},
	}
}

// Validate decodes raw into editEntryArgs. For replace and supersede it
// removes entry id marker lines from content and checks filename and
// content with validateAppendInput; for delete it checks filename, that no
// content is given and that confirm is true. It returns the arguments with
// Filename lower-cased, or an error when an argument is invalid, the kind
// is an overwrite-style standard kind, or entry_id is not positive.
func (t *EditEntryTool) Validate(raw json.RawMessage) (any, error) {
	var args editEntryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if args.EntryID < 1 {
		return nil, fmt.Errorf(`"entry_id" must be a positive integer`)
	}
	switch args.Action {
	case editActionReplace, editActionSupersede:
		args.Content = stripEntryMarkers(args.Content)
		kind, err := validateAppendInput(args.Filename, args.Content)
		if err != nil {
			return nil, err
		}
		args.Filename = kind
	case editActionDelete:
		kind, err := validateKind(args.Filename)
		if err != nil {
			return nil, err
		}
		if isStandardKind(kind) && !isAppendOnlyKind(kind) {
			return nil, fmt.Errorf("%q %w", kind, errAppendToDocumentKind)
		}
		if args.Content != "" {
			return nil, fmt.Errorf(`"content" is not used by action "delete"`)
		}
		if !args.Confirm {
			return nil, fmt.Errorf(`"confirm" must be true to delete an entry permanently — ask the user before proceeding`)
		}
		args.Filename = kind
	default:
		return nil, fmt.Errorf(`"action" must be %q, %q or %q, got %q`, editActionReplace, editActionSupersede, editActionDelete, args.Action)
	}
	return args, nil
}

// Execute resolves the target project and applies the action to the entry.
// It returns an error result (IsError) when the project was only taken
// from the last session, the kind is stored as an overwrite-style
// document, or the entry does not belong to the project and kind; an
// unresolved project returns the instructional result; a missing project
// yields the error built by wrapNotFound; other failures are returned as
// errors.
func (t *EditEntryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(editEntryArgs)
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
	if refused := refuseRememberedTarget(rctx, fmt.Sprintf("%s entry %d of %s", args.Action, args.EntryID, args.Filename)); refused != nil {
		return *refused, nil
	}
	mode, err := s.KindMode(ctx, rctx.Project, rctx.Subproject, args.Filename)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}
	if mode == store.KindStorageDocument {
		return ToolResult{IsError: true, Text: fmt.Sprintf("%q %s", args.Filename, errAppendToDocumentKind)}, nil
	}

	var done string
	switch args.Action {
	case editActionReplace:
		var date *string
		if isAppendOnlyKind(args.Filename) {
			d := extractFirstDate(args.Content)
			date = &d
		}
		err = s.UpdateEntry(ctx, rctx.Project, rctx.Subproject, args.Filename, args.EntryID, args.Content, date)
		done = fmt.Sprintf("Replaced entry %d", args.EntryID)
	case editActionDelete:
		err = s.DeleteEntry(ctx, rctx.Project, rctx.Subproject, args.Filename, args.EntryID)
		done = fmt.Sprintf("Deleted entry %d", args.EntryID)
	case editActionSupersede:
		date := ""
		if isAppendOnlyKind(args.Filename) {
			date = extractFirstDate(args.Content)
		}
		var newID int64
		newID, err = s.SupersedeEntry(ctx, rctx.Project, rctx.Subproject, args.Filename, args.EntryID, date, args.Content, supersededMark)
		done = fmt.Sprintf("Superseded entry %d with new entry %d", args.EntryID, newID)
	}
	if errors.Is(err, store.ErrNotFound) {
		return ToolResult{IsError: true, Text: fmt.Sprintf(
			"Entry %d not found in %s/%s. Use read_memory with with_ids: true to list the entry ids.",
			args.EntryID, rctx.Label(), args.Filename)}, nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Text: fmt.Sprintf("%s in %s/%s%s", done, rctx.Label(), args.Filename, ContextNote(rctx))}, nil
}

// supersededMark returns oldBody, without trailing line breaks, followed by
// a blank line and "> Superseded by entry <newID> on <today>." with today's
// UTC date.
func supersededMark(oldBody string, newID int64) string {
	return fmt.Sprintf("%s\n\n> Superseded by entry %d on %s.",
		strings.TrimRight(oldBody, "\r\n"), newID, time.Now().UTC().Format("2006-01-02"))
}
