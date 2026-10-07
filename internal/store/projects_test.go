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
	"errors"
	"slices"
	"sync"
	"testing"
)

// TestEnsureProject_AutoCreatesParent verifies that EnsureProject creates a
// missing parent project together with the requested subproject.
func TestEnsureProject_AutoCreatesParent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sub, created, err := s.EnsureProject(ctx, "oito2", "sync82")
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if !created {
		t.Fatal("expected the subproject to be reported as created")
	}
	if sub.Name != "sync82" {
		t.Fatalf("got name %q, want %q", sub.Name, "sync82")
	}
	if !sub.ParentID.Valid {
		t.Fatal("expected subproject to have a parent_id")
	}

	parent, err := s.FindProjectByName(ctx, "oito2", nil)
	if err != nil {
		t.Fatalf("FindProjectByName: %v", err)
	}
	if parent == nil {
		t.Fatal("expected the parent project to have been auto-created")
	}
	if parent.ID != sub.ParentID.Int64 {
		t.Fatalf("subproject parent_id = %d, want %d", sub.ParentID.Int64, parent.ID)
	}

	again, created2, err := s.EnsureProject(ctx, "oito2", "sync82")
	if err != nil {
		t.Fatalf("EnsureProject (second call): %v", err)
	}
	if created2 {
		t.Fatal("expected created=false on the second call — the row already existed")
	}
	if again.ID != sub.ID {
		t.Fatalf("expected the same subproject row (id %d), got id %d", sub.ID, again.ID)
	}
}

// TestEnsureProject_ConcurrentCallsForSameNewTopLevelProject verifies that
// concurrent EnsureProject calls for the same new project name all succeed;
// the loser of the create race must resolve to the winner's row instead of
// returning a raw UNIQUE constraint error.
func TestEnsureProject_ConcurrentCallsForSameNewTopLevelProject(t *testing.T) {
	s := newTestStore(t)

	const n = 20
	var wg sync.WaitGroup
	ids := make([]int64, n)
	createdFlags := make([]bool, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, created, err := s.EnsureProject(context.Background(), "acme", "")
			errs[i] = err
			createdFlags[i] = created
			if p != nil {
				ids[i] = p.ID
			}
		}(i)
	}
	wg.Wait()

	createdCount := 0
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: EnsureProject returned an error under concurrent creation: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("goroutine %d: got project id %d, want the same id %d as goroutine 0", i, ids[i], ids[0])
		}
		if createdFlags[i] {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created=true reported %d times, want exactly 1 (exactly one goroutine should win the race)", createdCount)
	}

	projects, err := s.ListTopLevelProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("len(ListTopLevelProjects) = %d, want 1 (no duplicate row created)", len(projects))
	}
}

// TestEnsureProject_ConcurrentCallsForSameNewSubproject verifies that
// concurrent EnsureProject calls that both need to create the same new parent
// and the same new subproject all succeed.
func TestEnsureProject_ConcurrentCallsForSameNewSubproject(t *testing.T) {
	s := newTestStore(t)

	const n = 20
	var wg sync.WaitGroup
	ids := make([]int64, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, _, err := s.EnsureProject(context.Background(), "oito2", "sync82")
			errs[i] = err
			if p != nil {
				ids[i] = p.ID
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: EnsureProject returned an error under concurrent creation: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("goroutine %d: got subproject id %d, want the same id %d as goroutine 0", i, ids[i], ids[0])
		}
	}

	parents, err := s.ListTopLevelProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(parents) != 1 {
		t.Fatalf("len(ListTopLevelProjects) = %d, want 1 (no duplicate parent created)", len(parents))
	}
	subs, err := s.ListSubprojects(context.Background(), parents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("len(ListSubprojects) = %d, want 1 (no duplicate subproject created)", len(subs))
	}
}

// TestPromoteSubproject_ConcurrentPromoteToSameNameReportsErrAlreadyExists
// verifies that when two subprojects with the same name race to be promoted,
// exactly one succeeds and the other gets ErrAlreadyExists.
func TestPromoteSubproject_ConcurrentPromoteToSameNameReportsErrAlreadyExists(t *testing.T) {
	// Two subprojects named "widgets" under different parents race to be
	// promoted; both can pass the collision pre-check before either UPDATE
	// runs. Exactly one must win and the loser must get ErrAlreadyExists.
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", "widgets"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "other", "widgets"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	parents := []string{"acme", "other"}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = s.PromoteSubproject(context.Background(), parents[i], "widgets")
		}(i)
	}
	wg.Wait()

	okCount, existsCount := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			okCount++
		case errors.Is(err, ErrAlreadyExists):
			existsCount++
		default:
			t.Fatalf("goroutine %d: PromoteSubproject error = %v, want nil or ErrAlreadyExists", i, err)
		}
	}
	if okCount != 1 || existsCount != 1 {
		t.Fatalf("got %d ok + %d ErrAlreadyExists, want exactly 1 of each", okCount, existsCount)
	}

	top, err := s.ListTopLevelProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	widgetsCount := 0
	for _, p := range top {
		if p.Name == "widgets" {
			widgetsCount++
		}
	}
	if widgetsCount != 1 {
		t.Fatalf("top-level projects named %q = %d, want exactly 1", "widgets", widgetsCount)
	}
}

// TestDeleteProject_CascadesToSubprojectsAndData verifies that deleting a
// project also removes its subprojects, documents and entries.
func TestDeleteProject_CascadesToSubprojectsAndData(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sub, _, err := s.EnsureProject(ctx, "oito2", "sync82")
	if err != nil {
		t.Fatalf("EnsureProject: %v", err)
	}
	if err := s.WriteDocument(ctx, "oito2", "sync82", "memory", "hello"); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	if err := s.AppendEntry(ctx, "oito2", "sync82", "progress", "2026-01-01", "## 2026-01-01\n- x"); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	if err := s.DeleteProject(ctx, "oito2"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// Without foreign_keys enabled on the connection, ON DELETE CASCADE would
	// do nothing and these rows would survive as orphans.
	var projectCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, sub.ID).Scan(&projectCount); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if projectCount != 0 {
		t.Fatal("expected the subproject row to cascade-delete along with its parent")
	}

	var docCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&docCount); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if docCount != 0 {
		t.Fatal("expected documents to cascade-delete along with their project")
	}

	var entryCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries`).Scan(&entryCount); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if entryCount != 0 {
		t.Fatal("expected entries to cascade-delete along with their project")
	}
}

// TestPromoteAllSubprojects_PromotesEveryChild verifies that
// PromoteAllSubprojects promotes every subproject to a top-level project and
// returns their names.
func TestPromoteAllSubprojects_PromotesEveryChild(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "perci"); err != nil {
		t.Fatal(err)
	}

	promoted, err := s.PromoteAllSubprojects(ctx, "oito2")
	if err != nil {
		t.Fatalf("PromoteAllSubprojects: %v", err)
	}
	if len(promoted) != 2 {
		t.Fatalf("promoted = %v, want 2 names", promoted)
	}
	for _, name := range []string{"sync82", "perci"} {
		p, err := s.FindProjectByName(ctx, name, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p == nil {
			t.Fatalf("expected %q to be a top-level project after promotion", name)
		}
	}
}

// TestPromoteAllSubprojects_CollisionRollsBackTheWholeBatch verifies that a
// name collision on the middle of three subprojects makes
// PromoteAllSubprojects fail with ErrAlreadyExists and leaves none of the
// three promoted.
func TestPromoteAllSubprojects_CollisionRollsBackTheWholeBatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "oito2", "aaa-first"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "mmm-collides"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "zzz-last"); err != nil {
		t.Fatal(err)
	}
	// A top-level project already occupies the name of the middle
	// subproject (ListSubprojects returns them ordered by name, so
	// "mmm-collides" is promoted second, after "aaa-first").
	if _, _, err := s.EnsureProject(ctx, "mmm-collides", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := s.PromoteAllSubprojects(ctx, "oito2"); err == nil {
		t.Fatal("expected PromoteAllSubprojects to fail on the name collision")
	}

	// None of the three should have been promoted — not the one before
	// the collision, not the one after.
	parentID := mustParentID(t, ctx, s, "oito2")
	for _, name := range []string{"aaa-first", "zzz-last"} {
		p, err := s.FindProjectByName(ctx, name, &parentID)
		if err != nil {
			t.Fatal(err)
		}
		if p == nil {
			t.Fatalf("expected %q to still be a subproject of oito2 — the whole batch should have rolled back", name)
		}
	}
}

// mustParentID returns the ID of the top-level project name, failing the test
// immediately when the lookup errors or the project does not exist.
func mustParentID(t *testing.T, ctx context.Context, s *Store, name string) int64 {
	t.Helper()
	p, err := s.FindProjectByName(ctx, name, nil)
	if err != nil || p == nil {
		t.Fatalf("could not resolve parent %q: %v", name, err)
	}
	return p.ID
}

// TestRenameProject_TopLevel verifies that a top-level project can be
// renamed.
func TestRenameProject_TopLevel(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameProject(ctx, "oito2", "", "sync82"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	if p, err := s.FindProjectByName(ctx, "oito2", nil); err != nil || p != nil {
		t.Fatalf("expected the old name to no longer resolve, got %+v, err %v", p, err)
	}
	p, err := s.FindProjectByName(ctx, "sync82", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("expected the new name to resolve to the renamed project")
	}
}

// TestRenameProject_Subproject verifies that a subproject can be renamed
// without affecting its parent.
func TestRenameProject_Subproject(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "oito2", "borg-82"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameProject(ctx, "oito2", "borg-82", "sync82"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	parentID := mustParentID(t, ctx, s, "oito2")
	if p, err := s.FindProjectByName(ctx, "borg-82", &parentID); err != nil || p != nil {
		t.Fatalf("expected the old subproject name to no longer resolve, got %+v, err %v", p, err)
	}
	sub, err := s.FindProjectByName(ctx, "sync82", &parentID)
	if err != nil {
		t.Fatal(err)
	}
	if sub == nil {
		t.Fatal("expected the new subproject name to resolve")
	}
}

// TestRenameProject_CollisionReturnsErrAlreadyExists verifies that renaming to
// a name already used in the same scope returns ErrAlreadyExists.
func TestRenameProject_CollisionReturnsErrAlreadyExists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", ""); err != nil {
		t.Fatal(err)
	}

	err := s.RenameProject(ctx, "acme", "", "oito2")
	if err == nil {
		t.Fatal("expected an error renaming onto an existing top-level project name")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}

	// Nothing should have changed — "acme" still resolves under its
	// original name.
	if p, err := s.FindProjectByName(ctx, "acme", nil); err != nil || p == nil {
		t.Fatalf("expected %q to be unchanged after a failed rename, got %+v, err %v", "acme", p, err)
	}
}

// TestRenameProject_NotFound verifies that renaming a missing project returns
// ErrNotFound.
func TestRenameProject_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	err := s.RenameProject(ctx, "nonexistent", "", "whatever")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestPromoteSubprojectsAndDelete verifies that the subprojects are promoted
// and the parent deleted together, and that a name collision changes
// nothing at all.
func TestPromoteSubprojectsAndDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	for _, p := range [][2]string{{"oito2", "sync82"}, {"oito2", "perci"}, {"other", "build82"}, {"build82", ""}} {
		if _, _, err := s.EnsureProject(ctx, p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}

	promoted, err := s.PromoteSubprojectsAndDelete(ctx, "oito2")
	if err != nil || !slices.Equal(promoted, []string{"perci", "sync82"}) {
		t.Fatalf("PromoteSubprojectsAndDelete = %v, %v", promoted, err)
	}
	for name, want := range map[string]bool{"oito2": false, "perci": true, "sync82": true} {
		if exists, _ := s.ProjectExists(ctx, name, ""); exists != want {
			t.Errorf("project %q exists = %v, want %v", name, exists, want)
		}
	}

	if _, err := s.PromoteSubprojectsAndDelete(ctx, "other"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("collision: err = %v, want ErrAlreadyExists", err)
	}
	if exists, _ := s.ProjectExists(ctx, "other", "build82"); !exists {
		t.Error("a failed promotion changed the project tree")
	}
}
