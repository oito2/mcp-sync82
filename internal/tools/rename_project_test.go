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

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestRenameProjectTool_TopLevel verifies that renaming a top-level project
// reports the rename and makes the project findable under its new name.
func TestRenameProjectTool_TopLevel(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "borg-82", ""); err != nil {
		t.Fatal(err)
	}

	tool := &RenameProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "borg-82", "new_name": "sync82"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Renamed "borg-82" to "sync82".`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	if p, err := s.FindProjectByName(ctx, "sync82", nil); err != nil || p == nil {
		t.Fatalf("expected the renamed project to be findable under its new name, got %+v, err %v", p, err)
	}
}

// TestRenameProjectTool_Subproject verifies that renaming a subproject keeps
// its parent and reports both qualified names.
func TestRenameProjectTool_Subproject(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "borg-82"); err != nil {
		t.Fatal(err)
	}

	tool := &RenameProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "oito2", "subproject": "borg-82", "new_name": "sync82"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Renamed "oito2/borg-82" to "oito2/sync82".`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestRenameProjectTool_CollisionIsBusinessErrorNotProtocolError verifies that
// renaming to a name already in use returns an error result (IsError) rather
// than a Go error.
func TestRenameProjectTool_CollisionIsBusinessErrorNotProtocolError(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}

	tool := &RenameProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "new_name": "oito2"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("expected a business-level result (IsError:true), not a protocol error, got: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError:true, got result: %+v", result)
	}
}

// TestRenameProjectTool_ProjectNotFound verifies that renaming a nonexistent
// project returns an execution error.
func TestRenameProjectTool_ProjectNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &RenameProjectTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "nonexistent", "new_name": "whatever"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(context.Background(), parsed); err == nil {
		t.Fatal("expected an execution error for a nonexistent project")
	}
}

// TestRenameProjectTool_Validate verifies that validation rejects a missing
// project or new_name and malformed project, subproject or new_name values,
// and accepts valid arguments.
func TestRenameProjectTool_Validate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &RenameProjectTool{Resolver: r, Stores: mgr}

	cases := []map[string]string{
		{"new_name": "sync82"},                                               // missing project
		{"project": "acme"},                                                  // missing new_name
		{"project": "bad name", "new_name": "sync82"},                        // invalid project format
		{"project": "acme", "new_name": "bad name"},                          // invalid new_name format
		{"project": "acme", "subproject": "bad name!", "new_name": "sync82"}, // invalid subproject format
	}
	for _, c := range cases {
		if _, err := tool.Validate(mustJSON(t, c)); err == nil {
			t.Errorf("Validate(%+v): expected an error, got none", c)
		}
	}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "new_name": "acme-2"})); err != nil {
		t.Errorf("Validate: expected valid args to pass, got: %v", err)
	}
}

// TestRenameProjectTool_UpdatesRememberedLastProject verifies that renaming
// the remembered last project updates the global config, so a later call
// resolved from it does not recreate the old name.
func TestRenameProjectTool_UpdatesRememberedLastProject(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}); err != nil {
		t.Fatal(err)
	}

	tool := &RenameProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "new_name": "acme2"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastProject != "acme2" {
		t.Fatalf("LastProject = %q, want %q", cfg.LastProject, "acme2")
	}
}
