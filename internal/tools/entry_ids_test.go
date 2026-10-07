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
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/store"
)

// TestStripEntryMarkers verifies that only whole marker lines are removed,
// with their line breaks, and that markers inside other text are kept.
func TestStripEntryMarkers(t *testing.T) {
	cases := map[string]string{
		"<!-- entry:12 -->\n## 2026-01-01\n- a":                      "## 2026-01-01\n- a",
		"<!-- entry:1 -->\r\nx\n\n  <!-- entry:22 -->  \ny":          "x\n\ny",
		"text <!-- entry:3 --> inline":                               "text <!-- entry:3 --> inline",
		"<!-- entry:abc -->\nnot a marker":                           "<!-- entry:abc -->\nnot a marker",
		"ends with a marker\n<!-- entry:9 -->":                       "ends with a marker\n",
		"## 2026-01-01\n- a\n\n<!-- entry:5 -->\n## 2026-01-02\n- b": "## 2026-01-01\n- a\n\n## 2026-01-02\n- b",
		"no markers at all":                                          "no markers at all",
	}
	for in, want := range cases {
		if got := stripEntryMarkers(in); got != want {
			t.Errorf("stripEntryMarkers(%q) = %q, want %q", in, got, want)
		}
	}
}

// seedLog creates project acme in the default vault of r with two dated
// progress entries and returns the store and their ids, oldest first.
func seedLog(t *testing.T, r *Resolver, mgr *store.Manager) (*store.Store, int64, int64) {
	t.Helper()
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"2026-01-01", "2026-02-01"} {
		if err := s.AppendEntry(ctx, "acme", "", "progress", d, "## "+d+"\n- entry "+d); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := s.ReadEntries(ctx, "acme", "", "progress", false)
	if err != nil || len(entries) != 2 {
		t.Fatalf("ReadEntries = %+v, %v", entries, err)
	}
	return s, entries[0].ID, entries[1].ID
}

// runTool validates args with tool and executes it, failing the test on
// any error, and returns the result.
func runTool(t *testing.T, tool Tool, args map[string]any) ToolResult {
	t.Helper()
	parsed, err := tool.Validate(mustJSON(t, args))
	if err != nil {
		t.Fatalf("Validate(%v): %v", args, err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute(%v): %v", args, err)
	}
	return result
}

// TestReadMemoryTool_WithIDsMarksEachEntry verifies that with_ids puts a
// marker line with the entry id before each entry, and that writing the
// result back stores the same entries without the markers.
func TestReadMemoryTool_WithIDsMarksEachEntry(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, first, second := seedLog(t, r, mgr)

	text := runTool(t, &ReadMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress", "with_ids": true}).Text
	want := entryMarker(first) + "\n## 2026-01-01\n- entry 2026-01-01\n\n" + entryMarker(second) + "\n## 2026-02-01\n- entry 2026-02-01"
	if text != want {
		t.Fatalf("read_memory with_ids = %q, want %q", text, want)
	}

	runTool(t, &WriteMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress", "content": text})
	content, ok, err := s.ReadContent(context.Background(), "acme", "", "progress")
	if err != nil || !ok || content != "## 2026-01-01\n- entry 2026-01-01\n\n## 2026-02-01\n- entry 2026-02-01" {
		t.Fatalf("after writing back, content = %q (ok=%v, err=%v)", content, ok, err)
	}
}

// TestReadMemoryTool_WithIDsIgnoredForDocuments verifies that with_ids does
// not change an overwrite-style document.
func TestReadMemoryTool_WithIDsIgnoredForDocuments(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, _, _ := seedLog(t, r, mgr)
	if err := s.WriteDocument(context.Background(), "acme", "", "memory", "state"); err != nil {
		t.Fatal(err)
	}
	if text := runTool(t, &ReadMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "memory", "with_ids": true}).Text; text != "state" {
		t.Fatalf("read_memory with_ids on a document = %q, want %q", text, "state")
	}
}

// TestAppendAndImport_StripEntryMarkers verifies that append_memory and
// import_memory never store entry id marker lines.
func TestAppendAndImport_StripEntryMarkers(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	s, _, _ := seedLog(t, r, mgr)
	ctx := context.Background()

	runTool(t, &AppendMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "filename": "progress", "content": "<!-- entry:7 -->\n## 2026-03-01\n- appended"})
	content, _, err := s.ReadContent(ctx, "acme", "", "progress")
	if err != nil || strings.Contains(content, "<!-- entry:") {
		t.Fatalf("append stored a marker: %q, %v", content, err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "decisions.md"), []byte("<!-- entry:3 -->\n## 2026-01-01\n- imported"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTool(t, &ImportMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "input_dir": dir})
	content, _, err = s.ReadContent(ctx, "acme", "", "decisions")
	if err != nil || content != "## 2026-01-01\n- imported" {
		t.Fatalf("import stored %q, %v; want the marker removed", content, err)
	}
}

// TestSearchMemoryTool_JSONHitsCarryEntryID verifies that search_memory's
// JSON output gives the entry id of a match in a log.
func TestSearchMemoryTool_JSONHitsCarryEntryID(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	_, _, second := seedLog(t, r, mgr)

	text := runTool(t, &SearchMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"project": "acme", "query": "entry 2026-02-01", "match": "exact", "format": "json"}).Text
	var report struct {
		Results []struct {
			EntryID int64 `json:"entry_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("unmarshal %q: %v", text, err)
	}
	if len(report.Results) != 1 || report.Results[0].EntryID != second {
		t.Fatalf("results = %+v, want one hit with entry_id %d", report.Results, second)
	}
}
