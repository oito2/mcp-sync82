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
	"regexp"
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// CheckProjectHealthTool implements check_project_health: report which of
// the six standard kinds exist for the resolved project. "Exists" means a
// documents row is present (even empty — it was written on purpose) or at
// least one non-archived entries row is present. It also reports warnings
// that never make the project unhealthy: a current-state document left
// behind by newer log entries, a standard document still empty or holding
// the blank template, undated log entries and a long active log.

type CheckProjectHealthTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// checkProjectHealthArgs holds the decoded arguments of the
// check_project_health tool; its JSON tags match the property names declared
// in InputSchema.
type checkProjectHealthArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
	Format           string `json:"format,omitempty"`
	StaleDays        int    `json:"stale_days,omitempty"`
}

// Health warning thresholds: the default number of days after which a
// current-state document older than the newest log entry counts as stale,
// and the number of active dated entries in one log above which archiving
// is suggested.
const (
	defaultStaleDays   = 30
	largeHistoryLength = 200
)

// healthNow returns the current time used to age documents.
var healthNow = time.Now

// The values of healthWarning.Check.
const (
	checkStale          = "stale"
	checkTemplate       = "template"
	checkUndatedEntries = "undated_entries"
	checkLargeHistory   = "large_history"
)

// healthWarning is one problem check_project_health reports without making
// the project unhealthy: the file it concerns, which check found it and a
// message telling the agent what to do.
type healthWarning struct {
	File    string `json:"file"`
	Check   string `json:"check"`
	Message string `json:"message"`
}

// Name returns the MCP tool name, "check_project_health".
func (t *CheckProjectHealthTool) Name() string { return "check_project_health" }

// Description returns the text shown to the calling agent that explains what
// the tool does and how to use it.
func (t *CheckProjectHealthTool) Description() string {
	return "Report which of the six standard memory files exist for a project, plus warnings about memory that may be out of date: a current-state file (memory, architecture, stack, next_steps) not updated for stale_days (default 30) while progress or decisions got newer entries, a standard file still empty or holding the blank template, undated progress/decisions entries, and a log with more than 200 active entries. Returns an error result (isError: true) when the project is unhealthy (a standard file is missing) — a deliberate signal for the calling agent to act on, not a crash; warnings alone never make it unhealthy."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional property format plus the project-resolution properties.
func (t *CheckProjectHealthTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project":            map[string]any{"type": "string", "description": "Project name. If omitted, auto-discovered from workspace_root or the last used project."},
			"subproject":         map[string]any{"type": "string", "description": "Subproject name."},
			"workspace_root":     map[string]any{"type": "string", "description": "Path to your project folder, used to auto-discover the project via .sync82.json."},
			"search_parent_dirs": map[string]any{"type": "boolean", "description": SearchParentDirsDescription},
			"path":               map[string]any{"type": "string", "description": PathDescription},
			"format":             formatProperty(),
			"stale_days":         map[string]any{"type": "integer", "minimum": 1, "description": "Days after which a current-state file older than the newest progress/decisions entry is reported as possibly out of date (default 30)."},
		},
	}
}

// Validate decodes raw into checkProjectHealthArgs, normalizes format to
// "text" or "json" and defaults StaleDays to defaultStaleDays. It returns
// the arguments, or an error when format is invalid or stale_days is
// negative.
func (t *CheckProjectHealthTool) Validate(raw json.RawMessage) (any, error) {
	var args checkProjectHealthArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	format, err := validateFormat(args.Format)
	if err != nil {
		return nil, err
	}
	args.Format = format
	if args.StaleDays < 0 {
		return nil, fmt.Errorf(`"stale_days" must be >= 1`)
	}
	if args.StaleDays == 0 {
		args.StaleDays = defaultStaleDays
	}
	return args, nil
}

// Execute resolves the target project, checks that each standard kind
// exists and collects the healthWarnings. The result is a text or JSON
// report; its IsError flag is set when any standard kind is missing, never
// for warnings alone. Store failures are returned as errors.
func (t *CheckProjectHealthTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(checkProjectHealthArgs)
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

	type kindStatus struct {
		kind   string
		exists bool
	}
	statuses := make([]kindStatus, 0, len(standardKinds))
	isHealthy := true
	for _, k := range standardKinds {
		exists, err := s.KindExists(ctx, rctx.Project, rctx.Subproject, k)
		if err != nil {
			return ToolResult{}, wrapNotFound(err, rctx.Label())
		}
		statuses = append(statuses, kindStatus{kind: k, exists: exists})
		if !exists {
			isHealthy = false
		}
	}

	warnings, err := healthWarnings(ctx, s, rctx, args.StaleDays)
	if err != nil {
		return ToolResult{}, err
	}

	if args.Format == "json" {
		report := healthReport{Project: rctx.Label(), Vault: rctx.DBPath, Healthy: isHealthy, Files: map[string]bool{}, Warnings: warnings}
		for _, st := range statuses {
			report.Files[st.kind] = st.exists
		}
		return jsonResult(report, !isHealthy)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Health report for project: %s%s\n", rctx.Label(), ContextNote(rctx))
	if isHealthy {
		b.WriteString("Status: HEALTHY ✅\n\nFiles:\n")
	} else {
		b.WriteString("Status: UNHEALTHY ❌\n\nFiles:\n")
	}
	for _, st := range statuses {
		status := "MISSING"
		if st.exists {
			status = "OK"
		}
		fmt.Fprintf(&b, "- %s: %s\n", st.kind, status)
	}
	if !isHealthy {
		b.WriteString("\nRecommendation: Use create_project or init_project_memory to restore missing files.")
	}
	if len(warnings) > 0 {
		if !isHealthy {
			b.WriteString("\n")
		}
		b.WriteString("\nWarnings:\n")
		for _, w := range warnings {
			fmt.Fprintf(&b, "- %s: %s\n", w.File, w.Message)
		}
	}

	return ToolResult{Text: b.String(), IsError: !isHealthy}, nil
}

// healthWarnings returns the warnings for the project of rctx in s, current
// state files first, in standard kind order. A current-state document still
// empty or equal to its blank template gets a template warning; otherwise it
// gets a stale warning when it was last updated more than staleDays days ago
// and progress or decisions has a dated entry after that day. The two logs
// get an undated_entries warning for undated entries other than a preamble
// and a large_history warning above largeHistoryLength active dated
// entries. Store failures are returned as errors.
func healthWarnings(ctx context.Context, s *store.Store, rctx ResolvedContext, staleDays int) ([]healthWarning, error) {
	var warnings []healthWarning
	newestEntry := ""
	logStats := map[string]store.EntryStats{}
	for _, k := range []string{"progress", "decisions"} {
		st, err := s.EntryStats(ctx, rctx.Project, rctx.Subproject, k)
		if err != nil {
			return nil, err
		}
		logStats[k] = st
		if st.LatestDate > newestEntry {
			newestEntry = st.LatestDate
		}
	}

	today := healthNow().UTC().Truncate(24 * time.Hour)
	for _, k := range currentStateKinds {
		content, ok, err := s.ReadDocument(ctx, rctx.Project, rctx.Subproject, k)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if isUnfilledTemplate(k, content, rctx.Label()) {
			warnings = append(warnings, healthWarning{File: k, Check: checkTemplate,
				Message: "still empty or holding the blank template; fill it in with write_memory"})
			continue
		}
		meta, err := s.Metadata(ctx, rctx.Project, rctx.Subproject, k)
		if err != nil {
			return nil, err
		}
		updated, err := time.Parse("2006-01-02", meta.LastModified)
		if err != nil {
			continue
		}
		age := int(today.Sub(updated).Hours() / 24)
		if age > staleDays && newestEntry > meta.LastModified {
			warnings = append(warnings, healthWarning{File: k, Check: checkStale, Message: fmt.Sprintf(
				"last updated %s (%d days ago), but progress/decisions have entries up to %s; check that it still matches them and update it with write_memory",
				meta.LastModified, age, newestEntry)})
		}
	}

	for _, k := range []string{"progress", "decisions"} {
		st := logStats[k]
		if st.Undated > 0 {
			warnings = append(warnings, healthWarning{File: k, Check: checkUndatedEntries, Message: fmt.Sprintf(
				"%d undated %s; archive_memory never archives them — add a \"## YYYY-MM-DD\" header with edit_entry",
				st.Undated, pluralize(st.Undated, "entry", "entries"))})
		}
		if st.Dated > largeHistoryLength {
			warnings = append(warnings, healthWarning{File: k, Check: checkLargeHistory, Message: fmt.Sprintf(
				"%d active dated entries; archive the old ones with archive_memory to keep the loaded context small",
				st.Dated)})
		}
	}
	return warnings, nil
}

// lastUpdatedLine matches the "- Last updated: YYYY-MM-DD" line of the
// memory template, whose date changes with the day it is rendered.
var lastUpdatedLine = regexp.MustCompile(`(?m)^- Last updated: \d{4}-\d{2}-\d{2}[ \t]*$`)

// isUnfilledTemplate reports whether content of the standard kind is blank
// or equal, apart from surrounding white space and the date of the
// lastUpdatedLine, to the template init_project_memory renders for the
// project called label when no answer is given.
func isUnfilledTemplate(kind, content, label string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}
	for _, tpl := range standardDocumentTemplates {
		if tpl.kind == kind {
			blank := strings.TrimSpace(tpl.render(initAnswers{}, label))
			strip := func(text string) string { return lastUpdatedLine.ReplaceAllString(text, "- Last updated:") }
			return strip(content) == strip(blank)
		}
	}
	return false
}

// healthReport is check_project_health's JSON result: the project label,
// the vault path, the overall verdict, whether each standard kind exists
// and the warnings, omitted when there are none.
type healthReport struct {
	Project  string          `json:"project"`
	Vault    string          `json:"vault"`
	Healthy  bool            `json:"healthy"`
	Files    map[string]bool `json:"files"`
	Warnings []healthWarning `json:"warnings,omitempty"`
}
