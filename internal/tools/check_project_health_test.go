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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// TestCheckProjectHealthTool_Healthy verifies that a project with every
// standard kind populated is reported HEALTHY, with each kind marked OK and a
// non-error result.
func TestCheckProjectHealthTool_Healthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps"} {
		if err := s.WriteDocument(ctx, "acme", "", k, "content"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- x"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "decisions", "2026-01-01", "## 2026-01-01\n- y"); err != nil {
		t.Fatal(err)
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false for a healthy project, got text: %s", result.Text)
	}
	if !strings.Contains(result.Text, "HEALTHY") {
		t.Fatalf("expected HEALTHY status, got: %s", result.Text)
	}
	for _, k := range standardKinds {
		if !strings.Contains(result.Text, k+": OK") {
			t.Errorf("expected %q reported OK, got: %s", k, result.Text)
		}
	}
}

// TestCheckProjectHealthTool_Unhealthy verifies that a project with missing
// kinds is reported UNHEALTHY with the missing kinds listed and a
// recommendation, and that this is returned as an error result rather than a
// Go error.
func TestCheckProjectHealthTool_Unhealthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	// Only memory is written — everything else stays missing.
	if err := s.WriteDocument(ctx, "acme", "", "memory", "content"); err != nil {
		t.Fatal(err)
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for an unhealthy project — this is deliberate, not a crash")
	}
	if !strings.Contains(result.Text, "UNHEALTHY") {
		t.Fatalf("expected UNHEALTHY status, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "memory: OK") {
		t.Fatalf("expected memory reported OK, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "architecture: MISSING") {
		t.Fatalf("expected architecture reported MISSING, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Recommendation") {
		t.Fatalf("expected a recommendation when unhealthy, got: %s", result.Text)
	}
}

// TestCheckProjectHealthTool_NeedsInput verifies that, with no project
// resolvable, the tool returns NeedsInputMessage as a non-error result.
func TestCheckProjectHealthTool_NeedsInput(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Text != NeedsInputMessage {
		t.Fatalf("expected NeedsInputMessage, got: %s", result.Text)
	}
	if result.IsError {
		t.Fatal("NeedsInputMessage must not be an error result")
	}
}

// TestCheckProjectHealthTool_FullyArchivedLogStaysHealthy verifies that a log
// kind (progress, decisions) whose entries have all been archived still counts
// as present, so the project stays HEALTHY.
func TestCheckProjectHealthTool_FullyArchivedLogStaysHealthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps"} {
		if err := s.WriteDocument(ctx, "acme", "", k, "content"); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []string{"progress", "decisions"} {
		if err := s.AppendEntry(ctx, "acme", "", k, "2020-01-01", "## 2020-01-01\n- old"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ArchiveEntries(ctx, "acme", "", k, "2021-01-01", nil); err != nil {
			t.Fatal(err)
		}
	}

	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError || !strings.Contains(result.Text, "HEALTHY") || strings.Contains(result.Text, "UNHEALTHY") {
		t.Fatalf("expected HEALTHY, got: %s", result.Text)
	}
}

// seedHealthyProject creates project acme in the default vault of r with
// the four current-state documents filled in and one dated entry in each
// log, dated date, and returns the store.
func seedHealthyProject(t *testing.T, r *Resolver, mgr *store.Manager, date string) *store.Store {
	t.Helper()
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range currentStateKinds {
		if err := s.WriteDocument(ctx, "acme", "", k, "# "+k+"\n\nreal content"); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []string{"progress", "decisions"} {
		if err := s.AppendEntry(ctx, "acme", "", k, date, "## "+date+"\n- done"); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// setHealthNow makes healthNow return now until the test ends.
func setHealthNow(t *testing.T, now time.Time) {
	t.Helper()
	saved := healthNow
	healthNow = func() time.Time { return now }
	t.Cleanup(func() { healthNow = saved })
}

// TestCheckProjectHealthTool_NoWarningsKeepsTheOutput verifies that a
// project without warnings gets exactly the report it got before warnings
// existed.
func TestCheckProjectHealthTool_NoWarningsKeepsTheOutput(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	result := runTool(t, &CheckProjectHealthTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme"})
	want := "Status: HEALTHY ✅\n\nFiles:\n- memory: OK\n- architecture: OK\n- stack: OK\n- decisions: OK\n- progress: OK\n- next_steps: OK\n"
	if result.IsError || !strings.HasPrefix(result.Text, "Health report for project: acme") || !strings.HasSuffix(result.Text, want) {
		t.Fatalf("report = %q, want it to end with %q", result.Text, want)
	}
}

// TestCheckProjectHealthTool_WarnsAboutStaleDocuments verifies that a
// current-state document older than stale_days is reported once the logs
// have a newer entry, and not when they don't or stale_days is larger.
func TestCheckProjectHealthTool_WarnsAboutStaleDocuments(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	now := time.Now().UTC()
	s := seedHealthyProject(t, r, mgr, now.Format("2006-01-02"))
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	setHealthNow(t, now.AddDate(0, 0, 40))

	if text := runTool(t, tool, map[string]any{"project": "acme"}).Text; strings.Contains(text, "Warnings") {
		t.Fatalf("no entry is newer than the documents, so no warning expected: %s", text)
	}

	later := now.AddDate(0, 0, 10).Format("2006-01-02")
	if err := s.AppendEntry(context.Background(), "acme", "", "decisions", later, "## "+later+"\n- changed"); err != nil {
		t.Fatal(err)
	}
	result := runTool(t, tool, map[string]any{"project": "acme"})
	if result.IsError || !strings.Contains(result.Text, "Status: HEALTHY") {
		t.Fatalf("warnings must not make the project unhealthy: %+v", result)
	}
	want := fmt.Sprintf("- architecture: last updated %s (40 days ago), but progress/decisions have entries up to %s;", now.Format("2006-01-02"), later)
	if !strings.Contains(result.Text, "\nWarnings:\n") || !strings.Contains(result.Text, want) {
		t.Fatalf("report = %s\nwant a line starting %q", result.Text, want)
	}
	if n := strings.Count(result.Text, "days ago"); n != 4 {
		t.Errorf("got %d stale warnings, want one per current-state document", n)
	}

	if text := runTool(t, tool, map[string]any{"project": "acme", "stale_days": 60}).Text; strings.Contains(text, "Warnings") {
		t.Errorf("stale_days 60 should not warn about 40-day-old documents: %s", text)
	}
}

// TestCheckProjectHealthTool_WarnsAboutTemplates verifies that a blank
// document and one still equal to the init_project_memory template, with
// any date, are reported, and that a filled-in one is not.
func TestCheckProjectHealthTool_WarnsAboutTemplates(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s := seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	ctx := context.Background()
	memory := lastUpdatedLine.ReplaceAllString(renderMemoryTemplate(initAnswers{}, "acme"), "- Last updated: 2020-01-01")
	for kind, content := range map[string]string{
		"memory":       memory,
		"stack":        "  \n",
		"architecture": renderArchitectureTemplate(initAnswers{}, "acme") + "\n",
	} {
		if err := s.WriteDocument(ctx, "acme", "", kind, content); err != nil {
			t.Fatal(err)
		}
	}
	text := runTool(t, &CheckProjectHealthTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme"}).Text
	for _, kind := range []string{"memory", "architecture", "stack"} {
		if !strings.Contains(text, "- "+kind+": still empty or holding the blank template") {
			t.Errorf("no template warning for %s: %s", kind, text)
		}
	}
	if strings.Contains(text, "- next_steps: still empty") {
		t.Errorf("the filled-in next_steps was reported: %s", text)
	}
}

// TestCheckProjectHealthTool_WarnsAboutLogs verifies the undated-entries
// warning, which ignores a preamble, and the large-history warning.
func TestCheckProjectHealthTool_WarnsAboutLogs(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s := seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	ctx := context.Background()
	if err := s.ReplaceAllEntries(ctx, "acme", "", "decisions", []store.EntrySection{
		{Body: "# Decisions", Preamble: true},
		{Date: "2026-01-01", Body: "## 2026-01-01\n- a"},
		{Body: "an undated decision"},
		{Body: "another undated decision"},
	}); err != nil {
		t.Fatal(err)
	}
	sections := make([]store.EntrySection, largeHistoryLength+1)
	for i := range sections {
		sections[i] = store.EntrySection{Date: "2026-01-01", Body: "## 2026-01-01\n- step"}
	}
	if err := s.ReplaceAllEntries(ctx, "acme", "", "progress", sections); err != nil {
		t.Fatal(err)
	}

	text := runTool(t, &CheckProjectHealthTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme"}).Text
	if !strings.Contains(text, "- decisions: 2 undated entries; archive_memory never archives them") {
		t.Errorf("no undated warning for decisions: %s", text)
	}
	if !strings.Contains(text, fmt.Sprintf("- progress: %d active dated entries; archive the old ones with archive_memory", largeHistoryLength+1)) {
		t.Errorf("no large-history warning for progress: %s", text)
	}
	if strings.Contains(text, "- progress: 0 undated") || strings.Contains(text, "- decisions: 3 undated") {
		t.Errorf("the preamble or an empty count was reported: %s", text)
	}
}

// TestCheckProjectHealthTool_JSONWarnings verifies that warnings appear in
// the JSON report, with their check, and are omitted when there are none.
func TestCheckProjectHealthTool_JSONWarnings(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s := seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}

	if text := runTool(t, tool, map[string]any{"project": "acme", "format": "json"}).Text; strings.Contains(text, "warnings") {
		t.Fatalf("warnings should be omitted when there are none: %s", text)
	}
	if err := s.WriteDocument(context.Background(), "acme", "", "stack", ""); err != nil {
		t.Fatal(err)
	}
	var report healthReport
	if err := json.Unmarshal([]byte(runTool(t, tool, map[string]any{"project": "acme", "format": "json"}).Text), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Healthy || len(report.Warnings) != 1 || report.Warnings[0].File != "stack" || report.Warnings[0].Check != checkTemplate {
		t.Fatalf("report = %+v, want healthy with one template warning for stack", report)
	}
}

// TestCheckProjectHealthTool_ValidateStaleDays verifies the stale_days
// default and that a negative value is rejected.
func TestCheckProjectHealthTool_ValidateStaleDays(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme"}))
	if err != nil || parsed.(checkProjectHealthArgs).StaleDays != defaultStaleDays {
		t.Fatalf("default stale_days: %+v, %v", parsed, err)
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "stale_days": -1})); err == nil {
		t.Fatal("a negative stale_days should be rejected")
	}
}

// TestCheckProjectHealthTool_AllProjects checks a vault holding a healthy
// project, an unhealthy subproject and a project with a warning only: one
// status line each, details for the two that aren't plainly healthy, an
// error result because one is unhealthy, and the same verdicts in JSON.
func TestCheckProjectHealthTool_AllProjects(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s := seedHealthyProject(t, r, mgr, time.Now().UTC().Format("2006-01-02"))
	if _, _, err := s.EnsureProject(ctx, "acme", "api"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "api", "memory", "# api"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "beta", ""); err != nil {
		t.Fatal(err)
	}
	for _, k := range standardKinds {
		if err := s.WriteDocument(ctx, "beta", "", k, "# "+k+"\n\nreal content"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.WriteDocument(ctx, "beta", "", "stack", ""); err != nil {
		t.Fatal(err)
	}
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}

	res := runTool(t, tool, map[string]any{"all_projects": true})
	if !res.IsError {
		t.Error("an unhealthy subproject should make the vault report an error result")
	}
	for _, want := range []string{
		"- acme: HEALTHY ✅", "- acme/api: UNHEALTHY ❌ (3 missing)", "- beta: WARNINGS ⚠️ (1)",
		"3 projects: 1 healthy, 1 unhealthy, 1 with warnings only.",
		"acme/api:\n- missing: architecture, stack, next_steps", "beta:\n- stack: still empty",
	} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("text report lacks %q:\n%s", want, res.Text)
		}
	}
	if _, ok := res.Structured.(VaultHealthReport); !ok {
		t.Errorf("structured content = %T, want VaultHealthReport", res.Structured)
	}

	var report VaultHealthReport
	if err := json.Unmarshal([]byte(runTool(t, tool, map[string]any{"all_projects": true, "format": "json"}).Text), &report); err != nil {
		t.Fatal(err)
	}
	if report.Vault != r.DefaultDBPath || report.Healthy || len(report.Projects) != 3 {
		t.Fatalf("report = %+v", report)
	}
	api := report.Projects[1]
	if api.Project != "acme" || api.Subproject != "api" || api.Healthy || len(api.Missing) != 3 {
		t.Errorf("acme/api entry = %+v", api)
	}
	if beta := report.Projects[2]; !beta.Healthy || len(beta.Warnings) != 1 || beta.Missing == nil {
		t.Errorf("beta entry = %+v, want healthy, one warning and an empty missing list", beta)
	}
}

// TestCheckProjectHealthTool_AllProjectsEmptyVault reports an empty vault
// as healthy, in text and JSON.
func TestCheckProjectHealthTool_AllProjectsEmptyVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	res := runTool(t, tool, map[string]any{"all_projects": true})
	if res.IsError || !strings.Contains(res.Text, "No projects in the vault.") {
		t.Errorf("empty vault: %+v", res)
	}
	if text := runTool(t, tool, map[string]any{"all_projects": true, "format": "json"}).Text; !strings.Contains(text, `"projects": []`) || !strings.Contains(text, `"healthy": true`) {
		t.Errorf("empty vault JSON = %s", text)
	}
}

// TestCheckProjectHealthTool_AllProjectsConflicts rejects all_projects
// combined with an argument naming one project, and never falls back to
// the last-used project.
func TestCheckProjectHealthTool_AllProjectsConflicts(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	for _, arg := range []string{"project", "subproject", "workspace_root"} {
		if _, err := tool.Validate(mustJSON(t, map[string]any{"all_projects": true, arg: "x"})); err == nil || !strings.Contains(err.Error(), arg) {
			t.Errorf("all_projects with %s: err = %v", arg, err)
		}
	}
	if err := config.UpdateLastProject("ghost", "", filepath.Join(t.TempDir(), "other.db")); err != nil {
		t.Fatal(err)
	}
	res := runTool(t, tool, map[string]any{"all_projects": true})
	if !strings.Contains(res.Text, r.DefaultDBPath) {
		t.Errorf("all_projects should check the default vault, not the last session's: %s", res.Text)
	}
}

// TestCheckProjectHealthTool_AllProjectsMissingVault reports a path with no
// vault as an error result without creating it.
func TestCheckProjectHealthTool_AllProjectsMissingVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	missing := filepath.Join(t.TempDir(), "none.db")
	res := runTool(t, tool, map[string]any{"all_projects": true, "path": missing})
	if !res.IsError || !strings.Contains(res.Text, "No vault exists") {
		t.Errorf("missing vault: %+v", res)
	}
}

// TestCheckProjectHealthTool_FreshProjectIsHealthy checks that a project
// just set up by init_project_memory, whose logs have no entry yet, is
// healthy, with an empty_log warning for each log and the logs shown as
// empty rather than missing.
func TestCheckProjectHealthTool_FreshProjectIsHealthy(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	runTool(t, &InitProjectMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "fresh"})
	tool := &CheckProjectHealthTool{Resolver: r, Stores: mgr}
	res := runTool(t, tool, map[string]any{"project": "fresh"})
	if res.IsError || !strings.Contains(res.Text, "Status: HEALTHY") || !strings.Contains(res.Text, "- progress: EMPTY (no entry yet)") {
		t.Fatalf("fresh project: isError=%v\n%s", res.IsError, res.Text)
	}
	var report healthReport
	if err := json.Unmarshal([]byte(runTool(t, tool, map[string]any{"project": "fresh", "format": "json"}).Text), &report); err != nil {
		t.Fatal(err)
	}
	empty := 0
	for _, w := range report.Warnings {
		if w.Check == checkEmptyLog {
			empty++
		}
	}
	if !report.Healthy || report.Files["progress"] || report.Files["decisions"] || empty != 2 {
		t.Errorf("report = %+v, want healthy, logs not present and two empty_log warnings", report)
	}

	runTool(t, &CreateProjectTool{Resolver: r, Stores: mgr}, map[string]any{"project": "bare"})
	res = runTool(t, tool, map[string]any{"project": "bare"})
	if !res.IsError || !strings.Contains(res.Text, "Use init_project_memory to write the missing files") {
		t.Errorf("a bare project: isError=%v\n%s", res.IsError, res.Text)
	}
}
