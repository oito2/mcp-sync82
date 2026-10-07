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
)

// TestListFilesTool_NoFiles verifies the message returned for a project
// without any kind.
func TestListFilesTool_NoFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &ListFilesTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `No files found in project "acme".`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestListFilesTool_ListsKindsAndMetadata verifies that the tool lists
// document and log kinds by name, and adds token and modification-time details
// when metadata is requested.
func TestListFilesTool_ListsKindsAndMetadata(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "# Memory"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- did X"); err != nil {
		t.Fatal(err)
	}

	tool := &ListFilesTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "- memory") || !strings.Contains(result.Text, "- progress") {
		t.Fatalf("expected both kinds listed, got: %s", result.Text)
	}

	parsedMeta, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "metadata": true}))
	if err != nil {
		t.Fatalf("Validate (metadata): %v", err)
	}
	resultMeta, err := tool.Execute(ctx, parsedMeta)
	if err != nil {
		t.Fatalf("Execute (metadata): %v", err)
	}
	if !strings.Contains(resultMeta.Text, "tokens") || !strings.Contains(resultMeta.Text, "modified:") {
		t.Fatalf("expected metadata fields in output, got: %s", resultMeta.Text)
	}
}

// TestListFilesTool_NeedsInput verifies that, with no project resolvable, the
// tool returns NeedsInputMessage as a non-error result.
func TestListFilesTool_NeedsInput(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ListFilesTool{Resolver: r, Stores: mgr}

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

// TestListFilesTool_MetadataWithFullyArchivedKind verifies that a kind whose
// entries are all archived does not make the metadata listing fail, and is
// omitted from the output.
func TestListFilesTool_MetadataWithFullyArchivedKind(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "content"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- old"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01", nil); err != nil {
		t.Fatal(err)
	}

	tool := &ListFilesTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "metadata": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "memory") || strings.Contains(result.Text, "progress") {
		t.Fatalf("Text = %q, want memory listed and the fully archived progress omitted", result.Text)
	}
}
