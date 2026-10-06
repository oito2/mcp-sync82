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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestListProjectsTool_Empty verifies the message returned for a vault without
// projects.
func TestListProjectsTool_Empty(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &ListProjectsTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatal("expected IsError=false")
	}
	want := "No projects found. Use create_project to add one."
	if result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestListProjectsTool_WithSubprojects verifies that projects are listed
// alphabetically as a tree, with each project's subprojects nested under it.
func TestListProjectsTool_WithSubprojects(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatalf("Get store: %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	tool := &ListProjectsTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "Projects in vault:\n- acme\n- oito2\n  └─ perci\n  └─ sync82"
	if result.Text != want {
		t.Fatalf("Text =\n%q\nwant\n%q", result.Text, want)
	}
}

// TestListProjectsTool_CustomPathOverride verifies that the "path" argument
// makes the tool list the projects of that vault instead of the default one.
func TestListProjectsTool_CustomPathOverride(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	customPath := filepath.Join(t.TempDir(), "custom.db")
	s, err := mgr.Get(ctx, customPath)
	if err != nil {
		t.Fatalf("Get store: %v", err)
	}
	if _, _, err := s.EnsureProject(ctx, "custom-project", ""); err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}

	tool := &ListProjectsTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"path": customPath}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := "Projects in vault:\n- custom-project"
	if result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestReadOnlyTools_DoNotCreateMissingVault verifies that list_projects,
// read_memory and search_memory report a missing vault named by "path" instead
// of creating an empty vault file there, which would hide a mistyped path
// behind a "no projects" result.
func TestReadOnlyTools_DoNotCreateMissingVault(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "vualt.db")

	listTool := &ListProjectsTool{Resolver: r, Stores: mgr}
	parsed, err := listTool.Validate(mustJSON(t, map[string]string{"path": missing}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := listTool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("list_projects: %v", err)
	}
	if !strings.Contains(result.Text, missing) {
		t.Errorf("list_projects Text = %q, want it to name the missing vault", result.Text)
	}

	readTool := &ReadMemoryTool{Resolver: r, Stores: mgr}
	parsed, err = readTool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": "memory", "path": missing}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err = readTool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("read_memory: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "No vault exists") {
		t.Errorf("read_memory result = %+v, want a missing-vault error result", result)
	}

	searchTool := &SearchMemoryTool{Resolver: r, Stores: mgr}
	parsed, err = searchTool.Validate(mustJSON(t, map[string]string{"query": "x", "path": missing}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result, err := searchTool.Execute(ctx, parsed); err != nil || !result.IsError {
		t.Errorf("search_memory = %+v, %v; want a missing-vault error result", result, err)
	}

	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("a vault file was created at %s (err=%v)", missing, err)
	}
}

// TestListingTools_JSONFormat verifies the optional "format": "json" output of
// list_projects, list_files, check_project_health and search_memory: the text
// is a JSON document that is also returned as structured content,
// check_project_health still flags an unhealthy project as an error, and an
// unknown format is rejected.
func TestListingTools_JSONFormat(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", "api"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "needle here"); err != nil {
		t.Fatal(err)
	}

	run := func(tool Tool, args map[string]any) ToolResult {
		t.Helper()
		parsed, err := tool.Validate(mustJSON(t, args))
		if err != nil {
			t.Fatalf("%s Validate: %v", tool.Name(), err)
		}
		result, err := tool.Execute(ctx, parsed)
		if err != nil {
			t.Fatalf("%s Execute: %v", tool.Name(), err)
		}
		if result.Structured == nil {
			t.Fatalf("%s: no structured content", tool.Name())
		}
		return result
	}

	var projects projectList
	if err := json.Unmarshal([]byte(run(&ListProjectsTool{Resolver: r, Stores: mgr}, map[string]any{"format": "json"}).Text), &projects); err != nil {
		t.Fatal(err)
	}
	if len(projects.Projects) != 1 || projects.Projects[0].Name != "acme" || !slices.Equal(projects.Projects[0].Subprojects, []string{"api"}) {
		t.Errorf("list_projects json = %+v", projects)
	}

	var files fileList
	if err := json.Unmarshal([]byte(run(&ListFilesTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "metadata": true, "format": "json"}).Text), &files); err != nil {
		t.Fatal(err)
	}
	if len(files.Files) != 1 || files.Files[0].Name != "memory" || files.Files[0].SizeBytes == nil {
		t.Errorf("list_files json = %+v", files)
	}

	health := run(&CheckProjectHealthTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "format": "json"})
	var report healthReport
	if err := json.Unmarshal([]byte(health.Text), &report); err != nil {
		t.Fatal(err)
	}
	if report.Healthy || !report.Files["memory"] || report.Files["progress"] || !health.IsError {
		t.Errorf("check_project_health json = %+v (isError %v)", report, health.IsError)
	}

	var search searchReport
	if err := json.Unmarshal([]byte(run(&SearchMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"query": "needle", "format": "json"}).Text), &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Results) != 1 || search.Results[0].File != "memory" || search.Results[0].Line != 1 {
		t.Errorf("search_memory json = %+v", search)
	}

	if _, err := (&ListProjectsTool{Resolver: r, Stores: mgr}).Validate(mustJSON(t, map[string]any{"format": "xml"})); err == nil {
		t.Error("expected format \"xml\" to be rejected")
	}
}
