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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestImportProjectCore_RoundTripsWithExport verifies that exporting a project
// and importing the directory into a new project restores its documents and
// its log entries with their original dates.
func TestImportProjectCore_RoundTripsWithExport(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "# Memory\noverview"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- did X"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-02-01", "## 2026-02-01\n- did Y"); err != nil {
		t.Fatal(err)
	}

	exportDir := t.TempDir()
	if _, err := exportProjectCore(ctx, s, "acme", "", exportDir, false); err != nil {
		t.Fatalf("exportProjectCore: %v", err)
	}

	// Import into a brand-new project in the same vault.
	report, err := importProjectCore(ctx, s, "acme-restored", "", exportDir, false)
	imported, skipped := report.Imported(), report.Skipped
	if err != nil {
		t.Fatalf("importProjectCore: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected nothing skipped, got %v", skipped)
	}
	if imported != 2 { // memory.md + progress.md
		t.Fatalf("imported = %d, want 2", imported)
	}

	content, ok, err := s.ReadContent(ctx, "acme-restored", "", "memory")
	if err != nil || !ok {
		t.Fatalf("ReadContent(memory): ok=%v err=%v", ok, err)
	}
	// exportProjectCore ends each file with a trailing newline, and import
	// stores the file content verbatim, trailing newline included.
	if content != "# Memory\noverview\n" {
		t.Errorf("memory content = %q", content)
	}

	entries, err := s.ReadEntries(ctx, "acme-restored", "", "progress", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].EntryDate != "2026-01-01" || entries[1].EntryDate != "2026-02-01" {
		t.Fatalf("expected 2 entries with the original dates preserved, got %+v", entries)
	}
}

// TestImportProjectCore_CreatesProjectIfMissing verifies that importing into a
// project that does not exist creates it.
func TestImportProjectCore_CreatesProjectIfMissing(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := importProjectCore(ctx, s, "brand-new", "", dir, false)
	imported := report.Imported()
	if err != nil {
		t.Fatalf("importProjectCore: %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported = %d, want 1", imported)
	}

	p, err := s.FindProjectByName(ctx, "brand-new", nil)
	if err != nil || p == nil {
		t.Fatalf("expected the project to have been created, got %+v, err %v", p, err)
	}
}

// TestImportProjectCore_SkipsInvalidKindNamesAndEmptyFiles verifies that
// import skips files whose name is not a valid kind, is a well-known
// non-kind name (README, CHANGELOG…) or whose content is blank, ignores
// files that are not .md, and imports the remaining files.
func TestImportProjectCore_SkipsInvalidKindNamesAndEmptyFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	files := map[string]string{
		"memory.md":    "real content",
		"README.md":    "a repository readme next to the export",
		"CHANGELOG.md": "a repository changelog",
		"empty.md":     "   ", // whitespace-only, treated as empty
		"notes.json":   "irrelevant, not .md",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A filename starting with a hyphen fails the kind slug pattern checked
	// by validateKind.
	if err := os.WriteFile(filepath.Join(dir, "-bad-name.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := importProjectCore(ctx, s, "acme", "", dir, false)
	imported, skipped := report.Imported(), report.Skipped
	if err != nil {
		t.Fatalf("importProjectCore: %v", err)
	}
	// Only memory.md is imported. README.md and CHANGELOG.md are valid
	// slugs but well-known non-kind names, "-bad-name.md" fails the slug
	// pattern and "empty.md" is blank; "notes.json" is not a .md file.
	if imported != 1 {
		t.Fatalf("imported = %d, want 1 (memory)", imported)
	}
	if want := []string{"-bad-name.md", "CHANGELOG.md", "README.md", "empty.md"}; !slices.Equal(skipped, want) {
		t.Fatalf("skipped = %v, want %v", skipped, want)
	}
	kinds, err := s.ListKinds(ctx, "acme", "", false)
	if err != nil || !slices.Equal(kinds, []string{"memory"}) {
		t.Fatalf("kinds = %v, %v; want only memory", kinds, err)
	}
}

// TestImportProjectCore_SkipsOversizedFileWithoutReadingIt verifies that a
// file larger than maxContentSize is reported as skipped without aborting the
// import of the other files.
func TestImportProjectCore_SkipsOversizedFileWithoutReadingIt(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("normal content"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, maxContentSize+1)
	for i := range big {
		big[i] = 'x'
	}
	if err := os.WriteFile(filepath.Join(dir, "architecture.md"), big, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := importProjectCore(ctx, s, "acme", "", dir, false)
	imported, skipped := report.Imported(), report.Skipped
	if err != nil {
		t.Fatalf("importProjectCore: %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported = %d, want 1 (memory.md) — an oversized file must be skipped, not abort the whole import", imported)
	}
	if len(skipped) != 1 || skipped[0] != "architecture.md" {
		t.Fatalf("skipped = %v, want [architecture.md]", skipped)
	}
}

// TestImportProjectCore_OverwritesExistingContent verifies that importing a
// file replaces the existing content of the same kind.
func TestImportProjectCore_OverwritesExistingContent(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "old content"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := importProjectCore(ctx, s, "acme", "", dir, false); err != nil {
		t.Fatalf("importProjectCore: %v", err)
	}

	content, ok, err := s.ReadContent(ctx, "acme", "", "memory")
	if err != nil || !ok {
		t.Fatalf("ReadContent: ok=%v err=%v", ok, err)
	}
	if content != "new content" {
		t.Errorf("content = %q, want %q", content, "new content")
	}
}

// TestImportProjectCore_ReportsAndDryRun verifies that a dry run reports which
// kinds would be created or overwritten without writing anything, that
// FormatImportReport mentions the dry run and the overwritten file, and that a
// real import then applies the content.
func TestImportProjectCore_ReportsAndDryRun(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "old"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"memory.md": "new", "stack.md": "Go", "README.txt": "ignored"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	report, err := importProjectCore(ctx, s, "acme", "", dir, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !slices.Equal(report.Created, []string{"stack.md"}) || !slices.Equal(report.Overwritten, []string{"memory.md"}) {
		t.Fatalf("report = %+v, want stack.md new and memory.md overwritten", report)
	}
	if content, _, _ := s.ReadContent(ctx, "acme", "", "memory"); content != "old" {
		t.Fatalf("dry run wrote memory = %q", content)
	}
	if text := FormatImportReport(report, "acme", dir, true); !strings.Contains(text, "Dry run") || !strings.Contains(text, "Overwritten: memory.md") {
		t.Errorf("FormatImportReport = %q", text)
	}

	if _, err := importProjectCore(ctx, s, "acme", "", dir, false); err != nil {
		t.Fatalf("import: %v", err)
	}
	if content, _, _ := s.ReadContent(ctx, "acme", "", "memory"); content != "new" {
		t.Fatalf("memory = %q after import, want %q", content, "new")
	}
}

// TestImportProjectCore_RejectsFilesCollidingAfterLowerCasing verifies that
// import fails when two files, such as Memory.md and memory.md, map to the
// same kind after lower-casing. The test is skipped on case-insensitive
// filesystems.
func TestImportProjectCore_RejectsFilesCollidingAfterLowerCasing(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Memory.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("b"), 0o644); err != nil {
		t.Skip("case-insensitive filesystem")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Skip("case-insensitive filesystem")
	}
	if _, err := importProjectCore(ctx, s, "acme", "", dir, false); err == nil {
		t.Fatal("expected Memory.md and memory.md to be rejected as the same kind")
	}
}

// TestImportProjectCore_RefusesMoreThanTheTotalLimit verifies that an
// import whose files hold more than maxImportTotalBytes in total fails
// before writing anything, even when each file is within maxContentSize.
func TestImportProjectCore_RefusesMoreThanTheTotalLimit(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	perFile := int64(maxContentSize - 1)
	for i := 0; int64(i)*perFile <= maxImportTotalBytes; i++ {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("kind%02d.md", i)))
		if err != nil {
			t.Fatal(err)
		}
		// A sparse file: the size counts, nothing is written to disk.
		if err := f.Truncate(perFile); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	if _, err := importProjectCore(ctx, s, "acme", "", dir, false); err == nil || !strings.Contains(err.Error(), "in total") {
		t.Fatalf("err = %v, want the total-size error", err)
	}
	if exists, err := s.ProjectExists(ctx, "acme", ""); err != nil || exists {
		t.Fatalf("project created by a refused import (exists=%v, err=%v)", exists, err)
	}
}
