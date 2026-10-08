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
	"testing"
)

// TestAppendMemoryTool_Validate_RequiresDateHeaderForStandardKinds verifies
// that appending to the progress and decisions kinds fails validation when the
// content has no date header, and passes when it has one.
func TestAppendMemoryTool_Validate_RequiresDateHeaderForStandardKinds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress", "content": "no header here"})); err == nil {
		t.Fatal("expected a validation error when progress content has no date header")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "decisions", "content": "no header here"})); err == nil {
		t.Fatal("expected a validation error when decisions content has no date header")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress", "content": "## 2026-01-01\n- ok"})); err != nil {
		t.Fatalf("expected valid content with a date header to pass, got: %v", err)
	}
}

// TestAppendMemoryTool_CustomKindNeverRequiresDateHeader verifies that
// appending to a custom kind without a date header succeeds and stores a
// single undated entry.
func TestAppendMemoryTool_CustomKindNeverRequiresDateHeader(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "notes", "content": "just a note, no date"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "Appended to: acme/notes"; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "notes", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].EntryDate != "" {
		t.Fatalf("expected 1 undated entry, got %+v", entries)
	}
}

// TestAppendMemoryTool_AppendsWithExtractedDate verifies that the entry date
// is taken from the first date header of the appended content.
func TestAppendMemoryTool_AppendsWithExtractedDate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress", "content": "## 2026-03-15\n- did a thing"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].EntryDate != "2026-03-15" {
		t.Fatalf("expected 1 entry dated 2026-03-15, got %+v", entries)
	}
}

// TestAppendMemoryTool_ProjectNotFound verifies that executing against a
// nonexistent project returns an error.
func TestAppendMemoryTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "nonexistent", "filename": "notes", "content": "x"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(context.Background(), parsed); err == nil {
		t.Fatal("expected an execution error for a nonexistent project")
	}
}

// TestAppendMemoryTool_RejectsOverwriteStyleStandardKind verifies that
// appending to the overwrite-style "memory" kind fails validation, since
// entries appended to it would never be returned by any read path.
func TestAppendMemoryTool_RejectsOverwriteStyleStandardKind(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory", "content": "note"})); err == nil {
		t.Fatal("expected a validation error when appending to memory")
	}
}

// TestAppendMemoryTool_MarkerOnlyContentIsRefused checks that content
// holding only entry id markers is refused in Validate, like empty
// content, instead of failing later in Execute.
func TestAppendMemoryTool_MarkerOnlyContentIsRefused(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &AppendMemoryTool{Resolver: r, Stores: mgr}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "notes", "content": "<!-- entry:3 -->\n"})); err == nil {
		t.Error("content made only of an entry marker was accepted")
	}
}
