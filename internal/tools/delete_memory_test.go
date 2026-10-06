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

// TestDeleteMemoryTool_Validate_ProtectsStandardKinds verifies that validation
// rejects deleting any standard kind.
func TestDeleteMemoryTool_Validate_ProtectsStandardKinds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &DeleteMemoryTool{Resolver: r, Stores: mgr}

	for _, k := range standardKinds {
		if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "filename": k})); err == nil {
			t.Errorf("expected a validation error protecting standard kind %q", k)
		}
	}
}

// TestDeleteMemoryTool_DeletesCustomKind verifies that a confirmed delete
// removes a custom kind and reports it.
func TestDeleteMemoryTool_DeletesCustomKind(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "custom-notes", "some content"); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "custom-notes", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if want := "Deleted: acme/custom-notes"; result.Text != want {
		t.Fatalf("Text = %q, want %q", result.Text, want)
	}

	exists, err := s.KindExists(ctx, "acme", "", "custom-notes")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected the custom kind to be gone")
	}
}

// TestDeleteMemoryTool_KindNotFound verifies that deleting a kind that does
// not exist returns an error.
func TestDeleteMemoryTool_KindNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "nonexistent", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err == nil {
		t.Fatal("expected an execution error for a kind that doesn't exist")
	}
}

// TestDeleteMemoryTool_RequiresConfirm verifies that validation rejects a
// delete without the confirm flag.
func TestDeleteMemoryTool_RequiresConfirm(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &DeleteMemoryTool{Resolver: r, Stores: mgr}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "notes"})); err == nil {
		t.Fatal("expected a validation error without confirm")
	}
}

// TestDeleteMemoryTool_RefusesProjectFromLastSession verifies that, with no
// project given, the tool returns an error result and deletes nothing instead
// of acting on the last-used project from the global config, which another
// session may have changed.
func TestDeleteMemoryTool_RefusesProjectFromLastSession(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "notes", "keep me"); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}); err != nil {
		t.Fatal(err)
	}

	tool := &DeleteMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{"filename": "notes", "confirm": true}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result = %+v, want an error result", result)
	}
	if _, ok, err := s.ReadContent(ctx, "acme", "", "notes"); err != nil || !ok {
		t.Fatalf("notes was deleted (ok=%v, err=%v)", ok, err)
	}
}
