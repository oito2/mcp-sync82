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

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWrites_RacingAProjectDeleteFailCleanly verifies that writes racing
// the delete and re-creation of their project either succeed or fail with
// ErrNotFound, never with a raw foreign-key error: the project is resolved
// inside each write's own transaction.
func TestWrites_RacingAProjectDeleteFailCleanly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 400)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			_ = s.DeleteProject(ctx, "acme")
			_, _, _ = s.EnsureProject(ctx, "acme", "")
		}
	}()
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				errs <- s.AppendEntry(ctx, "acme", "", "progress", "2026-01-01", fmt.Sprintf("## 2026-01-01\n- %d/%d", w, i))
				errs <- s.WriteDocument(ctx, "acme", "", "memory", "state")
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatalf("a write racing the delete failed with %v, want nil or ErrNotFound", err)
		}
	}
}

// TestKindModes_MatchesKindMode verifies that KindModes reports, for every
// kind, what KindMode reports, including a kind stored in both tables and
// one whose entries are all archived.
func TestKindModes_MatchesKindMode(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "doc"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "progress", "2020-01-01", "## 2020-01-01\n- old"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ArchiveEntries(ctx, "acme", "", "progress", "2021-01-01", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "both", "doc"); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "both", "", "entry"); err != nil {
		t.Fatal(err)
	}

	modes, err := s.KindModes(ctx, "acme", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"memory", "progress", "both", "missing"} {
		want, err := s.KindMode(ctx, "acme", "", kind)
		if err != nil {
			t.Fatal(err)
		}
		if modes[kind] != want {
			t.Errorf("KindModes[%q] = %q, KindMode = %q", kind, modes[kind], want)
		}
	}
	if _, err := s.KindModes(ctx, "ghost", ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("KindModes on a missing project: err = %v, want ErrNotFound", err)
	}
}

// TestWriteKinds_RollsBackWhenALaterWriteFails verifies that a failing
// write in a WriteKinds batch leaves the earlier writes of the batch
// unapplied. The failure comes from a temporary trigger on the store's
// single connection.
func TestWriteKinds_RollsBackWhenALaterWriteFails(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TEMP TRIGGER fail_boom BEFORE INSERT ON entries
		WHEN NEW.kind = 'boom' BEGIN SELECT RAISE(ABORT, 'boom refused'); END`); err != nil {
		t.Fatal(err)
	}
	doc := "should not stay"
	err := s.WriteKinds(ctx, "acme", "", []KindWrite{
		{Kind: "memory", Document: &doc},
		{Kind: "boom", Append: true, Sections: []EntrySection{{Body: "x"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "boom refused") {
		t.Fatalf("WriteKinds err = %v, want the trigger's error", err)
	}
	if _, ok, _ := s.ReadDocument(ctx, "acme", "", "memory"); ok {
		t.Fatal("the first write of a failed batch was kept")
	}
}

// TestDeleteSubproject removes only the named subproject and reports a
// missing parent or subproject as ErrNotFound.
func TestDeleteSubproject(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, sub := range []string{"api", "web"} {
		if _, _, err := s.EnsureProject(ctx, "acme", sub); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteSubproject(ctx, "ACME", "Api"); err != nil {
		t.Fatalf("DeleteSubproject: %v", err)
	}
	if exists, _ := s.ProjectExists(ctx, "acme", "api"); exists {
		t.Error("api still exists")
	}
	if exists, _ := s.ProjectExists(ctx, "acme", "web"); !exists {
		t.Error("web was deleted too")
	}
	for _, c := range [][2]string{{"acme", "api"}, {"ghost", "api"}} {
		if err := s.DeleteSubproject(ctx, c[0], c[1]); !errors.Is(err, ErrNotFound) {
			t.Errorf("DeleteSubproject(%s/%s): err = %v, want ErrNotFound", c[0], c[1], err)
		}
	}
}

// TestManager_GetExisting refuses a missing vault without creating it and
// opens an existing one.
func TestManager_GetExisting(t *testing.T) {
	ctx := context.Background()
	// The directory is created before the Close cleanup is registered, so
	// the vault is closed before the directory is removed.
	path := filepath.Join(t.TempDir(), "vault.db")
	m := NewManager()
	t.Cleanup(func() { m.Close() })

	if _, err := m.GetExisting(ctx, path); !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf("GetExisting on a missing vault: err = %v, want ErrVaultNotFound", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("GetExisting created the vault: %v", err)
	}
	created, err := m.Get(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.GetExisting(ctx, path)
	if err != nil || got != created {
		t.Fatalf("GetExisting on an existing vault = %p, %v; want %p", got, err, created)
	}
}

// TestEscapeLike checks that LIKE metacharacters and the escape character
// are escaped.
func TestEscapeLike(t *testing.T) {
	for in, want := range map[string]string{
		`100%`:      `100\%`,
		`a_b`:       `a\_b`,
		`back\path`: `back\\path`,
		`plain`:     `plain`,
	} {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFTSQuery checks the FTS5 expressions built for words and phrase
// queries: folded, quoted terms with a trailing * kept as a prefix.
func TestFTSQuery(t *testing.T) {
	cases := []struct {
		query string
		mode  SearchMode
		want  string
	}{
		{"Sessão instal*", SearchWords, `"sessao" "instal"*`},
		{`edit_entry "NEAR"`, SearchWords, `"edit" "entry" "near"`},
		{"sobre o instal*", SearchPhrase, `"sobre o instal"*`},
		{"config file", SearchPhrase, `"config file"`},
	}
	for _, c := range cases {
		if got := ftsQuery(parseSearchTerms(c.query), c.mode); got != c.want {
			t.Errorf("ftsQuery(%q, %s) = %s, want %s", c.query, c.mode, got, c.want)
		}
	}
}

// lockNewVault creates an empty vault file and holds an exclusive lock on
// it through a separate rollback-journal connection, which a WAL switch
// reports as SQLITE_BUSY. The returned function releases the lock.
func lockNewVault(t *testing.T, path string) func() {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=locking_mode(EXCLUSIVE)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	for _, q := range []string{"CREATE TABLE IF NOT EXISTS lock_holder (x)", "INSERT INTO lock_holder VALUES (1)"} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	var once sync.Once
	release := func() { once.Do(func() { db.Close() }) }
	t.Cleanup(release)
	return release
}

// TestOpen_WaitsForALockedVault verifies that Open waits while another
// connection holds the file's lock and succeeds once it is released.
func TestOpen_WaitsForALockedVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.db")
	release := lockNewVault(t, path)
	go func() {
		time.Sleep(150 * time.Millisecond)
		release()
	}()
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open after the lock is released: %v", err)
	}
	s.Close()
}

// BenchmarkHealthStoreCalls measures the store reads check_project_health
// makes for a project with every default kind: one KindModes call and one
// document read per current-state kind.
func BenchmarkHealthStoreCalls(b *testing.B) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(b.TempDir(), "vault.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		b.Fatal(err)
	}
	docs := []string{"memory", "architecture", "stack", "next_steps"}
	for _, kind := range docs {
		if err := s.WriteDocument(ctx, "acme", "", kind, "# "+kind); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < 500; i++ {
		date := fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)
		for _, kind := range []string{"progress", "decisions"} {
			if err := s.AppendEntry(ctx, "acme", "", kind, date, "## "+date+"\n- entry"); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.KindModes(ctx, "acme", ""); err != nil {
			b.Fatal(err)
		}
		for _, kind := range docs {
			if _, _, err := s.ReadDocument(ctx, "acme", "", kind); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// TestCreateProject_DuplicateIsErrAlreadyExists verifies that creating a
// project whose name is taken in the same scope wraps ErrAlreadyExists,
// while the same name under another parent is allowed.
func TestCreateProject_DuplicateIsErrAlreadyExists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	parent, err := s.CreateProject(ctx, "acme", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, "ACME", nil); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate top-level project: err = %v, want ErrAlreadyExists", err)
	}
	if _, err := s.CreateProject(ctx, "acme", &parent.ID); err != nil {
		t.Fatalf("same name as a subproject: %v", err)
	}
	if _, err := s.CreateProject(ctx, "acme", &parent.ID); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate subproject: err = %v, want ErrAlreadyExists", err)
	}
}

// TestWriteKinds_RefusesAStorageConflict checks that a kind keeps the
// storage it has: a document write to a log, or an entries write to a
// document, is refused as ErrStorageConflict, including when an earlier
// write of the same batch gave the kind its storage, and nothing of the
// batch is kept.
func TestWriteKinds_RefusesAStorageConflict(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEntry(ctx, "acme", "", "log", "", "entry"); err != nil {
		t.Fatal(err)
	}
	doc := "doc"
	if err := s.WriteKinds(ctx, "acme", "", []KindWrite{{Kind: "log", Document: &doc}}); !errors.Is(err, ErrStorageConflict) {
		t.Errorf("document write to a log: err = %v, want ErrStorageConflict", err)
	}
	err := s.WriteKinds(ctx, "acme", "", []KindWrite{
		{Kind: "api", Append: true, Sections: []EntrySection{{Body: "entry"}}},
		{Kind: "API", Document: &doc},
	})
	if !errors.Is(err, ErrStorageConflict) {
		t.Fatalf("one kind written both ways in a batch: err = %v, want ErrStorageConflict", err)
	}
	if mode, _ := s.KindMode(ctx, "acme", "", "api"); mode != KindStorageNone {
		t.Errorf("api after the refused batch: mode %v, want nothing written", mode)
	}
}

// TestCreateProject_MissingParentIsErrNotFound checks that a subproject
// created under a parent id that no longer exists is reported as not
// found, not as a raw foreign-key error.
func TestCreateProject_MissingParentIsErrNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	parent, err := s.CreateProject(ctx, "gone", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(ctx, "gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, "api", &parent.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("CreateProject under a deleted parent: err = %v, want ErrNotFound", err)
	}
}

// TestProjectWrites_RacingADeleteFailCleanly runs renames, promotions and
// subproject deletions while their project is deleted and created again:
// each succeeds or fails with ErrNotFound or ErrAlreadyExists, never with a
// raw database error.
func TestProjectWrites_RacingADeleteFailCleanly(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 300)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _, _ = s.EnsureProject(ctx, "acme", "api")
			_ = s.DeleteProject(ctx, "acme")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			errs <- s.RenameProject(ctx, "acme", "api", "web")
			errs <- s.RenameProject(ctx, "acme", "web", "api")
			errs <- s.DeleteSubproject(ctx, "acme", "api")
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("a project write racing a delete failed with %v", err)
		}
	}
}

// TestProjectTree lists every top-level project with its subprojects, in
// name order, with an empty (not nil) list for a project without any.
func TestProjectTree(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, p := range [][2]string{{"beta", ""}, {"acme", "web"}, {"acme", "api"}} {
		if _, _, err := s.EnsureProject(ctx, p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := s.ProjectTree(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 2 || tree[0].Name != "acme" || strings.Join(tree[0].Subprojects, ",") != "api,web" ||
		tree[1].Name != "beta" || tree[1].Subprojects == nil || len(tree[1].Subprojects) != 0 {
		t.Errorf("tree = %+v", tree)
	}
}
