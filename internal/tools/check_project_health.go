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
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/store"
)

// CheckProjectHealthTool implements check_project_health: report which of
// the six standard kinds exist for the resolved project. "Exists" means a
// documents row is present (even empty — it was written on purpose) or at
// least one entries row is present, archived or not. A missing
// current-state document makes the project unhealthy; a log (progress,
// decisions) with no entry yet only gets an empty_log warning. It also
// reports warnings that never make the project unhealthy: a current-state
// document left behind by newer log entries, a standard document still empty
// or holding the blank template, undated log entries and a long active log.
type CheckProjectHealthTool struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// checkProjectHealthArgs holds the decoded arguments of the
// check_project_health tool; its JSON tags match the property names declared
// in InputSchema.
type checkProjectHealthArgs struct {
	targetArgs
	AllProjects bool   `json:"all_projects,omitempty"`
	Format      string `json:"format,omitempty"`
	StaleDays   int    `json:"stale_days,omitempty"`
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
	checkEmptyLog       = "empty_log"
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
	return "Report which of the six standard memory files exist for a project. A missing current-state file (memory, architecture, stack, next_steps) makes the project unhealthy; progress or decisions with no entry yet only gets a warning. Other warnings point at memory that may be out of date: a current-state file (memory, architecture, stack, next_steps) not updated for stale_days (default 30) while progress or decisions got newer entries, a standard file still empty or holding the blank template, undated progress/decisions entries, and a log with more than 200 active entries. With all_projects: true it checks every project and subproject of the vault instead, one line each, then the details of the ones that are not healthy. Returns an error result (isError: true) when the project (or any project) is unhealthy (a current-state file is missing) — a deliberate signal for the calling agent to act on, not a crash; warnings alone never make it unhealthy."
}

// InputSchema returns the JSON Schema of the tool's arguments: an object
// with the optional properties all_projects, format and stale_days plus the
// project-resolution properties.
func (t *CheckProjectHealthTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": targetProperties(targetSchema{}, map[string]any{
			"all_projects": map[string]any{"type": "boolean", "description": "Check every project and subproject of the vault (the one named by path, or the default vault) instead of one project. Can't be combined with project, subproject or workspace_root."},
			"format":       formatProperty(),
			"stale_days":   map[string]any{"type": "integer", "minimum": 1, "description": "Days after which a current-state file older than the newest progress/decisions entry is reported as possibly out of date (default 30)."},
		}),
	}
}

// Validate decodes raw into checkProjectHealthArgs, normalizes format to
// "text" or "json" and defaults StaleDays to defaultStaleDays. It returns
// the arguments, or an error when format is invalid, stale_days is
// negative, or all_projects is combined with project, subproject or
// workspace_root.
func (t *CheckProjectHealthTool) Validate(raw json.RawMessage) (any, error) {
	var args checkProjectHealthArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	var problems []string
	format, err := validateFormat(args.Format)
	if err != nil {
		problems = append(problems, err.Error())
	}
	args.Format = format
	if args.StaleDays < 0 {
		problems = append(problems, `"stale_days" must be >= 1`)
	}
	if args.StaleDays == 0 {
		args.StaleDays = defaultStaleDays
	}
	if args.AllProjects {
		for _, f := range []struct{ name, value string }{{"project", args.Project}, {"subproject", args.Subproject}, {"workspace_root", args.WorkspaceRoot}} {
			if strings.TrimSpace(f.value) != "" {
				problems = append(problems, fmt.Sprintf(`"all_projects" can't be combined with %q`, f.name))
			}
		}
	}
	if err := problemsError(problems); err != nil {
		return nil, err
	}
	return args, nil
}

// Execute checks the target project, or with AllProjects every project of
// the vault, and returns a text or JSON report; in both formats the report
// is also the structured content. Its IsError flag is set when any checked
// project misses a standard kind, never for warnings alone. An unresolved
// project returns the instructional result; a project that does not exist
// yields an error wrapping store.ErrNotFound, and other store failures are
// returned as errors.
func (t *CheckProjectHealthTool) Execute(ctx context.Context, rawArgs any) (ToolResult, error) {
	args := rawArgs.(checkProjectHealthArgs)
	if args.AllProjects {
		return t.executeVault(ctx, args)
	}
	s, rctx, ready, err := t.Resolver.ResolveStore(ctx, t.Stores, args.contextArgs())
	if ready != nil {
		return *ready, nil
	}
	if err != nil {
		return ToolResult{}, err
	}

	h, err := checkHealth(ctx, s, rctx.Project, rctx.Subproject, args.StaleDays)
	if err != nil {
		return ToolResult{}, err
	}
	report := healthReport{Project: rctx.Label(), Vault: rctx.DBPath, Healthy: h.Healthy, Files: map[string]bool{}, Warnings: h.Warnings}
	for _, k := range standardKinds {
		report.Files[k] = h.Exists[k]
	}
	if args.Format == "json" {
		return jsonResult(report, !h.Healthy)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Health report for project: %s%s\n", rctx.Label(), ContextNote(rctx))
	if h.Healthy {
		b.WriteString("Status: HEALTHY ✅\n\nFiles:\n")
	} else {
		b.WriteString("Status: UNHEALTHY ❌\n\nFiles:\n")
	}
	for _, k := range standardKinds {
		status := "OK"
		switch {
		case report.Files[k]:
		case slices.Contains(h.Missing, k):
			status = "MISSING"
		default:
			status = "EMPTY (no entry yet)"
		}
		fmt.Fprintf(&b, "- %s: %s\n", k, status)
	}
	if !h.Healthy {
		b.WriteString("\n" + missingFilesAdvice)
	}
	if len(h.Warnings) > 0 {
		if !h.Healthy {
			b.WriteString("\n")
		}
		b.WriteString("\nWarnings:\n")
		for _, w := range h.Warnings {
			fmt.Fprintf(&b, "- %s: %s\n", w.File, w.Message)
		}
	}

	return ToolResult{Text: b.String(), IsError: !h.Healthy, Structured: report}, nil
}

// executeVault checks every project and subproject of the vault named by
// args.Path (or the default vault), never the last-used project, and
// returns VaultHealthText or the JSON report, with the report as the
// structured content. A missing vault is an error result.
func (t *CheckProjectHealthTool) executeVault(ctx context.Context, args checkProjectHealthArgs) (ToolResult, error) {
	dbPath := t.Resolver.DBPathOrDefault(args.Path)
	s, err := t.Stores.GetExisting(ctx, dbPath)
	if errors.Is(err, store.ErrVaultNotFound) {
		return vaultMissingResult(dbPath), nil
	}
	if err != nil {
		return ToolResult{}, err
	}
	report, err := VaultHealth(ctx, s, dbPath, args.StaleDays)
	if err != nil {
		return ToolResult{}, err
	}
	if args.Format == "json" {
		return jsonResult(report, !report.Healthy)
	}
	return ToolResult{Text: VaultHealthText(report), IsError: !report.Healthy, Structured: report}, nil
}

// projectHealth is the outcome of checking one project: whether it is
// healthy, the current-state kinds it misses, in standard kind order,
// whether each standard kind exists, and its warnings.
type projectHealth struct {
	Healthy  bool
	Missing  []string
	Exists   map[string]bool
	Warnings []healthWarning
}

// missingFilesAdvice is the recommendation printed when a current-state
// file is missing: init_project_memory writes the ones that are missing
// and leaves the others as they are.
const missingFilesAdvice = "Recommendation: Use init_project_memory to write the missing files; it leaves the existing ones as they are."

// checkHealth checks the project or subproject named by project and
// subproject in s: every current-state kind must exist, a log with no
// entry yet gets an empty_log warning, and healthWarnings lists what may be
// out of date. It returns an error wrapping store.ErrNotFound when the
// project does not exist, or a store error.
func checkHealth(ctx context.Context, s *store.Store, project, subproject string, staleDays int) (projectHealth, error) {
	modes, err := s.KindModes(ctx, project, subproject)
	if err != nil {
		return projectHealth{}, wrapNotFound(err, FormatLabel(project, subproject))
	}
	h := projectHealth{Healthy: true, Missing: []string{}, Exists: map[string]bool{}}
	for _, k := range standardKinds {
		h.Exists[k] = modes[k] != store.KindStorageNone
	}
	for _, k := range currentStateKinds {
		if !h.Exists[k] {
			h.Healthy = false
			h.Missing = append(h.Missing, k)
		}
	}
	h.Warnings, err = healthWarnings(ctx, s, project, subproject, modes, staleDays)
	if err != nil {
		return projectHealth{}, err
	}
	for _, k := range []string{"progress", "decisions"} {
		if !h.Exists[k] {
			h.Warnings = append(h.Warnings, healthWarning{File: k, Check: checkEmptyLog,
				Message: "no entry yet; record the work with update_project_memory or append_memory"})
		}
	}
	return h, nil
}

// VaultHealthReport is the report of a vault-wide health check: the vault
// path, whether every project in it is healthy, and one entry per project
// and subproject, in name order with each project followed by its
// subprojects.
type VaultHealthReport struct {
	Vault    string                `json:"vault"`
	Healthy  bool                  `json:"healthy"`
	Projects []ProjectHealthReport `json:"projects"`
}

// ProjectHealthReport is one project's entry in a VaultHealthReport: its
// name, its subproject name (empty for a top-level project), whether it is
// healthy, the standard kinds it misses and its warnings.
type ProjectHealthReport struct {
	Project    string          `json:"project"`
	Subproject string          `json:"subproject,omitempty"`
	Healthy    bool            `json:"healthy"`
	Missing    []string        `json:"missing"`
	Warnings   []healthWarning `json:"warnings"`
}

// VaultHealth checks every project and subproject of s, whose path is
// vault, with staleDays as the stale threshold. An empty vault is healthy.
// Store failures are returned as errors.
func VaultHealth(ctx context.Context, s *store.Store, vault string, staleDays int) (VaultHealthReport, error) {
	report := VaultHealthReport{Vault: vault, Healthy: true, Projects: []ProjectHealthReport{}}
	tree, err := s.ProjectTree(ctx)
	if err != nil {
		return VaultHealthReport{}, err
	}
	check := func(project, subproject string) error {
		h, err := checkHealth(ctx, s, project, subproject, staleDays)
		if err != nil {
			return err
		}
		if h.Warnings == nil {
			h.Warnings = []healthWarning{}
		}
		report.Healthy = report.Healthy && h.Healthy
		report.Projects = append(report.Projects, ProjectHealthReport{Project: project, Subproject: subproject, Healthy: h.Healthy, Missing: h.Missing, Warnings: h.Warnings})
		return nil
	}
	for _, top := range tree {
		if err := check(top.Name, ""); err != nil {
			return VaultHealthReport{}, err
		}
		for _, sub := range top.Subprojects {
			if err := check(top.Name, sub); err != nil {
				return VaultHealthReport{}, err
			}
		}
	}
	return report, nil
}

// VaultHealthText renders report as text: one status line per project
// (HEALTHY, UNHEALTHY with the missing count, or WARNINGS with their
// count), a summary line, then the missing files and warnings of every
// project that is not plainly healthy.
func VaultHealthText(report VaultHealthReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Health report for vault: %s\n", report.Vault)
	if len(report.Projects) == 0 {
		b.WriteString("\nNo projects in the vault.\n")
		return b.String()
	}
	b.WriteString("\n")
	var unhealthy, warned int
	for _, p := range report.Projects {
		label := FormatLabel(p.Project, p.Subproject)
		switch {
		case !p.Healthy:
			unhealthy++
			fmt.Fprintf(&b, "- %s: UNHEALTHY ❌ (%d missing)\n", label, len(p.Missing))
		case len(p.Warnings) > 0:
			warned++
			fmt.Fprintf(&b, "- %s: WARNINGS ⚠️ (%d)\n", label, len(p.Warnings))
		default:
			fmt.Fprintf(&b, "- %s: HEALTHY ✅\n", label)
		}
	}
	fmt.Fprintf(&b, "\n%d %s: %d healthy, %d unhealthy, %d with warnings only.\n",
		len(report.Projects), pluralize(len(report.Projects), "project", "projects"), len(report.Projects)-unhealthy-warned, unhealthy, warned)
	for _, p := range report.Projects {
		if p.Healthy && len(p.Warnings) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s:\n", FormatLabel(p.Project, p.Subproject))
		if len(p.Missing) > 0 {
			fmt.Fprintf(&b, "- missing: %s\n", strings.Join(p.Missing, ", "))
		}
		for _, w := range p.Warnings {
			fmt.Fprintf(&b, "- %s: %s\n", w.File, w.Message)
		}
	}
	if unhealthy > 0 {
		b.WriteString("\n" + missingFilesAdvice + "\n")
	}
	return b.String()
}

// healthWarnings returns the warnings for the project or subproject named
// by project and subproject in s: those of the current-state documents
// first, in standard kind order, then those of the two logs. modes is the
// project's store.KindModes, so only the kinds stored as documents are read.
// A current-state document still empty or equal to its blank template gets a
// template warning; otherwise it gets a stale warning when it was last
// updated more than staleDays days ago and progress or decisions has a dated
// entry after that day. Each log gets an undated_entries warning for undated
// entries other than a preamble and a large_history warning above
// largeHistoryLength active dated entries. Store failures are returned as
// errors.
func healthWarnings(ctx context.Context, s *store.Store, project, subproject string, modes map[string]store.KindStorage, staleDays int) ([]healthWarning, error) {
	var warnings []healthWarning
	newestEntry := ""
	logStats := map[string]store.EntryStats{}
	for _, k := range []string{"progress", "decisions"} {
		st, err := s.EntryStats(ctx, project, subproject, k)
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
		if modes[k] != store.KindStorageDocument {
			continue
		}
		content, ok, err := s.ReadDocument(ctx, project, subproject, k)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if isUnfilledTemplate(k, content, FormatLabel(project, subproject)) {
			warnings = append(warnings, healthWarning{File: k, Check: checkTemplate,
				Message: "still empty or holding the blank template; fill it in with write_memory"})
			continue
		}
		meta, err := s.Metadata(ctx, project, subproject, k)
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

// OutputSchema returns the JSON Schema of the structured content of every
// check_project_health result: a healthReport for one project, or a
// VaultHealthReport with all_projects. Both hold vault and healthy.
func (t *CheckProjectHealthTool) OutputSchema() map[string]any {
	warning := schemaObject(map[string]any{
		"file":    schemaString(),
		"check":   map[string]any{"type": "string", "enum": []string{checkStale, checkTemplate, checkUndatedEntries, checkLargeHistory, checkEmptyLog}},
		"message": schemaString(),
	}, "file", "check", "message")
	return schemaObject(map[string]any{
		"project":  schemaString(),
		"vault":    schemaString(),
		"healthy":  schemaBoolean(),
		"files":    map[string]any{"type": "object", "additionalProperties": schemaBoolean()},
		"warnings": schemaArray(warning),
		"projects": schemaArray(schemaObject(map[string]any{
			"project":    schemaString(),
			"subproject": schemaString(),
			"healthy":    schemaBoolean(),
			"missing":    schemaArray(schemaString()),
			"warnings":   schemaArray(warning),
		}, "project", "healthy", "missing", "warnings")),
	}, "vault", "healthy")
}
