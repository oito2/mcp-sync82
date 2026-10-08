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

	"github.com/oito2/mcp-sync82/internal/store"
)

// TestExportProjectCore_SkipsKindThatWouldEscapeOutputDir verifies that
// ExportProject skips a kind such as "../evil", lists it as skipped and
// writes nothing outside the output directory. The kind is written with Store.WriteDocument directly,
// bypassing the kind validation done by the tools layer, to simulate an
// unsanitized kind reaching the export.
func TestExportProjectCore_SkipsKindThatWouldEscapeOutputDir(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "../evil", "malicious content"); err != nil {
		t.Fatal(err)
	}

	parent := t.TempDir()
	outputDir := filepath.Join(parent, "out")
	report, err := ExportProject(ctx, s, "acme", "", outputDir, false)
	if err != nil || report.Written != 0 || len(report.Skipped) != 1 || report.Skipped[0] != "../evil" {
		t.Fatalf("export = %+v, %v; want the kind skipped and nothing written", report, err)
	}
	if _, err := os.Stat(filepath.Join(parent, "evil.md")); !os.IsNotExist(err) {
		t.Fatalf("a file was written outside the output directory (stat err = %v)", err)
	}
}

// TestExportProject_SkipsAKindWhoseNameCantBeAFileName stores a kind whose
// name is longer than validateKind allows (written straight to the store),
// and checks that the export writes the other kinds and lists it as skipped
// instead of failing.
func TestExportProject_SkipsAKindWhoseNameCantBeAFileName(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("k", 300)
	for _, kind := range []string{"memory", long} {
		if err := s.WriteDocument(ctx, "acme", "", kind, "content"); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ExportProject(ctx, s, "acme", "", t.TempDir(), false)
	if err != nil || report.Written != 1 || len(report.Skipped) != 1 || report.Skipped[0] != long {
		t.Fatalf("export = %+v, %v; want memory written and the long kind skipped", report, err)
	}
	if !strings.Contains(FormatExportNotes(report), "Skipped") {
		t.Errorf("notes = %q, want the skipped kind named", FormatExportNotes(report))
	}
}

// TestValidateKind_LengthAndSpaces checks the 128-character limit of kind
// names, that surrounding spaces are trimmed and the name lower-cased, and
// that validateProjectName enforces the same limit.
func TestValidateKind_LengthAndSpaces(t *testing.T) {
	if _, err := validateKind(strings.Repeat("a", maxNameLength)); err != nil {
		t.Errorf("a %d-character name: %v", maxNameLength, err)
	}
	if _, err := validateKind(strings.Repeat("a", maxNameLength+1)); err == nil || !strings.Contains(err.Error(), "at most 128") {
		t.Errorf("a %d-character name: err = %v, want the limit named", maxNameLength+1, err)
	}
	if got, err := validateKind("  Memory "); err != nil || got != "memory" {
		t.Errorf("validateKind(\"  Memory \") = %q, %v; want memory", got, err)
	}
	if err := validateProjectName("project", strings.Repeat("p", maxNameLength+1)); err == nil {
		t.Error("a project name over the limit was accepted")
	}
}

// TestExportProject_ManifestRestoresACustomLog exports a custom log and a
// custom document and imports them into a new project: the manifest makes
// the log come back as a log, and the document as a document. The
// manifest is not counted as a written file.
func TestExportProject_ManifestRestoresACustomLog(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, _ := mgr.Get(ctx, r.DefaultDBPath)
	if _, _, err := s.EnsureProject(ctx, "src", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "src", "", "api", "2026-01-01", "## 2026-01-01\n- change"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "src", "", "notes", "plain notes"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	report, err := ExportProject(ctx, s, "src", "", dir, false)
	if err != nil || report.Written != 2 {
		t.Fatalf("export = %+v, %v; want 2 files written", report, err)
	}
	if _, err := os.Stat(filepath.Join(dir, kindsManifestName)); err != nil {
		t.Fatalf("no manifest written: %v", err)
	}
	if _, err := ImportProject(ctx, s, "dst", "", dir, false); err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[string]store.KindStorage{"api": store.KindStorageEntries, "notes": store.KindStorageDocument} {
		if mode, err := s.KindMode(ctx, "dst", "", kind); err != nil || mode != want {
			t.Errorf("%s after import: mode %v, %v; want %v", kind, mode, err, want)
		}
	}
}

// TestImportProject_RejectsABadManifest checks that a manifest of another
// format version, or one that isn't JSON, stops the import before
// anything is written.
func TestImportProject_RejectsABadManifest(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, _ := mgr.Get(ctx, r.DefaultDBPath)
	for _, manifest := range []string{`{"version": 9, "kinds": {}}`, `not json`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, kindsManifestName), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ImportProject(ctx, s, "p", "", dir, false); err == nil || !strings.Contains(err.Error(), kindsManifestName) {
			t.Errorf("manifest %q: err = %v, want it refused", manifest, err)
		}
	}
	if exists, _ := s.ProjectExists(ctx, "p", ""); exists {
		t.Error("a refused import created the project")
	}
}

// TestExportProject_ListsStaleFiles checks that an export with overwrite
// into a folder holding the files of a kind the project no longer has
// lists them, and leaves non-kind files such as README.md out.
func TestExportProject_ListsStaleFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, _ := mgr.Get(ctx, r.DefaultDBPath)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "m"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"old.md", "old.archived.md", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := ExportProject(ctx, s, "acme", "", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(report.Stale, ",") != "old.archived.md,old.md" {
		t.Errorf("stale = %v, want old.archived.md and old.md", report.Stale)
	}
	if !strings.Contains(FormatExportNotes(report), "an import would bring them back") {
		t.Errorf("notes = %q", FormatExportNotes(report))
	}
}
