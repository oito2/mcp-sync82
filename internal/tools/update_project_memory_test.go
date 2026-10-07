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
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestUpdateProjectMemoryTool_NothingToUpdate verifies that a call without any
// content returns a non-error "Nothing to update" message.
func TestUpdateProjectMemoryTool_NothingToUpdate(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatal("expected IsError=false for the informational 'nothing to update' message")
	}
	if !strings.Contains(result.Text, "Nothing to update") {
		t.Fatalf("unexpected message: %s", result.Text)
	}
}

// TestUpdateProjectMemoryTool_MixedAppendAndOverwrite verifies that one call
// can append to a log kind (progress) and overwrite a document (memory), and
// that the result reports each operation.
func TestUpdateProjectMemoryTool_MixedAppendAndOverwrite(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{
		"project":  "acme",
		"progress": "## 2026-07-22\n- did phase 8",
		"memory":   "# Memory\nupdated",
	}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false, got text: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Appended: progress") {
		t.Errorf("expected progress reported as appended, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Overwritten: memory") {
		t.Errorf("expected memory reported as overwritten, got: %s", result.Text)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 progress entry, got %d", len(entries))
	}
	content, _, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if content != "# Memory\nupdated" {
		t.Fatalf("memory content = %q", content)
	}
}

// TestUpdateProjectMemoryTool_InvalidFieldWritesNothing verifies that a
// field its write would reject (progress without a date header) makes
// Validate fail with every problem listed, so the valid fields of the same
// call are not written either.
func TestUpdateProjectMemoryTool_InvalidFieldWritesNothing(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}
	_, err := tool.Validate(mustJSON(t, map[string]any{
		"project":   "acme",
		"progress":  "no date header here",
		"decisions": "## 2026-01-01\n- fine",
		"stack":     "",
		"memory":    "# Memory\nwould be written",
	}))
	if err == nil {
		t.Fatal("expected a validation error")
	}
	for _, want := range []string{"nothing was written", "progress:", "stack:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "decisions:") || strings.Contains(err.Error(), "memory:") {
		t.Errorf("valid fields reported as problems: %q", err)
	}
}

// TestUpdateProjectMemoryTool_StoreRejectionWritesNothing verifies that a
// field rejected only when the store is checked (an append to a custom kind
// stored as a document) is an error result and leaves every other field of
// the call unwritten.
func TestUpdateProjectMemoryTool_StoreRejectionWritesNothing(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "notes", "a document"); err != nil {
		t.Fatal(err)
	}
	result := runTool(t, &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}, map[string]any{
		"project": "acme",
		"memory":  "# Memory\nwould be written",
		"custom":  []map[string]string{{"filename": "notes", "content": "an appended note"}},
	})
	if !result.IsError || !strings.Contains(result.Text, "Nothing was written") || !strings.Contains(result.Text, "notes:") {
		t.Fatalf("result = %+v, want an error result naming notes", result)
	}
	if _, ok, _ := s.ReadDocument(ctx, "acme", "", "memory"); ok {
		t.Fatal("memory was written by a call that failed")
	}
}

// TestUpdateProjectMemoryTool_CustomItemsDefaultAppendAndExplicitWrite
// verifies that a custom item is appended by default and overwritten when its
// mode is "write", and that each is stored accordingly (entry versus
// document).
func TestUpdateProjectMemoryTool_CustomItemsDefaultAppendAndExplicitWrite(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]any{
		"project": "acme",
		"custom": []map[string]any{
			{"filename": "api", "content": "api notes, no date needed for custom append"},
			{"filename": "testing", "content": "testing doc content", "mode": "write"},
		},
	}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected IsError=false, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Appended: api") {
		t.Errorf("expected api reported as appended (default mode), got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "Overwritten: testing") {
		t.Errorf("expected testing reported as overwritten (mode=write), got: %s", result.Text)
	}

	entries, err := s.ReadEntries(ctx, "acme", "", "api", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected api to be stored as an entry, got %d", len(entries))
	}
	content, ok, err := s.ReadDocument(ctx, "acme", "", "testing")
	if err != nil || !ok || content != "testing doc content" {
		t.Fatalf("expected testing to be stored as a document: ok=%v content=%q err=%v", ok, content, err)
	}
}

// TestUpdateProjectMemoryTool_Validate_RejectsInvalidCustomMode verifies that
// validation rejects a custom item whose mode is neither append nor write.
func TestUpdateProjectMemoryTool_Validate_RejectsInvalidCustomMode(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}

	_, err := tool.Validate(mustJSON(t, map[string]any{
		"project": "acme",
		"custom":  []map[string]any{{"filename": "api", "content": "x", "mode": "delete"}},
	}))
	if err == nil {
		t.Fatal("expected a validation error for an invalid custom mode")
	}
}

// TestUpdateProjectMemoryTool_NeedsInput verifies that, when no project can be
// resolved, the tool returns NeedsInputMessage.
func TestUpdateProjectMemoryTool_NeedsInput(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]any{"memory": "content but no project resolvable"}))
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
}

// TestUpdateProjectMemoryTool_ExplicitEmptyStringStillCountsAsProvided
// verifies that a field explicitly sent as an empty string counts as
// provided: Validate rejects it as empty instead of treating it as absent.
func TestUpdateProjectMemoryTool_ExplicitEmptyStringStillCountsAsProvided(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}
	_, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "memory": ""}))
	if err == nil || !strings.Contains(err.Error(), "memory:") {
		t.Fatalf("err = %v, want the empty memory reported", err)
	}
}

// TestUpdateProjectMemoryTool_CustomRejectsStandardKinds verifies that
// validation rejects a custom item named after a standard kind, which would
// bypass that kind's own rules and could make writes invisible or overwrite
// the progress log.
func TestUpdateProjectMemoryTool_CustomRejectsStandardKinds(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}
	for _, kind := range []string{"memory", "progress"} {
		_, err := tool.Validate(mustJSON(t, map[string]any{
			"project": "acme",
			"custom":  []map[string]string{{"filename": kind, "content": "x", "mode": "write"}},
		}))
		if err == nil {
			t.Errorf("expected a validation error for custom item %q", kind)
		}
	}
}

// TestCustomKind_KeepsItsStorageMode verifies that a custom kind keeps the
// storage mode it was created with: appends after a rewrite stay readable
// together with the rewritten content, and appending to a kind created as a
// document fails with errAppendToDocumentKind.
func TestCustomKind_KeepsItsStorageMode(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	if err := appendMemoryCore(ctx, s, "acme", "", "notes", "first"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := writeMemoryCore(ctx, s, "acme", "", "notes", "rewritten"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := appendMemoryCore(ctx, s, "acme", "", "notes", "later"); err != nil {
		t.Fatalf("append after write: %v", err)
	}
	content, ok, err := s.ReadContent(ctx, "acme", "", "notes")
	if err != nil || !ok {
		t.Fatalf("ReadContent: ok=%v err=%v", ok, err)
	}
	if content != "rewritten\n\nlater" {
		t.Fatalf("notes = %q, want %q", content, "rewritten\n\nlater")
	}

	// A custom kind created as a document refuses appends instead of
	// storing an entry that nothing reads.
	if err := writeMemoryCore(ctx, s, "acme", "", "spec", "the spec"); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := appendMemoryCore(ctx, s, "acme", "", "spec", "more"); !errors.Is(err, errAppendToDocumentKind) {
		t.Fatalf("append to document kind: err = %v, want errAppendToDocumentKind", err)
	}
}

// TestUpdateProjectMemoryTool_BoundsOneCall verifies that validation rejects a
// call with more than maxCustomItems custom items or with total content above
// the combined size limit.
func TestUpdateProjectMemoryTool_BoundsOneCall(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}

	items := make([]map[string]string, maxCustomItems+1)
	for i := range items {
		items[i] = map[string]string{"filename": "notes" + strconv.Itoa(i), "content": "x"}
	}
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "custom": items})); err == nil {
		t.Error("expected too many custom items to be rejected")
	}

	half := strings.Repeat("x", maxContentSize/2+1)
	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "memory": half, "architecture": half})); err == nil {
		t.Error("expected content over the total limit to be rejected")
	}
}

// TestUpdateProjectMemoryTool_LastSessionAllowsOnlyAppends verifies that a
// project taken only from the last session accepts a call that only
// appends, and refuses one that overwrites, writing nothing at all.
func TestUpdateProjectMemoryTool_LastSessionAllowsOnlyAppends(t *testing.T) {
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
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme", LastVaultPath: r.DefaultDBPath}); err != nil {
		t.Fatal(err)
	}
	tool := &UpdateProjectMemoryTool{Resolver: r, Stores: mgr}

	result := runTool(t, tool, map[string]any{"progress": "## 2026-01-01\n- appended", "memory": "replaced"})
	if !result.IsError || !strings.Contains(result.Text, "Refusing to overwrite memory") {
		t.Fatalf("result = %+v, want a refusal", result)
	}
	if content, _, _ := s.ReadContent(ctx, "acme", "", "memory"); content != "keep me" {
		t.Fatalf("memory = %q, want it unchanged", content)
	}
	if _, ok, _ := s.ReadContent(ctx, "acme", "", "progress"); ok {
		t.Fatal("progress was written by a refused call")
	}

	result = runTool(t, tool, map[string]any{"progress": "## 2026-01-01\n- appended"})
	if result.IsError || !strings.Contains(result.Text, "Appended: progress") {
		t.Fatalf("append-only call = %+v, want it accepted", result)
	}
}
