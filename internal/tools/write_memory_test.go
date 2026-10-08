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
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestWriteMemoryTool_OverwritesDocumentInPlace verifies that writing to an
// existing document kind replaces its content.
func TestWriteMemoryTool_OverwritesDocumentInPlace(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &WriteMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory", "content": "first"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute (first write): %v", err)
	}

	parsed2, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory", "content": "second"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed2)
	if err != nil {
		t.Fatalf("Execute (second write): %v", err)
	}
	if want := "Written: acme/memory"; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	content, ok, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || content != "second" {
		t.Fatalf("got (%q, %v), want (\"second\", true)", content, ok)
	}
}

// TestWriteMemoryTool_OverwritesAppendOnlyKindEntirely verifies that writing
// to an append-only kind replaces its active entries with the entries parsed
// from the new content.
func TestWriteMemoryTool_OverwritesAppendOnlyKindEntirely(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- old entry to be wiped"); err != nil {
		t.Fatal(err)
	}

	tool := &WriteMemoryTool{Resolver: r, Stores: mgr}
	newContent := "## 2026-01-01\n- did X\n\n## 2026-01-02\n- did Y"
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress", "content": newContent}))
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
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after full overwrite, got %d", len(entries))
	}
	if entries[0].EntryDate != "2026-01-01" || entries[1].EntryDate != "2026-01-02" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

// TestWriteMemoryTool_ProjectNotFound verifies that writing to a nonexistent
// project returns an error that hints at create_project.
func TestWriteMemoryTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &WriteMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "nonexistent", "filename": "memory", "content": "x"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	_, err = tool.Execute(context.Background(), parsed)
	if err == nil {
		t.Fatal("expected an execution error for a nonexistent project")
	}
	if !strings.Contains(err.Error(), "create_project") {
		t.Fatalf("expected the error to hint at create_project, got: %v", err)
	}
}

// TestWriteMemoryTool_Validate verifies that validation rejects an invalid
// filename and empty content.
func TestWriteMemoryTool_Validate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &WriteMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "bad name!", "content": "x"})); err == nil {
		t.Fatal("expected a validation error for an invalid filename")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory", "content": ""})); err == nil {
		t.Fatal("expected a validation error for empty content")
	}
}

// TestWriteMemoryTool_KeepsArchivedEntries verifies that rewriting an append-
// only kind replaces only its active entries and keeps its archived entries.
func TestWriteMemoryTool_KeepsArchivedEntries(t *testing.T) {
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
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01", nil); err != nil {
		t.Fatal(err)
	}

	tool := &WriteMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "progress", "content": "## 2026-01-01\n- new"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].Archived || entries[1].Archived {
		t.Fatalf("entries = %+v, want the archived 2020 entry kept plus the new visible one", entries)
	}
}

// TestWriteMemoryTool_RefusesProjectFromLastSession verifies that
// write_memory refuses to overwrite a project taken only from the last
// session, and leaves its content unchanged.
func TestWriteMemoryTool_RefusesProjectFromLastSession(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "keep me"); err != nil {
		t.Fatal(err)
	}
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		*c = config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}
	}); err != nil {
		t.Fatal(err)
	}

	result := runTool(t, &WriteMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"filename": "memory", "content": "replaced"})
	if !result.IsError || !strings.Contains(result.Text, "Refusing to overwrite memory") {
		t.Fatalf("result = %+v, want a refusal", result)
	}
	if content, _, _ := s.ReadContent(ctx, "acme", "", "memory"); content != "keep me" {
		t.Fatalf("memory = %q, want it unchanged", content)
	}
}
