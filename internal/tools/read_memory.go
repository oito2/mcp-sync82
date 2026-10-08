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
	targetArgs
	Filename string `json:"filename"`
	WithIDs  bool   `json:"with_ids,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

// readMemoryBytes is the default size cap of a read_memory response; the
// max_bytes argument may set it between minContextBytes and
// maxContextBytes.
const readMemoryBytes = 1 << 20

// Name returns the MCP tool name, "read_memory".
func (t *ReadMemoryTool) Name() string { return "read_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ReadMemoryTool) Description() string {
	return `Read a memory file's content. For an append-only kind (progress, decisions, or a custom append kind), returns every non-archived entry concatenated in date order. With with_ids: true, each entry is preceded by a "<!-- entry:N -->" line giving the id that edit_entry takes; these lines are never stored if the content is written back. The response is cut at max_bytes (default 1 MB) with a note saying so; load_project_context with since or max_entries reads part of a long log.`
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property filename plus the project-resolution
// properties.
func (t *ReadMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{}, map[string]any{
			"filename":  map[string]any{"type": "string", "description": "The file/kind to read (e.g. \"memory\", \"progress\", or a custom name)."},
			"with_ids":  map[string]any{"type": "boolean", "description": "Put a \"<!-- entry:N -->\" line before each entry of an append-only kind, with the id edit_entry takes. No effect on overwrite-style files."},
			"max_bytes": map[string]any{"type": "integer", "minimum": minContextBytes, "maximum": maxContextBytes, "description": "Size cap of the response in bytes (default 1048576, i.e. 1 MB). Longer content is cut at a line break, with a note."},
		}),
		"required": []string{"filename"},
	}
}

// Validate decodes raw into readMemoryArgs, checks that filename is a
// valid kind name and that max_bytes, when given, is between
// minContextBytes and maxContextBytes, defaulting it to readMemoryBytes. It
// returns the arguments with Filename lower-cased, or an error listing
// every problem.
func (t *ReadMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args readMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	var problems []string
	kind, err := validateKind(args.Filename)
	if err != nil {
		problems = append(problems, err.Error())
	}
	args.Filename = kind
	switch {
	case args.MaxBytes == 0:
		args.MaxBytes = readMemoryBytes
	case args.MaxBytes < minContextBytes || args.MaxBytes > maxContextBytes:
		problems = append(problems, fmt.Sprintf(`"max_bytes" must be between %d and %d`, minContextBytes, maxContextBytes))
	}
	if err := problemsError(problems); err != nil {
		return nil, err
	}
	return args, nil
}

// Execute resolves the target project and returns the kind's content with
// surrounding whitespace trimmed, or "(file is empty)" when nothing remains,
// cut to MaxBytes by capReadMemory.
// With WithIDs, an entries-backed kind is returned entry by entry, each
// preceded by its entryMarker line. It returns an error when the project or
// the kind does not exist.
func (t *ReadMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(readMemoryArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, args.contextArgs())
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
	return ToolResult{Text: capReadMemory(trimmed, args.MaxBytes)}, nil
}

// capReadMemory returns text when it fits in maxBytes, and otherwise its
// start, cut at a line break by cutPoint, followed by a note giving the
// sizes; the result, note included, is never longer than maxBytes.
func capReadMemory(text string, maxBytes int) string {
	if len(text) <= maxBytes {
		return text
	}
	note := fmt.Sprintf("\n\n[cut: %d of %d bytes shown; pass a larger max_bytes, or use load_project_context with since or max_entries to read part of a log]", maxBytes, len(text))
	cut := cutPoint(text, maxBytes-len(note))
	return text[:cut] + fmt.Sprintf("\n\n[cut: %d of %d bytes shown; pass a larger max_bytes, or use load_project_context with since or max_entries to read part of a log]", cut, len(text))
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
