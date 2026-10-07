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
	"time"
	"unicode/utf8"

	"github.com/oito2/mcp-sync82/internal/store"
)

// ArchiveMemoryTool implements archive_memory: marks old dated entries in
// progress or decisions as archived through store.ArchiveEntries,
// optionally adding a summary of them as a new entry in the same
// transaction, or only lists them on a dry run. Entries with no date header
// are never archived, regardless of age.
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
	Summary          string `json:"summary,omitempty"`
	DryRun           bool   `json:"dry_run,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// maxKeepDays is the largest accepted keep_days (100 years). It keeps the
// cutoff date computation within a valid range.
const maxKeepDays = 36500

// Bounds of a dry-run listing: the most entries listed, and the most
// characters of each entry's first line.
const (
	maxDryRunEntries   = 200
	maxDryRunLineRunes = 100
)

// Name returns the MCP tool name, "archive_memory".
func (t *ArchiveMemoryTool) Name() string { return "archive_memory" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *ArchiveMemoryTool) Description() string {
	return `Archive old dated entries from progress.md or decisions.md, keeping only the last N days active (counted in UTC). Entries without a date header are never archived. Archived entries leave the loaded context; to keep what they said, first call with dry_run: true (lists the entries that would be archived, writes nothing), read them with read_memory, then call again with summary: the summary is added as a new active entry in the same step, dated today (UTC) and headed "## YYYY-MM-DD — Summary of N archived entries (from … to …)" unless it has its own "## YYYY-MM-DD" header, which must not be older than the archive cutoff. The project must be given via project or workspace_root, not taken from the last session.`
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
			"summary":            map[string]any{"type": "string", "description": "A summary of the entries being archived, written by you, added as one new active entry in the same step. Without its own \"## YYYY-MM-DD\" header it gets one dated today; its own header must not be older than the archive cutoff (today minus keep_days). Not written when nothing is archived."},
			"dry_run":            map[string]any{"type": "boolean", "description": "List the entries that would be archived (date and first line) without changing anything."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
		},
		"required": []string{"filename"},
	}
}

// Validate decodes raw into archiveMemoryArgs. It returns the arguments,
// with Filename trimmed and lower-cased, KeepDays defaulting to 90 and
// entry id marker lines removed from Summary, or an error when filename is
// not an append-only standard kind, keep_days is outside 1 to maxKeepDays,
// or summary is given but blank, larger than maxContentSize or dated by a
// header older than the archive cutoff, which the next archive would
// archive again.
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
	if args.Summary != "" {
		args.Summary = stripEntryMarkers(args.Summary)
		if strings.TrimSpace(args.Summary) == "" {
			return nil, fmt.Errorf(`"summary" must not be blank`)
		}
		if len(args.Summary) > maxContentSize {
			return nil, fmt.Errorf(`"summary" exceeds the %d byte limit`, maxContentSize)
		}
		if date, cutoff := extractFirstDate(args.Summary), archiveCutoff(args.KeepDays); date != "" && date < cutoff {
			return nil, fmt.Errorf(`the "summary" header date %s is older than the archive cutoff %s, so the next archive would archive it too; date it %s or later, or leave the header out to date it today`, date, cutoff, cutoff)
		}
	}
	return args, nil
}

// archiveCutoff returns the archive cutoff date for keepDays: today (UTC)
// minus keepDays, as "YYYY-MM-DD". Dated entries before it are archived.
func archiveCutoff(keepDays int) string {
	return time.Now().UTC().AddDate(0, 0, -keepDays).Format("2006-01-02")
}

// Execute resolves the target project and archives the dated entries older
// than KeepDays days (UTC), adding Summary as a new entry when one is
// given, then reports how many were archived and kept. With DryRun it only
// lists the entries it would archive. It refuses, with an error result, to
// act on a project that was only taken from the last session; store
// failures are returned as errors.
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

	cutoff := archiveCutoff(args.KeepDays)
	label := rctx.Label()
	if args.DryRun {
		entries, err := s.ListArchivable(ctx, rctx.Project, rctx.Subproject, args.Filename, cutoff)
		if err != nil {
			return ToolResult{}, wrapNotFound(err, label)
		}
		return ToolResult{Text: formatArchiveDryRun(entries, label+"/"+args.Filename+ContextNote(rctx), args.KeepDays, args.Summary != "")}, nil
	}

	var summary store.ArchiveSummary
	if args.Summary != "" {
		summary = func(archived store.ArchiveResult) (string, string, error) {
			date, body := summaryEntry(args.Summary, archived, time.Now().UTC().Format("2006-01-02"))
			return date, body, nil
		}
	}
	result, err := s.ArchiveEntries(ctx, rctx.Project, rctx.Subproject, args.Filename, cutoff, summary)
	if err != nil {
		return ToolResult{}, wrapNotFound(err, label)
	}

	if result.Archived == 0 {
		plural := pluralize(result.Kept, "entry", "entries")
		text := fmt.Sprintf(
			"Nothing to archive in %s/%s: all %d %s are within the last %d days.",
			label, args.Filename, result.Kept, plural, args.KeepDays,
		)
		if args.Summary != "" {
			text += " The summary was not written."
		}
		return ToolResult{Text: text}, nil
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
	if result.SummaryID != 0 {
		text += fmt.Sprintf("\n  → summary added as entry %d", result.SummaryID)
	}
	return ToolResult{Text: text}, nil
}

// summaryEntry returns the date and body of the entry that holds summary
// for the archived range: summary as given, dated by its first
// "## YYYY-MM-DD" header, or, when it has none, summary under a generated
// header dated today that names the archived range. A summary dated by the
// archived range itself would be older than the cutoff and archived by the
// next call.
func summaryEntry(summary string, archived store.ArchiveResult, today string) (date, body string) {
	if date := extractFirstDate(summary); date != "" {
		return date, summary
	}
	header := fmt.Sprintf("## %s — Summary of %d archived %s (from %s to %s)",
		today, archived.Archived, pluralize(archived.Archived, "entry", "entries"),
		archived.OldestDate, archived.NewestDate)
	return today, header + "\n" + strings.TrimLeft(summary, "\r\n")
}

// formatArchiveDryRun returns the dry-run report of archive_memory for
// target (label/file plus context note): the entries that would be
// archived, at most maxDryRunEntries of them, each as its date and
// entryPreview, and, when withSummary is set, where the summary would go.
func formatArchiveDryRun(entries []store.Entry, target string, keepDays int, withSummary bool) string {
	if len(entries) == 0 {
		text := fmt.Sprintf("Dry run — nothing would be archived in %s: no dated entry is older than %d days.", target, keepDays)
		if withSummary {
			text += " The summary would not be written."
		}
		return text
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Dry run — nothing was changed. Would archive %d %s from %s (from %s to %s):\n",
		len(entries), pluralize(len(entries), "entry", "entries"), target,
		entries[0].EntryDate, entries[len(entries)-1].EntryDate)
	for i, e := range entries {
		if i == maxDryRunEntries {
			fmt.Fprintf(&b, "- … and %d more; read them with read_memory\n", len(entries)-i)
			break
		}
		fmt.Fprintf(&b, "- [%s] entry %d: %s\n", e.EntryDate, e.ID, entryPreview(e.Body))
	}
	if withSummary {
		b.WriteString("The summary would be added as a new active entry dated today, unless it has its own date header.")
	} else {
		b.WriteString("To keep what they say, read them with read_memory and pass a summary when archiving.")
	}
	return b.String()
}

// entryPreview returns the first non-blank line of body that is not a
// "## YYYY-MM-DD" header, trimmed and cut to maxDryRunLineRunes characters,
// or "(empty)" when there is none.
func entryPreview(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || dateHeaderPattern.MatchString(line) {
			continue
		}
		if utf8.RuneCountInString(line) > maxDryRunLineRunes {
			line = string([]rune(line)[:maxDryRunLineRunes]) + "…"
		}
		return line
	}
	return "(empty)"
}
