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

// TestDeleteProjectTool_Validate verifies that validation rejects a missing
// project, a missing or false confirm flag, an unknown subproject_action and
// malformed names, and accepts a fully valid call.
func TestDeleteProjectTool_Validate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing project", map[string]any{"confirm": true}},
		{"missing confirm", map[string]any{"project": "acme"}},
		{"confirm false", map[string]any{"project": "acme", "confirm": false}},
		{"invalid subproject_action", map[string]any{"project": "acme", "confirm": true, "subproject_action": "yolo"}},
		{"malformed project name", map[string]any{"project": "my project", "confirm": true}},
		{"malformed subproject name", map[string]any{"project": "acme", "subproject": "my sub", "confirm": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tool.Validate(mustJSON(t, tt.args)); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}

	// A fully valid call must pass.
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "confirm": true})); err != nil {
		t.Fatalf("expected valid arguments to pass, got: %v", err)
	}
}

// TestDeleteProjectTool_DeletesSubprojectDirectly verifies that deleting a
// named subproject removes only that subproject and keeps its parent.
func TestDeleteProjectTool_DeletesSubprojectDirectly(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "subproject": "sync82", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Subproject "oito2/sync82" deleted.`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil {
		t.Fatal("expected the parent project to survive")
	}
	subs, err := s.ListSubprojects(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected no subprojects left, got %+v", subs)
	}
}

// TestDeleteProjectTool_WithSubprojects_NoAction_AsksNotErrors verifies that
// deleting a project that has subprojects without a subproject_action returns
// a non-error message listing the subprojects and asking for the action, and
// deletes nothing.
func TestDeleteProjectTool_WithSubprojects_NoAction_AsksNotErrors(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatal("the ask-for-subproject_action message must not be an error result")
	}
	if !strings.Contains(result.Text, "sync82") || !strings.Contains(result.Text, "perci") {
		t.Fatalf("expected the message to list both subprojects, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "subproject_action") {
		t.Fatalf("expected the message to mention subproject_action, got: %s", result.Text)
	}

	// The project must not have been touched.
	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil {
		t.Fatal("expected the project to still exist — nothing should be deleted without an explicit action")
	}
}

// TestDeleteProjectTool_Cancel verifies that the "cancel" subproject_action
// reports the cancellation and leaves the project in place.
func TestDeleteProjectTool_Cancel(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "confirm": true, "subproject_action": "cancel"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Deletion of "oito2" cancelled.`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil {
		t.Fatal("expected the project to survive a cancelled deletion")
	}
}

// TestDeleteProjectTool_Promote verifies that the "promote" subproject_action
// deletes the project and turns each of its subprojects into an independent
// top-level project.
func TestDeleteProjectTool_Promote(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "confirm": true, "subproject_action": "promote"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "deleted") || !strings.Contains(result.Text, "promoted") {
		t.Fatalf("unexpected result text: %s", result.Text)
	}

	// oito2 itself must be gone and both subprojects must be top-level
	// projects.
	if p, err := s.FindProjectByName(ctx, "oito2", nil); err != nil {
		t.Fatal(err)
	} else if p != nil {
		t.Fatal("expected \"oito2\" to be deleted after promoting its subprojects")
	}
	for _, name := range []string{"sync82", "perci"} {
		p, err := s.FindProjectByName(ctx, name, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p == nil {
			t.Fatalf("expected %q to have been promoted to a top-level project", name)
		}
	}
}

// TestDeleteProjectTool_Promote_CollisionReportsErrorAndAbortsDeletion
// verifies that promoting a subproject whose name is already taken by a top-
// level project returns an error result reporting the aborted deletion, and
// keeps the original project.
func TestDeleteProjectTool_Promote_CollisionReportsErrorAndAbortsDeletion(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	// A top-level project already uses the name "sync82", so promoting the
	// subproject of the same name must collide with it.
	if _, _, err := s.EnsureProject(ctx, "sync82", ""); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "confirm": true, "subproject_action": "promote"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute returned a Go error instead of an IsError result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true when promotion collides")
	}
	if !strings.Contains(result.Text, "Deletion aborted") {
		t.Fatalf("expected the message to report the aborted deletion, got: %s", result.Text)
	}

	// oito2 must still exist because the deletion was aborted.
	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil {
		t.Fatal("expected \"oito2\" to survive an aborted deletion")
	}
}

// TestDeleteProjectTool_DeleteAll verifies that the "delete_all"
// subproject_action deletes the project together with its subprojects and
// mentions them in the result.
func TestDeleteProjectTool_DeleteAll(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "oito2", "confirm": true, "subproject_action": "delete_all"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "sync82") {
		t.Fatalf("expected the message to mention the cascaded subproject, got: %s", result.Text)
	}

	if p, err := s.FindProjectByName(ctx, "oito2", nil); err != nil {
		t.Fatal(err)
	} else if p != nil {
		t.Fatal("expected \"oito2\" to be deleted")
	}
}

// TestDeleteProjectTool_PlainDeleteNoSubprojects verifies that a confirmed
// delete of a project without subprojects succeeds without any
// subproject_action.
func TestDeleteProjectTool_PlainDeleteNoSubprojects(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := `Project "acme" deleted.`; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}
}

// TestDeleteProjectTool_NotFound_IsExecutionError verifies that deleting a
// project that does not exist returns an execution error.
func TestDeleteProjectTool_NotFound_IsExecutionError(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	tool := &DeleteProjectTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "nonexistent", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected an execution error for a project that doesn't exist")
	}
}
