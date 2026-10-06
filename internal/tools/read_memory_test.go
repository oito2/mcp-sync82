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

// TestReadMemoryTool_Document verifies that reading an overwrite-style kind
// returns its stored content.
func TestReadMemoryTool_Document(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "# Memory\n\nSome content."); err != nil {
		t.Fatal(err)
	}

	tool := &ReadMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "# Memory\n\nSome content."; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestReadMemoryTool_EmptyDocument verifies that a document with blank content
// is reported as empty.
func TestReadMemoryTool_EmptyDocument(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	// write_memory requires non-empty content, so the blank document is
	// written directly through the store.
	if err := s.WriteDocument(ctx, "acme", "", "memory", "   "); err != nil {
		t.Fatal(err)
	}

	tool := &ReadMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "(file is empty)"; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestReadMemoryTool_AppendOnlyConcatenatesInDateOrder verifies that reading
// an append-only kind joins its entries in date order regardless of the order
// they were appended.
func TestReadMemoryTool_AppendOnlyConcatenatesInDateOrder(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-02", "## 2026-01-02\n- second"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- first"); err != nil {
		t.Fatal(err)
	}

	tool := &ReadMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := "## 2026-01-01\n- first\n\n## 2026-01-02\n- second"
	if result.Text != want {
		t.Fatalf("Text =\n%q\nwant\n%q", result.Text, want)
	}
}

// TestReadMemoryTool_FileNotFound verifies that reading an unknown kind
// returns an execution error.
func TestReadMemoryTool_FileNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &ReadMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "nonexistent"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected an execution error for an unknown kind")
	}
}

// TestReadMemoryTool_ProjectNotFound verifies that reading from a nonexistent
// project returns an execution error.
func TestReadMemoryTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ReadMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "nonexistent", "filename": "memory"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	_, err = tool.Execute(context.Background(), parsed)
	if err == nil {
		t.Fatal("expected an execution error for a nonexistent project")
	}
}
