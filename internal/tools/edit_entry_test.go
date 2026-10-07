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
	"strings"
	"testing"
	"time"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestEditEntryTool_Replace verifies that replace rewrites the entry in
// place, takes the new date from the content's header, and accepts content
// read with entry id markers.
func TestEditEntryTool_Replace(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, second := seedLog(t, r, mgr)
	tool := &EditEntryTool{Resolver: r, Stores: mgr}

	result := runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "entry_id": first, "action": "replace",
		"content": entryMarker(first) + "\n## 2026-01-01\n- entry fixed"})
	if result.IsError || !strings.HasPrefix(result.Text, fmt.Sprintf("Replaced entry %d in acme/progress", first)) {
		t.Fatalf("result = %+v", result)
	}
	e, err := s.ReadEntry(context.Background(), "acme", "", "progress", first)
	if err != nil || e.Body != "## 2026-01-01\n- entry fixed" || e.EntryDate != "2026-01-01" {
		t.Fatalf("entry after replace = %+v, %v", e, err)
	}

	runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "entry_id": first, "action": "replace", "content": "## 2026-03-01\n- moved"})
	entries, err := s.ReadEntries(context.Background(), "acme", "", "progress", false)
	if err != nil || len(entries) != 2 || entries[0].ID != second || entries[1].EntryDate != "2026-03-01" {
		t.Fatalf("entries after a date change = %+v, %v", entries, err)
	}
}

// TestEditEntryTool_Supersede verifies that supersede appends the new entry
// and marks the old one with the new id and today's date.
func TestEditEntryTool_Supersede(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, _ := seedLog(t, r, mgr)
	tool := &EditEntryTool{Resolver: r, Stores: mgr}

	result := runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "entry_id": first, "action": "supersede", "content": "## 2026-02-15\n- new decision"})
	entries, err := s.ReadEntries(context.Background(), "acme", "", "progress", false)
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	newEntry := entries[2]
	if newEntry.EntryDate != "2026-02-15" || newEntry.Body != "## 2026-02-15\n- new decision" {
		t.Fatalf("new entry = %+v", newEntry)
	}
	if !strings.HasPrefix(result.Text, fmt.Sprintf("Superseded entry %d with new entry %d in acme/progress", first, newEntry.ID)) {
		t.Fatalf("result = %q", result.Text)
	}
	today := time.Now().UTC().Format("2006-01-02")
	want := fmt.Sprintf("## 2026-01-01\n- entry 2026-01-01\n\n> Superseded by entry %d on %s.", newEntry.ID, today)
	if entries[0].Body != want {
		t.Fatalf("old entry = %q, want %q", entries[0].Body, want)
	}
}

// TestEditEntryTool_Delete verifies that delete requires confirm and then
// removes only that entry.
func TestEditEntryTool_Delete(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, second := seedLog(t, r, mgr)
	tool := &EditEntryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "progress", "entry_id": first, "action": "delete"})); err == nil {
		t.Fatal("delete without confirm should be rejected")
	}
	runTool(t, tool, map[string]any{"project": "acme", "filename": "progress", "entry_id": first, "action": "delete", "confirm": true})
	entries, err := s.ReadEntries(context.Background(), "acme", "", "progress", true)
	if err != nil || len(entries) != 1 || entries[0].ID != second {
		t.Fatalf("entries after delete = %+v, %v", entries, err)
	}
}

// TestEditEntryTool_EntryOfAnotherProjectIsNotFound verifies that an entry
// id is only accepted within its own project and kind, as an error result
// that changes nothing.
func TestEditEntryTool_EntryOfAnotherProjectIsNotFound(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, _ := seedLog(t, r, mgr)
	ctx := context.Background()
	if _, _, err := s.EnsureProject(ctx, "other", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "other", "", "progress", "2026-01-01", "## 2026-01-01\n- other"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "notes", "", "a custom log entry"); err != nil {
		t.Fatal(err)
	}
	tool := &EditEntryTool{Resolver: r, Stores: mgr}

	for _, args := range []map[string]any{
		{"project": "other", "filename": "progress", "entry_id": first, "action": "delete", "confirm": true},
		{"project": "acme", "filename": "notes", "entry_id": first, "action": "replace", "content": "x"},
		{"project": "acme", "filename": "progress", "entry_id": 999999, "action": "supersede", "content": "## 2026-01-01\n- x"},
	} {
		result := runTool(t, tool, args)
		if !result.IsError || !strings.Contains(result.Text, "not found") || !strings.Contains(result.Text, "with_ids") {
			t.Errorf("%v: result = %+v, want a not-found error result", args, result)
		}
	}
	if e, err := s.ReadEntry(ctx, "acme", "", "progress", first); err != nil || e.Body != "## 2026-01-01\n- entry 2026-01-01" {
		t.Fatalf("entry changed: %+v, %v", e, err)
	}
}

// TestEditEntryTool_RefusesProjectFromLastSession verifies that the last
// used project is refused and the entry is left unchanged.
func TestEditEntryTool_RefusesProjectFromLastSession(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, _ := seedLog(t, r, mgr)
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}); err != nil {
		t.Fatal(err)
	}
	result := runTool(t, &EditEntryTool{Resolver: r, Stores: mgr}, map[string]any{"filename": "progress", "entry_id": first, "action": "delete", "confirm": true})
	if !result.IsError || !strings.Contains(result.Text, "Refusing") {
		t.Fatalf("result = %+v, want a refusal", result)
	}
	if _, err := s.ReadEntry(context.Background(), "acme", "", "progress", first); err != nil {
		t.Fatalf("entry was deleted: %v", err)
	}
}

// TestEditEntryTool_DocumentKindIsRejected verifies that an overwrite-style
// kind is rejected, by Validate for a standard kind and by Execute for a
// custom kind stored as a document.
func TestEditEntryTool_DocumentKindIsRejected(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, _ := seedLog(t, r, mgr)
	if err := s.WriteDocument(context.Background(), "acme", "", "notes", "a document"); err != nil {
		t.Fatal(err)
	}
	tool := &EditEntryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "filename": "memory", "entry_id": first, "action": "delete", "confirm": true})); err == nil {
		t.Error("a standard overwrite-style kind should be rejected")
	}
	result := runTool(t, tool, map[string]any{"project": "acme", "filename": "notes", "entry_id": first, "action": "replace", "content": "x"})
	if !result.IsError || !strings.Contains(result.Text, "write_memory") {
		t.Errorf("result = %+v, want an error result pointing to write_memory", result)
	}
}

// TestEditEntryTool_Validate rejects invalid argument combinations.
func TestEditEntryTool_Validate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &EditEntryTool{Resolver: r, Stores: mgr}
	for name, args := range map[string]map[string]any{
		"no entry_id":              {"project": "acme", "filename": "progress", "action": "delete", "confirm": true},
		"negative entry_id":        {"project": "acme", "filename": "progress", "entry_id": -1, "action": "delete", "confirm": true},
		"unknown action":           {"project": "acme", "filename": "progress", "entry_id": 1, "action": "move"},
		"replace without content":  {"project": "acme", "filename": "progress", "entry_id": 1, "action": "replace"},
		"replace without date":     {"project": "acme", "filename": "progress", "entry_id": 1, "action": "replace", "content": "- no header"},
		"only a marker as content": {"project": "acme", "filename": "notes", "entry_id": 1, "action": "supersede", "content": "<!-- entry:1 -->\n"},
		"delete with content":      {"project": "acme", "filename": "progress", "entry_id": 1, "action": "delete", "confirm": true, "content": "x"},
	} {
		if _, err := tool.Validate(mustJSON(t, args)); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}
