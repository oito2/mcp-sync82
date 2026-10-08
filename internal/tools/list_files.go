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

// ListFilesTool implements list_files: list every document kind and entry
// kind of the resolved project. "File" here means a kind stored in the
// vault database, not a file on disk.
type ListFilesTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// listFilesArgs holds the decoded arguments of the list_files tool; its JSON
// tags match the property names declared in InputSchema.
type listFilesArgs struct {
	targetArgs
	Metadata bool   `json:"metadata,omitempty"`
	Format   string `json:"format,omitempty"`
}

// Name returns the MCP tool name, "list_files".
func (t *ListFilesTool) Name() string { return "list_files" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ListFilesTool) Description() string {
	return "List every memory file (document or entries kind) recorded for a project."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties metadata and format plus the project-
// resolution properties.
func (t *ListFilesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{}, map[string]any{
			"metadata": map[string]any{"type": "boolean", "description": "When true, include size, estimated tokens, and last-modified date per file."},
			"format":   formatProperty(),
		}),
	}
}

// Validate decodes raw into listFilesArgs and normalizes format to "text" or
// "json". It returns the arguments, or an error when format is invalid.
func (t *ListFilesTool) Validate(raw json.RawMessage) (any, error) {
	var args listFilesArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	format, err := validateFormat(args.Format)
	if err != nil {
		return nil, err
	}
	args.Format = format
	return args, nil
}

// Execute resolves the target project and lists its kinds, with size,
// estimated tokens and last-modified date per kind when Metadata is set. The
// output is text or JSON, and the list is also returned as structured
// content in both. An unresolved project returns the instructional result; a
// missing project yields the error built by wrapNotFound, and other store
// failures are returned as errors.
func (t *ListFilesTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(listFilesArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, args.contextArgs())
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
	if len(kinds) == 0 && args.Format != "json" {
		return ToolResult{Text: fmt.Sprintf("No files found in project %q.", rctx.Label()),
			Structured: fileList{Project: rctx.Label(), Vault: rctx.DBPath, Files: []fileEntry{}}}, nil
	}

	list := fileList{Project: rctx.Label(), Vault: rctx.DBPath, Files: []fileEntry{}}
	var lines []string
	for _, k := range kinds {
		entry := fileEntry{Name: k}
		if args.Metadata {
			info, err := s.Metadata(ctx, rctx.Project, rctx.Subproject, k)
			if errors.Is(err, store.ErrNotFound) {
				continue // deleted by another call since it was listed
			}
			if err != nil {
				return ToolResult{}, err
			}
			entry.SizeBytes, entry.EstimatedTokens, entry.LastModified = &info.SizeBytes, &info.EstimatedTokens, info.LastModified
			lines = append(lines, fmt.Sprintf("- %s  (%dB, ~%d tokens, modified: %s)", k, info.SizeBytes, info.EstimatedTokens, info.LastModified))
		} else {
			lines = append(lines, "- "+k)
		}
		list.Files = append(list.Files, entry)
	}

	if args.Format == "json" {
		return jsonResult(list, false)
	}
	return ToolResult{Text: fmt.Sprintf("Files in %q%s:\n%s", rctx.Label(), ContextNote(rctx), strings.Join(lines, "\n")), Structured: list}, nil
}

// OutputSchema returns the JSON Schema of fileList, the structured content
// of every successful list_files result.
func (t *ListFilesTool) OutputSchema() map[string]any {
	return schemaObject(map[string]any{
		"project": schemaString(),
		"vault":   schemaString(),
		"files": schemaArray(schemaObject(map[string]any{
			"name":             schemaString(),
			"size_bytes":       schemaInteger(),
			"estimated_tokens": schemaInteger(),
			"last_modified":    schemaString(),
		}, "name")),
	}, "project", "vault", "files")
}

// fileList is list_files' JSON result: the project label, the vault path
// and the files found.
type fileList struct {
	Project string      `json:"project"`
	Vault   string      `json:"vault"`
	Files   []fileEntry `json:"files"`
}

// fileEntry is one file in a fileList; the size and date fields are set
// only when metadata was requested.
type fileEntry struct {
	Name            string `json:"name"`
	SizeBytes       *int   `json:"size_bytes,omitempty"`
	EstimatedTokens *int   `json:"estimated_tokens,omitempty"`
	LastModified    string `json:"last_modified,omitempty"`
}
