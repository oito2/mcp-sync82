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
	"unicode/utf8"

	"github.com/oito2/mcp-sync82/internal/store"
)

// SearchMemoryTool implements search_memory: full-text search by words or
// phrase (case and Latin accents ignored, ranked by relevance), or a
// literal case-insensitive substring search in exact mode. Scope rule: no
// project → search everything; project only → that project's own
// documents/entries plus all its subprojects; project + subproject → that
// subproject only.
type SearchMemoryTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// searchMemoryArgs holds the decoded arguments of the search_memory tool;
// its JSON tags match the property names declared in InputSchema.
type searchMemoryArgs struct {
	targetArgs
	Query        string   `json:"query"`
	Match        string   `json:"match,omitempty"`
	Kinds        []string `json:"kinds,omitempty"`
	Since        string   `json:"since,omitempty"`
	Until        string   `json:"until,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Offset       int      `json:"offset,omitempty"`
	ContextLines int      `json:"context_lines,omitempty"`
	Format       string   `json:"format,omitempty"`
}

// Bounds on a search's output: the maximum context lines per match, the
// maximum size in bytes of the whole response as sent (its text and its
// structured content, both JSON-encoded), and the maximum size of one
// matching or context line, so a single huge line (content may be up to
// maxContentSize) cannot exceed the response size on its own.
const (
	maxSearchContextLines = 20
	maxSearchResponseSize = 1 << 20
	maxSearchLineBytes    = 4 << 10
)

// clipLine returns line unchanged when it fits in maxSearchLineBytes;
// otherwise it cuts it at a UTF-8 character boundary within that size and
// appends "…".
func clipLine(line string) string {
	if len(line) <= maxSearchLineBytes {
		return line
	}
	cut := maxSearchLineBytes
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return line[:cut] + "…"
}

// clipLines returns lines with clipLine applied to each, or nil for none.
func clipLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = clipLine(l)
	}
	return out
}

// Name returns the MCP tool name, "search_memory".
func (t *SearchMemoryTool) Name() string { return "search_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *SearchMemoryTool) Description() string {
	return "Search memory files. By default (match \"words\") finds the documents and entries holding every word of the query, in any order, ignoring case and accents (\"decisao\" finds \"Decisão\"), best matches first; a word ending in \"*\" matches as a prefix (\"instal*\"). match \"phrase\" needs the words in that order; match \"exact\" finds a literal case-insensitive substring, for paths or identifiers with punctuation. kinds limits the search to some files; since/until to dated entries in a date range. No project given searches the whole vault; project only searches that project and all its subprojects; project+subproject searches just that subproject. Each match is labeled project/file:line, or project/file[YYYY-MM-DD]:line for an entry of a dated log (line counted within that entry)."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the required property query, the optional properties limit, offset,
// context_lines and format, and the project-resolution properties.
func (t *SearchMemoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{
			Project:       "Limit the search to this project (and its subprojects, unless subproject is also given).",
			Subproject:    "Limit the search to this specific subproject (requires project or workspace_root).",
			WorkspaceRoot: "Path to your project folder, used to auto-discover the project via .sync82.json when project isn't given.",
		}, map[string]any{
			"query":         map[string]any{"type": "string", "description": "What to search for, on a single line: words (match \"words\", the default), a phrase (match \"phrase\") or a literal substring (match \"exact\")."},
			"match":         map[string]any{"type": "string", "enum": []string{string(store.SearchWords), string(store.SearchPhrase), string(store.SearchExact)}, "description": "\"words\" (default): every word, any order, accents and case ignored, ranked by relevance, \"word*\" for a prefix. \"phrase\": the words in that order. \"exact\": a literal case-insensitive substring, in file order."},
			"kinds":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Only search these files/kinds (e.g. [\"decisions\"])."},
			"since":         map[string]any{"type": "string", "description": "Only search dated entries on or after this date (\"YYYY-MM-DD\"); documents and undated entries are left out."},
			"until":         map[string]any{"type": "string", "description": "Only search dated entries on or before this date (\"YYYY-MM-DD\"); documents and undated entries are left out."},
			"limit":         map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "description": "Maximum number of results to return (1-1000, default 100)."},
			"offset":        map[string]any{"type": "integer", "minimum": 0, "description": "Number of results to skip, for pagination (default 0)."},
			"context_lines": map[string]any{"type": "integer", "minimum": 0, "maximum": maxSearchContextLines, "description": "Number of surrounding lines to include per match (0-20, default 0)."},
			"format":        formatProperty(),
		}),
		"required": []string{"query"},
	}
}

// Validate decodes raw into searchMemoryArgs and applies the defaults (Limit
// 100, words match, text format), lower-casing the kinds. It returns the
// arguments, or an error listing every problem: an empty or multi-line
// query, an invalid project or subproject name, a words or phrase query
// with no letter or number, an unknown match, an invalid kind, since or
// until date, since after until, a subproject without a project or
// workspace_root, or an out-of-range limit, offset or context_lines.
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
	switch store.SearchMode(args.Match) {
	case "":
		args.Match = string(store.SearchWords)
	case store.SearchWords, store.SearchPhrase, store.SearchExact:
	default:
		problems = append(problems, fmt.Sprintf(`"match" must be %q, %q or %q, got %q`, store.SearchWords, store.SearchPhrase, store.SearchExact, args.Match))
	}
	if args.Query != "" && args.Match != string(store.SearchExact) && !store.HasSearchTerms(args.Query) {
		problems = append(problems, `"query" has no letter or number to search for; use match "exact" to search for punctuation`)
	}
	for i, k := range args.Kinds {
		kind, err := validateKind(k)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		args.Kinds[i] = kind
	}
	for _, d := range []struct{ name, value string }{{"since", args.Since}, {"until", args.Until}} {
		if d.value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d.value); err != nil {
			problems = append(problems, fmt.Sprintf(`%q must be a valid date in the format "YYYY-MM-DD", got %q`, d.name, d.value))
		}
	}
	if args.Since != "" && args.Until != "" && args.Since > args.Until {
		problems = append(problems, `"since" must not be after "until"`)
	}
	for _, n := range []struct{ field, name string }{{"project", args.Project}, {"subproject", args.Subproject}} {
		if name := NormalizeName(n.name); name != "" {
			if err := validateProjectName(n.field, name); err != nil {
				problems = append(problems, err.Error())
			}
		}
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
// or JSON; the report is also returned as structured content. The output is
// also capped in size, with the next offset reported when it is cut. A
// missing vault, an unusable .sync82.json or a subproject given without a
// resolvable project yields an error result; a missing project and other
// store failures are returned as errors.
func (t *SearchMemoryTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(searchMemoryArgs)
	rctx := t.Resolver.resolveSearchScope(args.contextArgs())
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

	results, info, err := s.SearchText(ctx, store.SearchOptions{
		Query: args.Query, Mode: store.SearchMode(args.Match),
		Scope: store.SearchScope{Project: project, Subproject: subproject},
		Kinds: args.Kinds, Since: args.Since, Until: args.Until,
		Offset: args.Offset, Limit: args.Limit, ContextLines: args.ContextLines,
	})
	if err != nil {
		return ToolResult{}, wrapNotFound(err, FormatLabel(project, subproject))
	}
	scanTruncated := info.Truncated

	moreResults := len(results) > args.Limit
	if moreResults {
		results = results[:args.Limit]
	}
	if len(results) == 0 && args.Format != "json" {
		text := fmt.Sprintf("No results for %q", args.Query)
		if args.Offset > 0 {
			// Past the last page, not "nothing matches".
			text = fmt.Sprintf("No more results for %q at offset %d", args.Query, args.Offset)
		}
		if scanTruncated {
			text += " (the search stopped early on a very large vault; narrow it with project)"
		}
		return ToolResult{Text: text, Structured: searchReport{Query: args.Query, Results: []searchHit{}, ScanTruncated: scanTruncated, SubstringFallback: info.Substring}}, nil
	}

	report := searchReport{Query: args.Query, Results: []searchHit{}, ScanTruncated: scanTruncated, SubstringFallback: info.Substring}
	lines := make([]string, 0, len(results)+2)
	size := 0
	for i, r := range results {
		r.Line, r.ContextBefore, r.ContextAfter = clipLine(r.Line), clipLines(r.ContextBefore), clipLines(r.ContextAfter)
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
		hit := searchHit{
			Project: r.Project, Subproject: r.Subproject, File: r.Kind, EntryID: r.EntryID, EntryDate: r.EntryDate,
			Line: r.LineNumber, Text: r.Line, ContextBefore: r.ContextBefore, ContextAfter: r.ContextAfter,
		}
		cost, err := searchHitCost(hit, line, args.Format == "json")
		if err != nil {
			return ToolResult{}, err
		}
		if size+cost > maxSearchResponseSize && i > 0 {
			lines = append(lines, fmt.Sprintf("(output truncated after %d results; use offset %d to continue, or fewer context_lines)", i, args.Offset+i))
			report.NextOffset = args.Offset + i
			moreResults = false
			break
		}
		size += cost
		lines = append(lines, line)
		report.Results = append(report.Results, hit)
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
	if info.Substring {
		lines = append(lines, "(the full-text index found no match, so the words were matched inside the text instead; results are in reading order, not by relevance)")
	}

	return ToolResult{Text: strings.Join(lines, "\n"), Structured: report}, nil
}

// OutputSchema returns the JSON Schema of searchReport, the structured
// content of every successful search_memory result.
func (t *SearchMemoryTool) OutputSchema() map[string]any {
	return schemaObject(map[string]any{
		"query": schemaString(),
		"results": schemaArray(schemaObject(map[string]any{
			"project":        schemaString(),
			"subproject":     schemaString(),
			"file":           schemaString(),
			"entry_id":       schemaInteger(),
			"entry_date":     schemaString(),
			"line":           schemaInteger(),
			"text":           schemaString(),
			"context_before": schemaArray(schemaString()),
			"context_after":  schemaArray(schemaString()),
		}, "project", "file", "line", "text")),
		"next_offset":        schemaInteger(),
		"scan_truncated":     schemaBoolean(),
		"substring_fallback": schemaBoolean(),
	}, "query", "results")
}

// searchReport is search_memory's JSON result. NextOffset, when non-zero,
// is the offset that continues the listing; ScanTruncated is set when the
// search stopped early on a very large vault.
type searchReport struct {
	Query         string      `json:"query"`
	Results       []searchHit `json:"results"`
	NextOffset    int         `json:"next_offset,omitempty"`
	ScanTruncated bool        `json:"scan_truncated,omitempty"`
	// SubstringFallback reports that the full-text index found no match
	// and the words were matched as substrings, in reading order.
	SubstringFallback bool `json:"substring_fallback,omitempty"`
}

// searchHit is one matching line in a searchReport, with its location and
// the surrounding context lines when requested.
type searchHit struct {
	Project       string   `json:"project"`
	Subproject    string   `json:"subproject,omitempty"`
	File          string   `json:"file"`
	EntryID       int64    `json:"entry_id,omitempty"`
	EntryDate     string   `json:"entry_date,omitempty"`
	Line          int      `json:"line"`
	Text          string   `json:"text"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

// searchHitCost returns the bytes hit adds to a search response as it is
// sent: the hit in the structured content, plus its part of the text — line
// in text format, or the hit's indented JSON in JSON format — encoded as a
// JSON string. JSON escaping counts, since "<", ">" and "&" take six bytes
// each.
func searchHitCost(hit searchHit, line string, jsonFormat bool) (int, error) {
	structured, err := json.Marshal(hit)
	if err != nil {
		return 0, fmt.Errorf("encode search result: %w", err)
	}
	text := line
	if jsonFormat {
		indented, err := json.MarshalIndent(hit, "    ", "  ")
		if err != nil {
			return 0, fmt.Errorf("encode search result: %w", err)
		}
		text = string(indented)
	}
	encodedText, err := json.Marshal(text)
	if err != nil {
		return 0, fmt.Errorf("encode search result: %w", err)
	}
	return len(structured) + len(encodedText) + 8, nil
}
