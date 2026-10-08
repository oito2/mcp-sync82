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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestImportMemoryTool_ImportsIntoNewProject verifies that importing a
// directory into a project that does not exist yet creates it, stores the file
// content and reports the imported file count.
func TestImportMemoryTool_ImportsIntoNewProject(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("restored content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "input_dir": dir}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Imported 1 file into acme") {
		t.Fatalf("Text = %q, want it to report 1 imported file", result.Text)
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	content, ok, err := s.ReadContent(ctx, "acme", "", "memory")
	if err != nil || !ok || content != "restored content" {
		t.Fatalf("content = %q ok=%v err=%v", content, ok, err)
	}
}

// TestImportMemoryTool_ReportsSkippedFiles verifies that the result reports
// zero imported files and lists a file with an invalid kind name as skipped.
func TestImportMemoryTool_ReportsSkippedFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "-bad.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "input_dir": dir}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Imported 0 files") {
		t.Fatalf("Text = %q, want it to report 0 imported files", result.Text)
	}
	if !strings.Contains(result.Text, "Skipped") || !strings.Contains(result.Text, "-bad.md") {
		t.Fatalf("Text = %q, want it to list -bad.md as skipped", result.Text)
	}
}

// TestImportMemoryTool_Validate_RequiresInputDir verifies that validation
// fails when input_dir is missing.
func TestImportMemoryTool_Validate_RequiresInputDir(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"})); err == nil {
		t.Fatal("expected an error when input_dir is missing")
	}
}

// TestImportMemoryTool_NeedsInputMessageWhenProjectUnresolved verifies that,
// when no project can be resolved, the tool returns NeedsInputMessage.
func TestImportMemoryTool_NeedsInputMessageWhenProjectUnresolved(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"input_dir": t.TempDir()}))
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
}

// TestImportMemoryTool_DryRunDoesNotCreateVault verifies that a dry run
// against a vault that does not exist yet reports every file as new and
// leaves no vault file behind.
func TestImportMemoryTool_DryRunDoesNotCreateVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "new-vault.db")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "input_dir": dir, "path": dbPath, "dry_run": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError || !strings.Contains(result.Text, "New: memory.md") {
		t.Fatalf("result = %+v, want a dry-run report with memory.md as new", result)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("vault file exists after a dry run (stat err = %v)", err)
	}
}

// TestImportMemoryTool_RefusesProjectFromLastSession verifies that an import
// without project or workspace_root, which resolves to the remembered last
// project, returns an error result: a missing remembered project is not
// created, and an existing one keeps its content.
func TestImportMemoryTool_RefusesProjectFromLastSession(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		*c = config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}
	}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("imported"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"input_dir": dir}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "Refusing to import memory") {
		t.Fatalf("result = %+v, want an IsError result refusing the remembered project", result)
	}
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := s.ProjectExists(ctx, "acme", ""); err != nil || exists {
		t.Fatalf("ProjectExists(acme) = %v, %v; want false, nil", exists, err)
	}

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "original"); err != nil {
		t.Fatal(err)
	}
	result, err = tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "Refusing to import memory") {
		t.Fatalf("result = %+v, want an IsError result refusing the remembered project", result)
	}
	if got, _, err := s.ReadDocument(ctx, "acme", "", "memory"); err != nil || got != "original" {
		t.Fatalf("memory = %q, %v; want %q, nil (unchanged)", got, err, "original")
	}
}

// TestExportImportRoundTrip_KeepsArchivedEntries verifies that exporting a
// project and importing it back keeps archived entries archived and active
// entries active, so the round trip loses no entry.
func TestExportImportRoundTrip_KeepsArchivedEntries(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\nancient work"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-07-23", "## 2026-07-23\nnew work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01", nil); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if _, err := exportProjectCore(ctx, s, "acme", "", dir, false); err != nil {
		t.Fatalf("export: %v", err)
	}
	if report, err := importProjectCore(ctx, s, "acme", "", dir, false); err != nil || len(report.Skipped) != 0 {
		t.Fatalf("import: skipped=%v err=%v", report.Skipped, err)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil {
		t.Fatal(err)
	}
	var archived, visible int
	for _, e := range entries {
		if e.Archived {
			archived++
			if !strings.Contains(e.Body, "ancient work") {
				t.Errorf("archived entry = %q, want the ancient work", e.Body)
			}
		} else {
			visible++
		}
	}
	if archived != 1 || visible != 1 {
		t.Fatalf("after round trip: %d archived, %d visible entries; want 1 and 1", archived, visible)
	}
}

// TestImportMemoryTool_RejectsInvalidProjectNames verifies that import never
// creates a project with an invalid name, including path-like names such as
// "../../escape" that could escape the output folder of "sync82 export --all".
// Each case is rejected either by validation or by an error result.
func TestImportMemoryTool_RejectsInvalidProjectNames(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ImportMemoryTool{Resolver: r, Stores: mgr}
	for _, args := range []map[string]string{
		{"project": "../../escape", "input_dir": dir},
		{"project": "a b", "input_dir": dir},
		{"project": "acme", "subproject": "x/y", "input_dir": dir},
	} {
		parsed, err := tool.Validate(mustJSON(t, args))
		if err != nil {
			continue // rejected up front is fine too
		}
		result, err := tool.Execute(ctx, parsed)
		if err != nil {
			t.Fatalf("Execute(%v): %v", args, err)
		}
		if !result.IsError {
			t.Errorf("Execute(%v) = %+v, want an error result", args, result)
		}
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if projects, err := s.ListTopLevelProjects(ctx); err != nil || len(projects) != 0 {
		t.Fatalf("projects = %+v, err = %v; want none created", projects, err)
	}
}
