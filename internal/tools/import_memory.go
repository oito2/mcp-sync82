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

	"github.com/oito2/mcp-sync82/internal/store"
)

// ImportMemoryTool implements import_memory: the inverse of export_memory
// — reads every "<kind>.md" file from a local directory and writes each
// into the resolved project, creating the project first if it doesn't
// exist yet. Like write_memory, it replaces an existing kind's content
// rather than merging it. The import itself is done by ImportProject,
// which the "sync82 import" command also uses.
type ImportMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// importMemoryArgs holds the decoded arguments of the import_memory tool;
// its JSON tags match the property names declared in InputSchema.
type importMemoryArgs struct {
	targetArgs
	InputDir string `json:"input_dir"`
	DryRun   bool   `json:"dry_run,omitempty"`
}

// Name returns the MCP tool name, "import_memory".
func (t *ImportMemoryTool) Name() string { return "import_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ImportMemoryTool) Description() string {
	return "Import a project's memory from plain Markdown files (one per kind, e.g. memory.md, progress.md) previously produced by export_memory — the inverse operation. Creates the project if it doesn't exist yet. Destructive per kind: an existing kind's content is overwritten, not merged, the same as write_memory; a <kind>.archived.md file restores that kind's archived entries, which are otherwise kept. input_dir must be absolute (or start with ~/ or HOME/) and may be any directory the server process can read — it is not confined to the vault or workspace. Symlinks, special files, files over 10 MB and README.md, CHANGELOG.md, LICENSE.md, CONTRIBUTING.md and CODE_OF_CONDUCT.md are skipped; the files may hold at most 64 MB in total. The import is all or nothing, and the result lists which files create a new kind and which overwrite an existing one; with dry_run: true nothing is written and the result shows what would happen."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property input_dir, an optional dry_run and the project-
// resolution properties.
func (t *ImportMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{Project: projectRefusesLastDescription}, map[string]any{
			"input_dir": map[string]any{"type": "string", "description": "Absolute directory (or starting with ~/ or HOME/) to read exported .md files from. Required. Not confined to the vault or workspace. Every \"<kind>.md\" file present is imported; files that aren't valid kind names, are empty, or are README/CHANGELOG/LICENSE/CONTRIBUTING/CODE_OF_CONDUCT are skipped."},
			"dry_run":   map[string]any{"type": "boolean", "description": "Report what the import would create and overwrite without writing anything."},
		}),
		"required": []string{"input_dir"},
	}
}

// Validate decodes raw into importMemoryArgs and checks that input_dir is an
// absolute directory (after ~ or HOME expansion). It returns the arguments
// with InputDir expanded and cleaned, or an error.
func (t *ImportMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args importMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	dir, err := absoluteDirArg("input_dir", args.InputDir)
	if err != nil {
		return nil, err
	}
	args.InputDir = dir
	return args, nil
}

// Execute resolves the target project, creating the vault if needed (but
// not on a dry run), and imports the Markdown files in input_dir through
// importProjectCore, or only reports what would change when DryRun is set
// (a dry run against a missing vault previews the import as if the vault
// were empty). A project taken only from the last session yields an error
// result instead of being imported into; an unresolved project returns the
// instructional result, and other failures are returned as errors.
func (t *ImportMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(importMemoryArgs)
	ctxArgs := args.contextArgs()
	resolve := t.Resolver.ResolveStoreCreating
	if args.DryRun {
		resolve = t.Resolver.ResolveStore
	}
	s, rctx, ready, err := resolve(ctx, t.Stores, ctxArgs)
	if ready != nil && args.DryRun && rctx.OK {
		// With a resolved project, the only result ResolveStore returns
		// is the missing-vault one: a dry run previews the import
		// against an empty vault instead of creating the vault file.
		ready = nil
	}
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	// Importing overwrites existing kinds, so a project taken only from the
	// last session is refused: it is never overwritten or recreated.
	if refused := refuseRememberedTarget(rctx, "import memory"); refused != nil {
		return *refused, nil
	}

	report, err := importProjectCore(ctx, s, rctx.Project, rctx.Subproject, args.InputDir, args.DryRun)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Text: FormatImportReport(report, rctx.Label()+ContextNote(rctx), args.InputDir, args.DryRun)}, nil
}
