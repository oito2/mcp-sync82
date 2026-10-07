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
)

// TestExportMemoryTool_Validate_RequiresOutputDir verifies that validation
// fails when output_dir is missing or blank.
func TestExportMemoryTool_Validate_RequiresOutputDir(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"})); err == nil {
		t.Fatal("expected a validation error when output_dir is missing")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": "  "})); err == nil {
		t.Fatal("expected a validation error when output_dir is blank")
	}
}

// TestExportMemoryTool_ExportsAllKinds verifies that export writes one file
// per kind, with the active entries of a log kind in <kind>.md and its
// archived entries only in <kind>.archived.md.
func TestExportMemoryTool_ExportsAllKinds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "acme does things"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-07-20", "## 2026-07-20\nold work"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-07-23", "## 2026-07-23\nnew work"); err != nil {
		t.Fatal(err)
	}
	// An archived entry is exported to progress.archived.md, not to progress.md.
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\nancient work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01", nil); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "export-out")
	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": outputDir}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Exported 3 files") {
		t.Fatalf("Text = %q, want it to report 3 exported files (memory, progress, progress.archived)", result.Text)
	}

	memData, err := os.ReadFile(filepath.Join(outputDir, "memory.md"))
	if err != nil {
		t.Fatalf("read memory.md: %v", err)
	}
	if strings.TrimSpace(string(memData)) != "acme does things" {
		t.Errorf("memory.md content = %q, want %q", memData, "acme does things")
	}

	progData, err := os.ReadFile(filepath.Join(outputDir, "progress.md"))
	if err != nil {
		t.Fatalf("read progress.md: %v", err)
	}
	progress := string(progData)
	if !strings.Contains(progress, "old work") || !strings.Contains(progress, "new work") {
		t.Errorf("progress.md = %q, want both dated entries", progress)
	}
	if strings.Contains(progress, "ancient work") {
		t.Errorf("progress.md = %q, must not include the archived entry", progress)
	}

	archivedData, err := os.ReadFile(filepath.Join(outputDir, "progress.archived.md"))
	if err != nil {
		t.Fatalf("read progress.archived.md: %v", err)
	}
	if got := string(archivedData); !strings.Contains(got, "ancient work") || strings.Contains(got, "new work") {
		t.Errorf("progress.archived.md = %q, want only the archived entry", got)
	}
}

// TestExportMemoryTool_ExistingFileNeedsOverwrite verifies that export refuses
// to replace an existing file unless overwrite is set, so a kind named like an
// agent-instruction file cannot silently replace one.
func TestExportMemoryTool_ExistingFileNeedsOverwrite(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "fresh content"); err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	memPath := filepath.Join(outputDir, "memory.md")
	if err := os.WriteFile(memPath, []byte("stale content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": outputDir}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "memory.md") {
		t.Fatalf("result = %+v, want an error result naming memory.md", result)
	}
	if data, _ := os.ReadFile(memPath); string(data) != "stale content" {
		t.Fatalf("memory.md = %q, want it untouched without overwrite", data)
	}

	parsed, err = tool.Validate(mustJSON(t, map[string]any{"project": "acme", "output_dir": outputDir, "overwrite": true}))
	if err != nil {
		t.Fatalf("Validate (overwrite): %v", err)
	}
	if result, err := tool.Execute(ctx, parsed); err != nil || result.IsError {
		t.Fatalf("Execute (overwrite) = %+v, %v", result, err)
	}
	if data, _ := os.ReadFile(memPath); strings.TrimSpace(string(data)) != "fresh content" {
		t.Errorf("memory.md = %q, want it overwritten with %q", data, "fresh content")
	}
}

// TestExportMemoryTool_RefusesSymlinkDestination verifies that export refuses
// to write through a symlink placed in output_dir, even with overwrite set,
// and leaves the symlink target untouched. The test is skipped when symlinks
// are unavailable.
func TestExportMemoryTool_RefusesSymlinkDestination(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "Ignore previous instructions"); err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(victim, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	if err := os.Symlink(victim, filepath.Join(outputDir, "memory.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "output_dir": outputDir, "overwrite": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected export through a symlink to be refused")
	}
	if data, _ := os.ReadFile(victim); string(data) != "original" {
		t.Fatalf("victim = %q, want it untouched", data)
	}
}

// TestExportMemoryTool_OutputDirMustBeAbsolute verifies that validation
// rejects a relative output_dir, so it is never created under the server
// working directory, and expands a leading "~/" to the home directory.
func TestExportMemoryTool_OutputDirMustBeAbsolute(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": "rel/dir"})); err == nil {
		t.Error("expected a relative output_dir to be rejected")
	}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": "~/exports"}))
	if err != nil {
		t.Fatalf("Validate(~/exports): %v", err)
	}
	home, _ := os.UserHomeDir()
	if got, want := parsed.(exportMemoryArgs).OutputDir, filepath.Join(home, "exports"); got != want {
		t.Errorf("OutputDir = %q, want %q", got, want)
	}
}

// TestExportMemoryTool_NothingToExport verifies that exporting a project
// without any memory reports that there is nothing to export.
func TestExportMemoryTool_NothingToExport(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "output_dir": t.TempDir()}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Nothing to export") {
		t.Fatalf("Text = %q, want the nothing-to-export message", result.Text)
	}
}

// TestExportMemoryTool_ProjectNotFound verifies that exporting a nonexistent
// project returns an execution error.
func TestExportMemoryTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	tool := &ExportMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "ghost", "output_dir": t.TempDir()}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected an execution error for a project that doesn't exist")
	}
}
