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

// SearchMemoryTool implements search_memory: case-insensitive substring
// search. Scope rule: no project → search everything; project only → that
// project's own documents/entries plus all its subprojects; project +
// subproject → that subproject only.
type SearchMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// searchMemoryArgs holds the decoded arguments of the search_memory tool;
// its JSON tags match the property names declared in InputSchema.
type searchMemoryArgs struct {
	Query            string `json:"query"`
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	ContextLines     int    `json:"context_lines,omitempty"`
	Format           string `json:"format,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// Bounds on a search's output: the maximum context lines per match, and
// the maximum size in bytes of the whole response text.
const (
	maxSearchContextLines = 20
	maxSearchResponseSize = 1 << 20
)

// Name returns the MCP tool name, "search_memory".
func (t *SearchMemoryTool) Name() string { return "search_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *SearchMemoryTool) Description() string {
	return "Case-insensitive substring search across memory files (accents included: \"decisão\" finds \"DECISÃO\"). No project given searches the whole vault; project only searches that project and all its subprojects; project+subproject searches just that subproject. Each match is labeled project/file:line, or project/file[YYYY-MM-DD]:line for an entry of a dated log (line counted within that entry)."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property query, the optional properties limit, offset,
// context_lines and format, and the project-resolution properties.
func (t *SearchMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":              map[string]any{"type": "string", "description": "Substring to search for (case-insensitive, literal — not a pattern, single line)."},
			"project":            map[string]any{"type": "string", "description": "Limit the search to this project (and its subprojects, unless subproject is also given)."},
			"subproject":         map[string]any{"type": "string", "description": "Limit the search to this specific subproject (requires project or workspace_root)."},
			"limit":              map[string]any{"type": "integer", "description": "Maximum number of results to return (1-1000, default 100)."},
			"offset":             map[string]any{"type": "integer", "description": "Number of results to skip, for pagination (default 0)."},
			"context_lines":      map[string]any{"type": "integer", "minimum": 0, "maximum": maxSearchContextLines, "description": "Number of surrounding lines to include per match (0-20, default 0)."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json when project isn't given."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
			"format":             formatProperty(),
		},
		"required": []string{"query"},
	}
}

// Validate decodes raw into searchMemoryArgs and applies the defaults (Limit
// 100, text format). It returns the arguments, or an error listing every
// problem: an empty or multi-line query, a subproject without a project, or
// an out-of-range limit, offset or context_lines.
func (t *SearchMemoryTool) Validate(raw json.RawMessage) (any, error) {
	var args searchMemoryArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}

	var problems []string
	if args.Query == "" {
		problems = append(problems, `"query" is required and must not be empty`)
	} else if strings.ContainsAny(args.Query, "\r\n") {
		problems = append(problems, `"query" must be a single line: matches are found line by line`)
	}
	if strings.TrimSpace(args.Subproject) != "" && strings.TrimSpace(args.Project) == "" && strings.TrimSpace(args.WorkspaceRoot) == "" {
		problems = append(problems, `"subproject" requires "project" or "workspace_root"`)
	}
	if args.Limit == 0 {
		args.Limit = 100
	} else if args.Limit < 1 || args.Limit > 1000 {
		problems = append(problems, `"limit" must be between 1 and 1000`)
	}
	if args.Offset < 0 {
		problems = append(problems, `"offset" must be >= 0`)
	}
	if args.ContextLines < 0 || args.ContextLines > maxSearchContextLines {
		problems = append(problems, fmt.Sprintf(`"context_lines" must be between 0 and %d`, maxSearchContextLines))
	}
	format, err := validateFormat(args.Format)
	if err != nil {
		problems = append(problems, err.Error())
	}
	args.Format = format
	if len(problems) > 0 {
		return nil, fmt.Errorf("invalid arguments:\n- %s", strings.Join(problems, "\n- "))
	}
	return args, nil
}

// Execute searches the resolved scope for the query and returns the matching
// lines with their location, optional context and pagination hints, as text
// or JSON. The output is also capped in size, with the next offset reported
// when it is cut. A missing vault or an unusable .sync82.json yields an
// error result; store failures are returned as errors.
func (t *SearchMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(searchMemoryArgs)
	rctx := t.resolveScope(args)
	if rctx.Problem != "" {
		return ToolResult{IsError: true, Text: rctx.Problem}, nil
	}

	s, err := t.Stores.GetExisting(ctx, rctx.DBPath)
	if errors.Is(err, store.ErrVaultNotFound) {
		return vaultMissingResult(rctx.DBPath), nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	project, subproject := rctx.Project, rctx.Subproject
	if project != "" {
		t.Resolver.RememberIfExists(ctx, s, rctx)
	}

	results, scanTruncated, err := s.SearchText(ctx, args.Query, store.SearchScope{Project: project, Subproject: subproject}, args.Offset, args.Limit, args.ContextLines)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, FormatLabel(project, subproject))
	}

	moreResults := len(results) > args.Limit
	if moreResults {
		results = results[:args.Limit]
	}
	if len(results) == 0 && args.Format != "json" {
		text := fmt.Sprintf("No results for %q", args.Query)
		if scanTruncated {
			text += " (the search stopped early on a very large vault; narrow it with project)"
		}
		return ToolResult{Text: text}, nil
	}

	report := searchReport{Query: args.Query, Results: []searchHit{}, ScanTruncated: scanTruncated}
	lines := make([]string, 0, len(results)+2)
	size := 0
	for i, r := range results {
		location := FormatLabel(r.Project, r.Subproject) + "/" + r.Kind
		if r.EntryDate != "" {
			location += "[" + r.EntryDate + "]"
		}
		line := fmt.Sprintf("%s:%d  %s", location, r.LineNumber, r.Line)
		if args.ContextLines > 0 && (len(r.ContextBefore) > 0 || len(r.ContextAfter) > 0) {
			ctxLines := make([]string, 0, len(r.ContextBefore)+len(r.ContextAfter))
			for _, l := range r.ContextBefore {
				ctxLines = append(ctxLines, "  "+l)
			}
			for _, l := range r.ContextAfter {
				ctxLines = append(ctxLines, "  "+l)
			}
			line += "\nContext:\n" + strings.Join(ctxLines, "\n") + "\n---"
		}
		if size+len(line) > maxSearchResponseSize && i > 0 {
			lines = append(lines, fmt.Sprintf("(output truncated after %d results; use offset %d to continue, or fewer context_lines)", i, args.Offset+i))
			report.NextOffset = args.Offset + i
			moreResults = false
			break
		}
		size += len(line) + 1
		lines = append(lines, line)
		report.Results = append(report.Results, searchHit{
			Project: r.Project, Subproject: r.Subproject, File: r.Kind, EntryDate: r.EntryDate,
			Line: r.LineNumber, Text: r.Line, ContextBefore: r.ContextBefore, ContextAfter: r.ContextAfter,
		})
	}
	if moreResults {
		report.NextOffset = args.Offset + args.Limit
	}

	if args.Format == "json" {
		return jsonResult(report, false)
	}
	if moreResults {
		lines = append(lines, fmt.Sprintf("(limit of %d results reached, use offset to paginate)", args.Limit))
	}
	if scanTruncated {
		lines = append(lines, "(the search stopped early on a very large vault, so some matches may be missing; narrow it with project)")
	}

	return ToolResult{Text: strings.Join(lines, "\n")}, nil
}

// resolveScope resolves the project scope and vault of a search. Unlike
// the other memory tools, search_memory never falls back to the global
// config's last-used project: with neither project nor workspace_root,
// the scope stays empty (search everything) rather than narrowing to the
// project last used, and a workspace_root without a .sync82.json searches
// the whole vault. The returned context always carries the vault path; OK
// is true only when a project scope was resolved, and Problem is set when
// the workspace's .sync82.json cannot be read or names an invalid project.
func (t *SearchMemoryTool) resolveScope(args searchMemoryArgs) ResolvedContext {
	if strings.TrimSpace(args.Project) == "" && args.WorkspaceRoot != "" {
		rctx := t.Resolver.Resolve(ContextArgs{WorkspaceRoot: args.WorkspaceRoot, Subproject: args.Subproject, Path: args.Path, SearchParentDirs: args.SearchParentDirs})
		if !rctx.OK {
			rctx.DBPath = t.Resolver.DBPathOrDefault(args.Path)
		}
		return rctx
	}
	rctx := ResolvedContext{
		Project:    NormalizeName(args.Project),
		Subproject: NormalizeName(args.Subproject),
		DBPath:     t.Resolver.DBPathOrDefault(args.Path),
		Source:     SourceProvided,
	}
	rctx.OK = rctx.Project != ""
	return rctx
}

// searchReport is search_memory's JSON result. NextOffset, when non-zero,
// is the offset that continues the listing; ScanTruncated is set when the
// search stopped early on a very large vault.
type searchReport struct {
	Query         string      `json:"query"`
	Results       []searchHit `json:"results"`
	NextOffset    int         `json:"next_offset,omitempty"`
	ScanTruncated bool        `json:"scan_truncated,omitempty"`
}

// searchHit is one matching line in a searchReport, with its location and
// the surrounding context lines when requested.
type searchHit struct {
	Project       string   `json:"project"`
	Subproject    string   `json:"subproject,omitempty"`
	File          string   `json:"file"`
	EntryDate     string   `json:"entry_date,omitempty"`
	Line          int      `json:"line"`
	Text          string   `json:"text"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}
