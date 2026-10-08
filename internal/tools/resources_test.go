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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// TestParseResourceURI verifies which URIs name a resource and what they
// name, and that traversal, encoding and malformed URIs are rejected.
func TestParseResourceURI(t *testing.T) {
	valid := map[string]resourceTarget{
		"sync82://projects/acme/context":                          {project: "acme"},
		"sync82://projects/Acme/files/Progress":                   {project: "acme", file: "progress"},
		"sync82://projects/acme/files/context":                    {project: "acme", file: "context"},
		"sync82://projects/acme/subprojects/api/context":          {project: "acme", subproject: "api"},
		"sync82://projects/acme/subprojects/api/files/my_notes-2": {project: "acme", subproject: "api", file: "my_notes-2"},
	}
	for uri, want := range valid {
		if got, ok := parseResourceURI(uri); !ok || got != want {
			t.Errorf("parseResourceURI(%q) = %+v, %v; want %+v", uri, got, ok, want)
		}
	}
	for _, uri := range []string{
		"sync82://projects/acme",
		"sync82://projects/acme/",
		"sync82://projects//context",
		"sync82://projects/../context",
		"sync82://projects/%2e%2e/context",
		"sync82://projects/acme/files/..",
		"sync82://projects/acme/files/a%2Fb",
		"sync82://projects/acme/files/memory/extra",
		"sync82://projects/acme/context?x=1",
		"sync82://projects/acme/context#top",
		"sync82://projects/acme/subprojects/api",
		"sync82://projects/acme/subprojects/api/files",
		"sync82://projects/acme/other/memory",
		"file:///etc/passwd",
		"SYNC82://projects/acme/context",
	} {
		if got, ok := parseResourceURI(uri); ok {
			t.Errorf("parseResourceURI(%q) = %+v, want rejected", uri, got)
		}
	}
}

// newResourcesEnv returns Resources over a fresh default vault holding
// project acme (memory, progress) and acme/api (stack).
func newResourcesEnv(t *testing.T) (*Resources, *store.Store) {
	t.Helper()
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", "api"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "acme memory\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", "## 2026-01-01\n- did it"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "api", "stack", "Go"); err != nil {
		t.Fatal(err)
	}
	return &Resources{Resolver: r, Stores: mgr}, s
}

// TestResources_Read verifies the content of each kind of resource and
// that a missing project, subproject or file is ErrResourceNotFound.
func TestResources_Read(t *testing.T) {
	res, _ := newResourcesEnv(t)
	ctx := context.Background()
	read := func(uri string) (string, error) { return res.Read(ctx, uri) }

	text, err := read("sync82://projects/acme/context")
	if err != nil || !strings.HasPrefix(text, "# Context: acme\n\n## memory\n\nacme memory") || !strings.Contains(text, "- did it") {
		t.Errorf("context = %q, %v", text, err)
	}
	if text, err := read("sync82://projects/ACME/files/memory"); err != nil || text != "acme memory" {
		t.Errorf("memory file = %q, %v", text, err)
	}
	if text, err := read("sync82://projects/acme/subprojects/api/context"); err != nil || !strings.HasPrefix(text, "# Context: acme/api") || !strings.Contains(text, "Go") {
		t.Errorf("subproject context = %q, %v", text, err)
	}
	if text, err := read("sync82://projects/acme/subprojects/api/files/stack"); err != nil || text != "Go" {
		t.Errorf("subproject file = %q, %v", text, err)
	}
	for _, uri := range []string{
		"sync82://projects/missing/context",
		"sync82://projects/acme/files/missing",
		"sync82://projects/acme/subprojects/missing/context",
		"sync82://projects/acme/../context",
	} {
		if _, err := read(uri); !errors.Is(err, ErrResourceNotFound) {
			t.Errorf("Read(%q): err = %v, want ErrResourceNotFound", uri, err)
		}
	}
}

// TestResources_ListAndNoSideEffects verifies that List returns the
// context of every project and subproject, and that reading leaves the
// last used project unchanged.
func TestResources_ListAndNoSideEffects(t *testing.T) {
	res, _ := newResourcesEnv(t)
	ctx := context.Background()
	infos, err := res.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var uris []string
	for _, info := range infos {
		uris = append(uris, info.URI)
	}
	want := []string{"sync82://projects/acme/context", "sync82://projects/acme/subprojects/api/context"}
	if strings.Join(uris, " ") != strings.Join(want, " ") {
		t.Errorf("List = %q, want %q", uris, want)
	}
	if infos[1].Name != "acme/api-context" || !strings.Contains(infos[1].Description, "acme/api") {
		t.Errorf("subproject info = %+v", infos[1])
	}

	if _, err := res.Read(ctx, "sync82://projects/acme/context"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastProject != "" {
		t.Errorf("reading a resource set the last used project to %q", cfg.LastProject)
	}
}

// TestResources_MissingVault verifies that a missing default vault lists
// no resource, reads as not found, and is not created.
func TestResources_MissingVault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	vault := filepath.Join(t.TempDir(), "missing.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	res := &Resources{Resolver: NewResolver(vault, slog.New(slog.DiscardHandler)), Stores: mgr}

	if infos, err := res.List(context.Background()); err != nil || len(infos) != 0 {
		t.Errorf("List = %+v, %v; want none", infos, err)
	}
	if _, err := res.Read(context.Background(), "sync82://projects/acme/context"); !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("Read: err = %v, want ErrResourceNotFound", err)
	}
	if _, err := os.Stat(vault); !os.IsNotExist(err) {
		t.Errorf("the vault was created: %v", err)
	}
}

// TestPrompts_Render verifies the start_session prompt with a project and
// subproject and the end_session prompt without arguments, and that every
// prompt rejects a subproject without a project and malformed names.
func TestPrompts_Render(t *testing.T) {
	byName := map[string]PromptDefinition{}
	for _, p := range Prompts {
		byName[p.Name] = p
	}
	start, end := byName["start_session"], byName["end_session"]
	if start.Render == nil || end.Render == nil {
		t.Fatalf("prompts = %+v", Prompts)
	}

	text, err := start.Render(map[string]string{"project": "Acme", "subproject": "api"})
	if err != nil || !strings.Contains(text, `load_project_context with project "acme" and subproject "api"`) {
		t.Errorf("start_session = %q, %v", text, err)
	}
	text, err = end.Render(map[string]string{})
	if err != nil || !strings.Contains(text, "update_project_memory call, with workspace_root set to the current workspace folder") || !strings.Contains(text, "edit_entry") {
		t.Errorf("end_session = %q, %v", text, err)
	}
	for _, args := range []map[string]string{{"subproject": "api"}, {"project": "../x"}, {"project": "acme", "subproject": "a b"}} {
		for _, p := range Prompts {
			if _, err := p.Render(args); err == nil {
				t.Errorf("%s(%v) should be rejected", p.Name, args)
			}
		}
	}
}

// TestResources_Complete verifies the completion of project, subproject
// and file names, case-insensitive and by prefix, that an unknown argument
// or a subproject request without an existing project gives nothing, and
// that the number of returned values is capped at maxCompletionValues while
// the total still counts every match.
func TestResources_Complete(t *testing.T) {
	res, s := newResourcesEnv(t)
	ctx := context.Background()
	if _, _, err := s.EnsureProject(ctx, "acme-labs", ""); err != nil {
		t.Fatal(err)
	}
	complete := func(arg, value string, args map[string]string) ([]string, int) {
		t.Helper()
		values, total, err := res.Complete(ctx, arg, value, args)
		if err != nil {
			t.Fatalf("Complete(%s, %q): %v", arg, value, err)
		}
		return values, total
	}

	if got, total := complete("project", "AC", nil); strings.Join(got, ",") != "acme,acme-labs" || total != 2 {
		t.Errorf("project AC = %q (%d)", got, total)
	}
	if got, _ := complete("subproject", "", map[string]string{"project": "Acme"}); strings.Join(got, ",") != "api" {
		t.Errorf("subproject = %q", got)
	}
	if got, _ := complete("file", "p", map[string]string{"project": "acme"}); strings.Join(got, ",") != "progress" {
		t.Errorf("file p = %q", got)
	}
	if got, _ := complete("file", "s", map[string]string{"project": "acme", "subproject": "api"}); strings.Join(got, ",") != "stack" {
		t.Errorf("subproject file s = %q", got)
	}
	if got, _ := complete("file", "", nil); len(got) != len(standardKinds) {
		t.Errorf("file without project = %q, want the standard kinds", got)
	}
	for _, c := range []struct {
		arg  string
		args map[string]string
	}{{"other", nil}, {"subproject", nil}, {"subproject", map[string]string{"project": "ghost"}}} {
		if got, total := complete(c.arg, "", c.args); len(got) != 0 || total != 0 {
			t.Errorf("Complete(%s, %v) = %q", c.arg, c.args, got)
		}
	}

	if err := s.WriteDocument(ctx, "acme", "", "notes", "x"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxCompletionValues+5; i++ {
		if _, _, err := s.EnsureProject(ctx, fmt.Sprintf("bulk-%03d", i), ""); err != nil {
			t.Fatal(err)
		}
	}
	if got, total := complete("project", "bulk", nil); len(got) != maxCompletionValues || total != maxCompletionValues+5 {
		t.Errorf("bulk completion = %d values of %d, want %d of %d", len(got), total, maxCompletionValues, maxCompletionValues+5)
	}
}
