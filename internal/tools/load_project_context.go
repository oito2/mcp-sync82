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
// filter never applies to one — see filteredContent. "mode" picks the
// defaults: "summary" (the default) keeps the most recent
// summaryMaxEntries dated entries per kind within summaryContextBytes,
// "full" keeps every entry within fullContextBytes. When dated entries are
// left out, a footer says how many per kind. The response is cut at
// max_bytes.
type LoadProjectContextTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// loadProjectContextArgs holds the decoded arguments of the
// load_project_context tool; its JSON tags match the property names declared
// in InputSchema.
type loadProjectContextArgs struct {
	targetArgs
	Files      []string `json:"files,omitempty"`
	Since      string   `json:"since,omitempty"`
	MaxEntries int      `json:"max_entries,omitempty"`
	MaxBytes   int      `json:"max_bytes,omitempty"`
	Mode       string   `json:"mode,omitempty"`
}

// The values of the "mode" argument.
const (
	contextModeSummary = "summary"
	contextModeFull    = "full"
)

// Size bounds, in bytes, of a load_project_context response: the default
// when max_bytes is omitted in each mode, and the minimum and maximum
// accepted.
const (
	summaryContextBytes = 40 << 10
	fullContextBytes    = 200 << 10
	minContextBytes     = 1 << 10
	maxContextBytes     = 50 << 20
)

// summaryMaxEntries is the number of most recent dated entries per
// entries-backed kind that summary mode keeps when neither since nor
// max_entries is given.
const summaryMaxEntries = 10

// currentStateKinds are the kinds placed first, in this order, so a
// size-bounded response keeps the project's current state before its
// history.
var currentStateKinds = []string{"memory", "architecture", "stack", "next_steps"}

// Name returns the MCP tool name, "load_project_context".
func (t *LoadProjectContextTool) Name() string { return "load_project_context" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *LoadProjectContextTool) Description() string {
	return `Load a project's memory (every non-blank file) concatenated into one context block, ready to paste into a new session. Overwrite-style files (memory, architecture, stack, next_steps) are always included in full, since they represent current state, not history, and come first. By default (mode "summary") only the 10 most recent dated entries of each log (progress, decisions, custom append kinds) are included and the response is cut at 40 KB; a footer says how many entries were left out. mode "full" includes every entry, cut at 200 KB. "since", "max_entries" and "max_bytes" override these defaults; the response is cut at max_bytes with a note saying so.`
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties files, since, max_entries and max_bytes plus
// the project-resolution properties.
func (t *LoadProjectContextTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{}, map[string]any{
			"files":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Only load these specific files/kinds, instead of everything."},
			"mode":        map[string]any{"type": "string", "enum": []string{contextModeSummary, contextModeFull}, "description": "\"summary\" (default): the 10 most recent dated entries per log, cut at 40 KB. \"full\": every entry, cut at 200 KB. since, max_entries and max_bytes override these defaults."},
			"since":       map[string]any{"type": "string", "description": "Only include dated entries (progress, decisions, custom append kinds) on or after this date (\"YYYY-MM-DD\"). Undated entries are always included. Overwrite-style files are unaffected. In summary mode, giving since lifts the default 10-entry limit."},
			"max_entries": map[string]any{"type": "integer", "minimum": 0, "description": "Only include the most recent N dated entries per append-only kind (summary mode default: 10). Overwrite-style files are unaffected."},
			"max_bytes":   map[string]any{"type": "integer", "minimum": minContextBytes, "maximum": maxContextBytes, "description": "Cut the response at this many bytes (default 40960, i.e. 40 KB, in summary mode; 204800, i.e. 200 KB, in full mode), with a note when it is cut."},
		}),
	}
}

// Validate decodes raw into loadProjectContextArgs. It lower-cases and
// checks each file name and the since date, and fills in the mode defaults:
// Mode defaults to summary; in summary mode MaxEntries defaults to
// summaryMaxEntries when neither since nor max_entries is given; MaxBytes
// defaults to summaryContextBytes or fullContextBytes by mode. It returns
// the arguments, or an error when a file name, since or mode is invalid,
// max_entries is negative or max_bytes is out of range.
func (t *LoadProjectContextTool) Validate(raw json.RawMessage) (any, error) {
	var args loadProjectContextArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	var problems []string
	for i, f := range args.Files {
		kind, err := validateKind(f)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		args.Files[i] = kind
	}
	if args.Since != "" {
		if _, err := time.Parse("2006-01-02", args.Since); err != nil {
			problems = append(problems, fmt.Sprintf(`"since" must be a valid date in the format "YYYY-MM-DD", got %q`, args.Since))
		}
	}
	if args.MaxEntries < 0 {
		problems = append(problems, `"max_entries" must be >= 0`)
	}
	switch args.Mode {
	case "":
		args.Mode = contextModeSummary
	case contextModeSummary, contextModeFull:
	default:
		problems = append(problems, fmt.Sprintf(`"mode" must be %q or %q, got %q`, contextModeSummary, contextModeFull, args.Mode))
	}
	if args.MaxBytes != 0 && (args.MaxBytes < minContextBytes || args.MaxBytes > maxContextBytes) {
		problems = append(problems, fmt.Sprintf(`"max_bytes" must be between %d and %d`, minContextBytes, maxContextBytes))
	}
	if err := problemsError(problems); err != nil {
		return nil, err
	}
	if args.Mode == contextModeSummary && args.Since == "" && args.MaxEntries == 0 {
		args.MaxEntries = summaryMaxEntries
	}
	if args.MaxBytes == 0 {
		args.MaxBytes = fullContextBytes
		if args.Mode == contextModeSummary {
			args.MaxBytes = summaryContextBytes
		}
	}
	return args, nil
}

// Execute resolves the target project and concatenates the non-blank content
// of its kinds (all of them, or only the requested files) into one block,
// current-state kinds first, applying since and max_entries to dated
// history. When dated entries are left out, a footer lists, per kind, how
// many were shown out of how many. The block is cut at MaxBytes, footer
// included. Store failures are returned as errors.
func (t *LoadProjectContextTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(loadProjectContextArgs)
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, args.contextArgs())
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	text, err := projectContext(ctx, s, rctx.Project, rctx.Subproject, args)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, rctx.Label())
	}
	return ToolResult{Text: text}, nil
}

// summaryContextArgs returns the arguments of a load_project_context call
// in summary mode with every default: the arguments Validate produces for
// a call that gives none.
func summaryContextArgs() loadProjectContextArgs {
	return loadProjectContextArgs{Mode: contextModeSummary, MaxEntries: summaryMaxEntries, MaxBytes: summaryContextBytes}
}

// projectContext builds the load_project_context block of project and
// subproject in s with the validated args (Files, Since, MaxEntries,
// MaxBytes): the non-blank kinds, current state first, the omitted-history
// footer and the size cut. Store failures are returned as errors, a
// missing project as one wrapping store.ErrNotFound.
func projectContext(ctx context.Context, s *store.Store, project, subproject string, args loadProjectContextArgs) (string, error) {
	label := FormatLabel(project, subproject)
	kinds, err := s.ListKinds(ctx, project, subproject, false)
	if err != nil {
		return "", err
	}

	// A requested file that doesn't exist is named in the response, so a
	// misspelled name doesn't read as a project without content.
	var unknown []string
	if len(args.Files) > 0 {
		wanted := make(map[string]bool, len(args.Files))
		for _, f := range args.Files {
			wanted[f] = true
		}
		filtered := make([]string, 0, len(kinds))
		for _, k := range kinds {
			if wanted[k] {
				filtered = append(filtered, k)
				delete(wanted, k)
			}
		}
		kinds = filtered
		for _, f := range args.Files {
			if wanted[f] {
				unknown = append(unknown, f)
				delete(wanted, f)
			}
		}
	}
	unknownNote := ""
	if len(unknown) > 0 {
		unknownNote = fmt.Sprintf("[no file named: %s]", namedKinds(unknown))
	}
	kinds = currentStateFirst(kinds)

	modes, err := s.KindModes(ctx, project, subproject)
	if err != nil {
		return "", err
	}
	var sections []contextSection
	var omitted []omittedHistory
	for _, k := range kinds {
		fc, err := filteredContent(ctx, s, project, subproject, k, modes[k], args.Since, args.MaxEntries)
		if err != nil {
			return "", err
		}
		if fc.filtered {
			stats, err := s.EntryStats(ctx, project, subproject, k)
			if err != nil {
				return "", err
			}
			if fc.shownDated < stats.Dated {
				omitted = append(omitted, omittedHistory{kind: k, shown: fc.shownDated, total: stats.Dated, oldestShown: fc.oldestShownDate})
			}
		}
		if !fc.ok {
			continue
		}
		trimmed := strings.TrimSpace(fc.content)
		if trimmed == "" {
			continue
		}
		sections = append(sections, contextSection{kind: k, text: "## " + k + "\n\n" + trimmed})
	}

	if len(sections) == 0 {
		if unknownNote != "" {
			return fmt.Sprintf("# Context: %s\n\n%s", label, unknownNote), nil
		}
		return fmt.Sprintf("# Context: %s\n\n(no content yet)", label), nil
	}
	footer := omittedFooter(omitted, args.MaxBytes)
	if unknownNote != "" {
		footer = strings.TrimSpace(unknownNote + "\n" + footer)
	}
	return buildContext(label, sections, footer, args.MaxBytes), nil
}

// contextSection is one kind's part of a load_project_context response:
// the kind name and its text, starting with its "## kind" heading.
type contextSection struct {
	kind string
	text string
}

// omittedHistory records that only shown of the total non-archived dated
// entries of kind were included. oldestShown is the date of the oldest
// dated entry included, or "" when none was.
type omittedHistory struct {
	kind        string
	shown       int
	total       int
	oldestShown string
}

// Bounds of the notes a load_project_context response may end with: the
// most kind names one note lists (the rest are counted), and the room, in
// bytes, kept for the truncation note when the sections are cut.
const (
	maxNamedKinds         = 10
	truncationNoteReserve = 384
)

// namedKinds returns kinds joined with ", ", naming at most maxNamedKinds
// of them and counting the rest ("… and 3 more").
func namedKinds(kinds []string) string {
	if len(kinds) <= maxNamedKinds {
		return strings.Join(kinds, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(kinds[:maxNamedKinds], ", "), len(kinds)-maxNamedKinds)
}

// omittedFooter returns the note listing the dated entries left out of the
// response, one clause per kind, or "" when nothing was left out. When
// that note would take more than a quarter of maxBytes, it returns a short
// form that only counts the kinds, so the footer never crowds out the
// content.
func omittedFooter(omitted []omittedHistory, maxBytes int) string {
	if len(omitted) == 0 {
		return ""
	}
	const howToLoad = `load more with "max_entries" or "since", or set "mode" to "full"`
	clauses := make([]string, len(omitted))
	for i, o := range omitted {
		clauses[i] = fmt.Sprintf("%s: %d of %d dated entries shown", o.kind, o.shown, o.total)
		if o.oldestShown != "" {
			clauses[i] += fmt.Sprintf(" (oldest shown %s)", o.oldestShown)
		}
	}
	footer := fmt.Sprintf("[older history omitted — %s; %s]", strings.Join(clauses, "; "), howToLoad)
	if len(footer) > maxBytes/4 {
		footer = fmt.Sprintf("[older history omitted in %d %s; %s]", len(omitted), pluralize(len(omitted), "file", "files"), howToLoad)
	}
	return footer
}

// buildContext joins the "# Context: label" header and sections into one
// block and appends footer, when not empty, after a separator. When that
// is larger than maxBytes, the sections are cut at a line break (see
// cutPoint) and followed by a truncation note naming the kinds cut short
// or left out entirely; room for the note (truncationNoteReserve) and for
// the footer is kept, so the whole response, notes included, fits in
// maxBytes. The footer is never cut.
func buildContext(label string, sections []contextSection, footer string, maxBytes int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Context: %s\n\n", label)
	starts := make([]int, len(sections))
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		starts[i] = b.Len()
		b.WriteString(sec.text)
	}
	text := b.String()

	suffix := ""
	if footer != "" {
		suffix = "\n\n---\n\n" + footer
	}
	if len(text)+len(suffix) <= maxBytes {
		return text + suffix
	}

	budget := max(maxBytes-len(suffix)-truncationNoteReserve-2, 0)
	cut := cutPoint(text, budget)
	var short, dropped []string
	for i, sec := range sections {
		end := starts[i] + len(sec.text)
		switch {
		case starts[i] >= cut:
			dropped = append(dropped, sec.kind)
		case end > cut:
			short = append(short, sec.kind)
		}
	}
	note := fmt.Sprintf("[context truncated at %d of %d bytes", cut, len(text))
	if len(short) > 0 {
		note += "; cut short: " + namedKinds(short)
	}
	if len(dropped) > 0 {
		note += "; left out: " + namedKinds(dropped)
	}
	note += ` — narrow it with "since", "max_entries" or "files", or raise "max_bytes"]`
	if len(note) > truncationNoteReserve {
		note = note[:cutPoint(note, truncationNoteReserve-len("…]"))] + "…]"
	}
	return text[:cut] + "\n\n" + note + suffix
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

// cutPoint returns where to cut text so the kept part fits in maxBytes:
// maxBytes itself when text is not longer; otherwise the last line break
// in the second half of the first maxBytes bytes, so a cut never drops
// more than half of the room to end on a whole line, or else a UTF-8
// character boundary.
func cutPoint(text string, maxBytes int) int {
	if len(text) <= maxBytes {
		return len(text)
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	if nl := strings.LastIndexByte(text[:cut], '\n'); nl > cut/2 {
		cut = nl
	}
	return cut
}

// kindContent is the result of filteredContent. content is the kind's
// text and ok whether the kind has any. filtered is true when the content
// came from entries narrowed by since or maxEntries; then shownDated is the
// number of dated entries kept and oldestShownDate the date of the oldest
// of them ("" when none).
type kindContent struct {
	content         string
	ok              bool
	filtered        bool
	shownDated      int
	oldestShownDate string
}

// filteredContent returns the content of kind in the given project and
// subproject of s, reading only the table mode (the kind's
// store.KindStorage) names. An overwrite-style document is returned in
// full, since it has no dated history. An entries-backed kind is returned
// whole when since and maxEntries are both zero, and otherwise narrowed
// through Store.ReadEntriesSince: since (a "YYYY-MM-DD" date) keeps dated
// entries on or after that date and always keeps undated ones, and
// maxEntries (when positive) keeps only the most recent N of what remains,
// in ascending date and insertion order. Store failures are returned as
// errors.
func filteredContent(ctx context.Context, s *store.Store, project, subproject, kind string, mode store.KindStorage, since string, maxEntries int) (kindContent, error) {
	switch mode {
	case store.KindStorageNone:
		return kindContent{}, nil
	case store.KindStorageDocument:
		content, ok, err := s.ReadDocument(ctx, project, subproject, kind)
		return kindContent{content: content, ok: ok}, err
	}
	if since == "" && maxEntries == 0 {
		content, ok, err := s.ReadContent(ctx, project, subproject, kind)
		return kindContent{content: content, ok: ok}, err
	}

	entries, err := s.ReadEntriesSince(ctx, project, subproject, kind, since, maxEntries)
	if err != nil {
		return kindContent{}, err
	}
	out := kindContent{filtered: true}
	if len(entries) == 0 {
		return out, nil
	}

	bodies := make([]string, len(entries))
	for i, e := range entries {
		bodies[i] = e.Body
		if e.EntryDate != "" {
			out.shownDated++
			if out.oldestShownDate == "" || e.EntryDate < out.oldestShownDate {
				out.oldestShownDate = e.EntryDate
			}
		}
	}
	out.content = strings.Join(bodies, "\n\n")
	out.ok = true
	return out, nil
}
