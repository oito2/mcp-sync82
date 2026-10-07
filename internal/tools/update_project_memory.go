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

	"github.com/oito2/mcp-sync82/internal/store"
)

// updateProjectMemoryDescription is the description of
// update_project_memory: it tells the calling agent when to use the tool,
// how to decide which fields to fill, how each field is stored, and the
// date-header format.
const updateProjectMemoryDescription = `Save session work to the project memory in a single call.

USE THIS TOOL when the user says something vague like:
- "update the memory"
- "save what we did today"
- "record this session"
- "update the project notes"

FLOW — follow these steps before calling:

1. If the vault context is not already loaded, call load_project_context first
   to understand what is already recorded and avoid duplicating information.

2. Analyze the current conversation and identify what changed:
   - Decisions made → "decisions" field
   - Work completed, features implemented → "progress" field
   - Updated list of upcoming tasks → "next_steps" field
   - Changed understanding of the project purpose or status → "memory" field
   - Architectural changes → "architecture" field
   - New tools, libraries, or infrastructure added → "stack" field
   - Custom files (e.g. api, testing) → "custom" field

3. Populate only the fields that actually changed. Leave others unset.

PROJECT — pass "project" or "workspace_root" when any field overwrites
(next_steps, memory, architecture, stack, or a custom item with mode
"write"): a project taken only from the last session is refused for them.
Appends alone (progress, decisions, custom "append") may use it.

WRITE RULES — how each field is stored:
- "progress"     → APPENDED to progress   (log, never overwrites history)
- "decisions"    → APPENDED to decisions  (log, never overwrites history)
- "next_steps"   → OVERWRITES next_steps  (always reflects current state)
- "memory"       → OVERWRITES memory      (always reflects current state)
- "architecture" → OVERWRITES architecture
- "stack"        → OVERWRITES stack
- "custom"       → each item uses its own mode ("append" or "write", default "append")

FORMAT GUIDANCE:
- For appended fields (progress, decisions): the date header "## YYYY-MM-DD" is REQUIRED and enforced.
  Example: "## 2026-04-23\n- Implemented OAuth2 token validation\n- Files: auth/token.go"
  Without this header the write will be rejected with an error.
- For overwritten fields: provide the complete updated content, not just the diff.
- The call is all or nothing: if any field is invalid, nothing is written and every problem is listed.`

// UpdateProjectMemoryTool implements update_project_memory: save the work
// of a whole session in one call, appending to progress and decisions and
// overwriting the other standard kinds and any custom ones.
type UpdateProjectMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// customMemoryItem is one entry of the "custom" argument: content for a
// non-standard kind, written in the given mode ("append" when empty).
type customMemoryItem struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	Mode     string `json:"mode,omitempty"` // "append" | "write", default "append"
}

// updateProjectMemoryArgs holds the decoded arguments of the
// update_project_memory tool; its JSON tags match the property names
// declared in InputSchema. Every content field is a *string so that an
// unset field (nil) is distinguishable from one explicitly set to an empty
// string (non-nil), which Execute uses to decide whether any content was
// provided.
type updateProjectMemoryArgs struct {
	Project          string             `json:"project,omitempty"`
	Subproject       string             `json:"subproject,omitempty"`
	WorkspaceRoot    string             `json:"workspace_root,omitempty"`
	SearchParentDirs bool               `json:"search_parent_dirs,omitempty"`
	Path             string             `json:"path,omitempty"`
	Progress         *string            `json:"progress,omitempty"`
	Decisions        *string            `json:"decisions,omitempty"`
	NextSteps        *string            `json:"next_steps,omitempty"`
	Memory           *string            `json:"memory,omitempty"`
	Architecture     *string            `json:"architecture,omitempty"`
	Stack            *string            `json:"stack,omitempty"`
	Custom           []customMemoryItem `json:"custom,omitempty"`
}

// Name returns the MCP tool name, "update_project_memory".
func (t *UpdateProjectMemoryTool) Name() string { return "update_project_memory" }

// Description returns the usage instructions for the calling agent.
func (t *UpdateProjectMemoryTool) Description() string { return updateProjectMemoryDescription }

// InputSchema returns the JSON Schema of the tool's arguments: an object
// whose properties are the project-resolution ones, one string per standard
// kind and a custom array of extra files.
func (t *UpdateProjectMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project; the last used project is refused when any field overwrites."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"progress":           map[string]any{"type": "string", "description": "Appended to progress. Must contain a \"## YYYY-MM-DD\" date header."},
			"decisions":          map[string]any{"type": "string", "description": "Appended to decisions. Must contain a \"## YYYY-MM-DD\" date header."},
			"next_steps":         map[string]any{"type": "string", "description": "Overwrites next_steps with the full updated content."},
			"memory":             map[string]any{"type": "string", "description": "Overwrites memory with the full updated content."},
			"architecture":       map[string]any{"type": "string", "description": "Overwrites architecture with the full updated content."},
			"stack":              map[string]any{"type": "string", "description": "Overwrites stack with the full updated content."},
			"custom": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"filename": map[string]any{"type": "string", "description": "Custom file/kind name."},
						"content":  map[string]any{"type": "string", "description": "Content for this file."},
						"mode":     map[string]any{"type": "string", "enum": []string{"append", "write"}, "description": "Defaults to \"append\"."},
					},
					"required": []string{"filename", "content"},
				},
				"description": "Custom files outside the six standard ones, each with its own write mode.",
			},
			"path": map[string]any{"type": "string", "description": PathDescription},
		},
	}
}

// maxCustomItems is the maximum number of custom files one
// update_project_memory call may write.
const maxCustomItems = 50

// Validate decodes raw into updateProjectMemoryArgs. It lower-cases and
// checks the custom file names, removes entry id marker lines from every
// content field, and checks each provided field with the rules its write
// will apply (validateAppendInput for the appended ones, including the
// date header of progress and decisions; validateWriteInput for the
// overwritten ones). It returns the arguments, or an error listing every
// problem: too many custom items, total content over maxContentSize, a
// custom item naming a standard kind or with an unknown mode, or a field
// its write would reject — so a bad field never leaves the others
// half-written.
func (t *UpdateProjectMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args updateProjectMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}

	if len(args.Custom) > maxCustomItems {
		return nil, fmt.Errorf(`"custom" accepts at most %d items, got %d`, maxCustomItems, len(args.Custom))
	}
	total := 0
	for _, field := range []*string{args.Progress, args.Decisions, args.NextSteps, args.Memory, args.Architecture, args.Stack} {
		if field != nil {
			total += len(*field)
		}
	}
	for _, item := range args.Custom {
		total += len(item.Content)
	}
	if total > maxContentSize {
		return nil, fmt.Errorf("the content of one call exceeds the %d byte limit in total", maxContentSize)
	}

	for i, item := range args.Custom {
		kind, err := validateKind(item.Filename)
		if err != nil {
			return nil, err
		}
		if isStandardKind(kind) {
			return nil, fmt.Errorf(`custom item %q: standard files must use their own field (%s), not "custom"`, kind, strings.Join(standardKinds, ", "))
		}
		args.Custom[i].Filename = kind
		if item.Mode != "" && item.Mode != "append" && item.Mode != "write" {
			return nil, fmt.Errorf(`custom item %q: "mode" must be "append" or "write"`, item.Filename)
		}
	}

	var problems []string
	check := func(name string, content *string, isAppend bool) {
		if content == nil {
			return
		}
		*content = stripEntryMarkers(*content)
		var err error
		if isAppend {
			_, err = validateAppendInput(name, *content)
		} else {
			_, err = validateWriteInput(name, *content)
		}
		if err != nil {
			problems = append(problems, name+": "+err.Error())
		}
	}
	check("progress", args.Progress, true)
	check("decisions", args.Decisions, true)
	check("next_steps", args.NextSteps, false)
	check("memory", args.Memory, false)
	check("architecture", args.Architecture, false)
	check("stack", args.Stack, false)
	for i := range args.Custom {
		check(args.Custom[i].Filename, &args.Custom[i].Content, args.Custom[i].Mode != "write")
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments (nothing was written):\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// memoryOp is one write planned by update_project_memory: an append to, or
// an overwrite of, the kind filename with content.
type memoryOp struct {
	isAppend bool
	filename string
	content  string
}

// Execute resolves the target project and applies every provided field in
// one transaction: all of them are written, or none is. When any field
// overwrites, it refuses, with an error result, a project that was only
// taken from the last session. A field the store state rejects (an append
// to a custom kind stored as a document) makes the call an error result
// listing every such field, with nothing written; a missing project yields
// the error built by wrapNotFound. It returns a plain notice when no field
// was provided.
func (t *UpdateProjectMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(updateProjectMemoryArgs)

	hasContent := args.Progress != nil || args.Decisions != nil || args.NextSteps != nil ||
		args.Memory != nil || args.Architecture != nil || args.Stack != nil || len(args.Custom) > 0
	if !hasContent {
		return ToolResult{Text: "Nothing to update: no content fields were provided (progress, decisions, next_steps, memory, architecture, stack, custom)."}, nil
	}

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

	var ops []memoryOp
	if args.Progress != nil {
		ops = append(ops, memoryOp{isAppend: true, filename: "progress", content: *args.Progress})
	}
	if args.Decisions != nil {
		ops = append(ops, memoryOp{isAppend: true, filename: "decisions", content: *args.Decisions})
	}
	if args.NextSteps != nil {
		ops = append(ops, memoryOp{filename: "next_steps", content: *args.NextSteps})
	}
	if args.Memory != nil {
		ops = append(ops, memoryOp{filename: "memory", content: *args.Memory})
	}
	if args.Architecture != nil {
		ops = append(ops, memoryOp{filename: "architecture", content: *args.Architecture})
	}
	if args.Stack != nil {
		ops = append(ops, memoryOp{filename: "stack", content: *args.Stack})
	}
	for _, item := range args.Custom {
		ops = append(ops, memoryOp{isAppend: item.Mode != "write", filename: item.Filename, content: item.Content})
	}

	var overwrites []string
	for _, op := range ops {
		if !op.isAppend {
			overwrites = append(overwrites, op.filename)
		}
	}
	if len(overwrites) > 0 {
		if refused := refuseRememberedTarget(rctx, "overwrite "+strings.Join(overwrites, ", ")); refused != nil {
			return *refused, nil
		}
	}

	// Plan every write first, then apply them together: a field the store
	// rejects must not leave the others written.
	writes := make([]store.KindWrite, 0, len(ops))
	var appended, written, errs []string
	for _, op := range ops {
		var w store.KindWrite
		var opErr error
		if op.isAppend {
			w, opErr = planAppend(ctx, s, rctx.Project, rctx.Subproject, op.filename, op.content)
		} else {
			w, opErr = planWrite(ctx, s, rctx.Project, rctx.Subproject, op.filename, op.content)
		}
		if opErr != nil {
			if !errors.Is(opErr, errAppendToDocumentKind) {
				return ToolResult{}, opErr
			}
			errs = append(errs, "  - "+op.filename+": "+opErr.Error())
			continue
		}
		writes = append(writes, w)
		if op.isAppend {
			appended = append(appended, op.filename)
		} else {
			written = append(written, op.filename)
		}
	}
	if len(errs) > 0 {
		return ToolResult{IsError: true, Text: fmt.Sprintf("Project: %s%s\nNothing was written; fix these fields and call again:\n%s",
			rctx.Label(), ContextNote(rctx), strings.Join(errs, "\n"))}, nil
	}
	if err := s.WriteKinds(ctx, rctx.Project, rctx.Subproject, writes); err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}

	parts := []string{fmt.Sprintf("Project: %s%s", rctx.Label(), ContextNote(rctx))}
	if len(appended) > 0 {
		parts = append(parts, "Appended: "+strings.Join(appended, ", "))
	}
	if len(written) > 0 {
		parts = append(parts, "Overwritten: "+strings.Join(written, ", "))
	}
	return ToolResult{Text: strings.Join(parts, "\n")}, nil
}
