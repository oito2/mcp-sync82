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
	"slices"
	"strings"
	"testing"
)

// TestCreateProjectTool_TopLevel verifies that creating a top-level project
// reports it as created, and that repeating the call reports it as already
// existing.
func TestCreateProjectTool_TopLevel(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	tool := &CreateProjectTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Project "acme" created.`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	// A second call must report "already exists" instead of creating a duplicate.
	parsed2, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate (second call): %v", err)
	}
	result2, err := tool.Execute(ctx, parsed2)
	if err != nil {
		t.Fatalf("Execute (second call): %v", err)
	}
	if want := `Project "acme" already exists.`; result2.Text != want {
		t.Fatalf("Text = %q, want %q", result2.Text, want)
	}
}

// TestCreateProjectTool_SubprojectAutoCreatesParent verifies that creating a
// subproject also creates its missing parent project.
func TestCreateProjectTool_SubprojectAutoCreatesParent(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	tool := &CreateProjectTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "oito2", "subproject": "sync82"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Project "oito2/sync82" created.`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatalf("Get store: %v", err)
	}
	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatalf("FindProjectByName: %v", err)
	}
	if parent == nil {
		t.Fatal("expected the parent project to be auto-created")
	}
}

// TestCreateProjectTool_Validate_RequiresProject verifies that validation
// fails when the project is missing or whitespace-only.
func TestCreateProjectTool_Validate_RequiresProject(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CreateProjectTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{})); err == nil {
		t.Fatal("expected a validation error when project is missing")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "   "})); err == nil {
		t.Fatal("expected a validation error when project is blank/whitespace-only")
	}
}

// TestCreateProjectTool_Validate_RejectsMalformedNames verifies that
// validation rejects project and subproject names containing spaces or slashes
// or starting with a hyphen, and accepts a well-formed name.
func TestCreateProjectTool_Validate_RejectsMalformedNames(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &CreateProjectTool{Resolver: r, Stores: mgr}

	tests := []struct {
		name string
		args map[string]string
	}{
		{"project with space", map[string]string{"project": "my project"}},
		{"project with slash", map[string]string{"project": "my/project"}},
		{"project starting with hyphen", map[string]string{"project": "-acme"}},
		{"subproject with space", map[string]string{"project": "acme", "subproject": "my sub"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tool.Validate(mustJSON(t, tt.args)); err == nil {
				t.Fatal("expected a validation error for a malformed name")
			}
		})
	}

	// A well-formed name must still pass.
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "my-project_2", "subproject": "sub_1"})); err != nil {
		t.Fatalf("expected well-formed names to pass, got: %v", err)
	}
}

// TestNamesAreCaseInsensitive verifies that project and kind names are case-
// insensitive and stored lower-cased ("Acme"/"acme", "Memory"/"memory"), so
// that kinds cannot overwrite each other when exported to a case-insensitive
// filesystem such as macOS or Windows.
func TestNamesAreCaseInsensitive(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	create := &CreateProjectTool{Resolver: r, Stores: mgr}
	parsed, err := create.Validate(mustJSON(t, map[string]string{"project": "Acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result, err := create.Execute(ctx, parsed); err != nil || !strings.Contains(result.Text, `"acme"`) {
		t.Fatalf("create_project = %+v, %v; want the lower-cased name", result, err)
	}

	write := &WriteMemoryTool{Resolver: r, Stores: mgr}
	for _, args := range []map[string]string{
		{"project": "ACME", "filename": "Memory", "content": "first"},
		{"project": "acme", "filename": "memory", "content": "second"},
	} {
		parsed, err := write.Validate(mustJSON(t, args))
		if err != nil {
			t.Fatalf("Validate(%v): %v", args, err)
		}
		if result, err := write.Execute(ctx, parsed); err != nil || result.IsError {
			t.Fatalf("write_memory(%v) = %+v, %v", args, result, err)
		}
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if kinds, err := s.ListKinds(ctx, "acme", "", true); err != nil || !slices.Equal(kinds, []string{"memory"}) {
		t.Fatalf("kinds = %v, %v; want a single memory kind", kinds, err)
	}
	if content, ok, err := s.ReadContent(ctx, "Acme", "", "MEMORY"); err != nil || !ok || content != "second" {
		t.Fatalf("content = %q, %v, %v; want the last write", content, ok, err)
	}
}
