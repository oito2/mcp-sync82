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
)

// Project is a row of the projects table: a top-level project (ParentID not
// valid) or a subproject (ParentID holds the parent's ID). CreatedAt is an
// RFC 3339 timestamp.
type Project struct {
	ID        int64
	Name      string
	ParentID  sql.NullInt64
	CreatedAt string
}

// FindProjectByName looks up a project by name within the given parent scope
// (parentID nil means top-level). The name is normalized. It returns (nil,
// nil) when no matching row exists, and a database error otherwise.
func (s *Store) FindProjectByName(ctx context.Context, name string, parentID *int64) (*Project, error) {
	name = normalizeName(name)
	var row *sql.Row
	if parentID == nil {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, name, parent_id, created_at FROM projects WHERE parent_id IS NULL AND name = ?`, name)
	} else {
		row = s.db.QueryRowContext(ctx,
			`SELECT id, name, parent_id, created_at FROM projects WHERE parent_id = ? AND name = ?`, *parentID, name)
	}

	var p Project
	if err := row.Scan(&p.ID, &p.Name, &p.ParentID, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find project %q: %w", name, err)
	}
	return &p, nil
}

// ProjectExists reports whether the (project, subproject) pair exists.
// subproject may be empty to check the top-level project itself. It returns
// an error only for database failures.
func (s *Store) ProjectExists(ctx context.Context, project, subproject string) (bool, error) {
	_, err := s.resolveProjectID(ctx, project, subproject)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateProject inserts a new project row named name (normalized), top-level
// when parentID is nil and otherwise a child of parentID. It returns the
// created Project, an error wrapping ErrAlreadyExists when a project with
// that name already exists in the same scope, one wrapping ErrNotFound when
// parentID names no project (it was deleted meanwhile), or a database
// error. The
// existence of parentID is checked by the foreign key.
func (s *Store) CreateProject(ctx context.Context, name string, parentID *int64) (*Project, error) {
	name = normalizeName(name)
	var parent sql.NullInt64
	if parentID != nil {
		parent = sql.NullInt64{Int64: *parentID, Valid: true}
	}
	createdAt := now()

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (name, parent_id, created_at) VALUES (?, ?, ?)`, name, parent, createdAt)
	if isUniqueConstraintErr(err) {
		return nil, fmt.Errorf("create project %q: a project named %q already exists in the same scope: %w", name, name, ErrAlreadyExists)
	}
	if isForeignKeyErr(err) {
		// The parent was deleted after the caller looked it up.
		return nil, fmt.Errorf("create project %q: its parent project no longer exists: %w", name, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("create project %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create project %q: %w", name, err)
	}
	return &Project{ID: id, Name: name, ParentID: parent, CreatedAt: createdAt}, nil
}

// EnsureProject returns the project or subproject, creating it (and its
// parent project when missing) if needed. subproject may be empty for a
// top-level project. created reports whether the leaf (project or
// subproject) was created by this call; it does not report whether a missing
// parent was also created. It returns a database error on failure.
//
// The check-then-create sequence is not a single transaction, so concurrent
// calls for the same new name can both attempt the insert. The UNIQUE index
// decides the winner, and createOrRecoverExisting turns the loser's
// constraint violation into the existing row.
func (s *Store) EnsureProject(ctx context.Context, project, subproject string) (proj *Project, created bool, err error) {
	project, subproject = normalizeName(project), normalizeName(subproject)
	if subproject == "" {
		existing, err := s.FindProjectByName(ctx, project, nil)
		if err != nil {
			return nil, false, err
		}
		if existing != nil {
			return existing, false, nil
		}
		return s.createOrRecoverExisting(ctx, project, nil)
	}

	parent, err := s.FindProjectByName(ctx, project, nil)
	if err != nil {
		return nil, false, err
	}
	if parent == nil {
		parent, _, err = s.createOrRecoverExisting(ctx, project, nil)
		if err != nil {
			return nil, false, err
		}
	}

	existing, err := s.FindProjectByName(ctx, subproject, &parent.ID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	return s.createOrRecoverExisting(ctx, subproject, &parent.ID)
}

// createOrRecoverExisting creates a project (top-level if parentID is nil,
// otherwise a child of parentID) and reports created as true. If creation
// fails with ErrAlreadyExists, meaning another goroutine created the same
// name first, it returns the existing row with created false. Any
// other failure is returned as is, and so is the original error if the row
// cannot be found afterwards.
func (s *Store) createOrRecoverExisting(ctx context.Context, name string, parentID *int64) (*Project, bool, error) {
	p, err := s.CreateProject(ctx, name, parentID)
	if err == nil {
		return p, true, nil
	}
	if !errors.Is(err, ErrAlreadyExists) {
		return nil, false, err
	}

	existing, findErr := s.FindProjectByName(ctx, name, parentID)
	if findErr != nil {
		return nil, false, findErr
	}
	if existing == nil {
		// The conflicting row is gone again (for example deleted
		// concurrently); return the original error rather than a nil
		// project.
		return nil, false, err
	}
	return existing, false, nil
}

// ListTopLevelProjects returns every project with no parent, sorted by name.
// It returns a database error on failure.
func (s *Store) ListTopLevelProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, parent_id, created_at FROM projects WHERE parent_id IS NULL ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list top-level projects: %w", err)
	}
	defer rows.Close()
	return scanProjects(rows)
}

// ProjectNode is one top-level project of a ProjectTree with the names of
// its subprojects, in name order.
type ProjectNode struct {
	Name        string
	Subprojects []string
}

// ProjectTree returns every top-level project with its subprojects, both
// in name order, read in one query. Subprojects is empty, not nil, for a
// project without any. It returns a database error on failure.
func (s *Store) ProjectTree(ctx context.Context) ([]ProjectNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.name, c.name FROM projects p
		LEFT JOIN projects c ON c.parent_id = p.id
		WHERE p.parent_id IS NULL
		ORDER BY p.name, c.name`)
	if err != nil {
		return nil, fmt.Errorf("list project tree: %w", err)
	}
	defer rows.Close()
	var tree []ProjectNode
	for rows.Next() {
		var name string
		var sub sql.NullString
		if err := rows.Scan(&name, &sub); err != nil {
			return nil, fmt.Errorf("list project tree: %w", err)
		}
		if len(tree) == 0 || tree[len(tree)-1].Name != name {
			tree = append(tree, ProjectNode{Name: name, Subprojects: []string{}})
		}
		if sub.Valid {
			last := &tree[len(tree)-1]
			last.Subprojects = append(last.Subprojects, sub.String)
		}
	}
	return tree, rows.Err()
}

// ListSubprojects returns every project whose parent is parentID, sorted by
// name. It returns a database error on failure.
func (s *Store) ListSubprojects(ctx context.Context, parentID int64) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, parent_id, created_at FROM projects WHERE parent_id = ? ORDER BY name`, parentID)
	if err != nil {
		return nil, fmt.Errorf("list subprojects: %w", err)
	}
	defer rows.Close()
	return scanProjects(rows)
}

// scanProjects reads every remaining row of rows (columns id, name,
// parent_id, created_at) into Projects. It returns a scan or iteration error.
// The caller closes rows.
func scanProjects(rows *sql.Rows) ([]Project, error) {
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.ParentID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project row: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PromoteSubproject moves the subproject subName of parentName to the vault
// root as a top-level project, in one transaction that also resolves both
// names. It returns an error wrapping ErrNotFound when the parent or
// subproject does not exist, an error wrapping ErrAlreadyExists when a
// top-level project with that name exists, or a database error.
func (s *Store) PromoteSubproject(ctx context.Context, parentName, subName string) error {
	parentName, subName = normalizeName(parentName), normalizeName(subName)
	return s.withProjectTx(ctx, parentName, subName, func(tx *sql.Tx, subID int64) error {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET parent_id = NULL WHERE id = ?`, subID); err != nil {
			if isUniqueConstraintErr(err) {
				return fmt.Errorf("cannot promote %q: a top-level project named %q already exists: %w", subName, subName, ErrAlreadyExists)
			}
			return fmt.Errorf("promote subproject %q/%q: %w", parentName, subName, err)
		}
		return nil
	})
}

// PromoteAllSubprojects promotes every subproject of parentName to the vault
// root in a single transaction: either all of them are promoted or none is.
// The parent and its subprojects are read inside that transaction, so the
// promoted set is the one present when the writes are applied. It returns
// the promoted names (nil when there are none). It returns an
// error wrapping ErrNotFound when the parent does not exist, an error
// wrapping ErrAlreadyExists when any name collides with a top-level project
// (rolling the whole batch back), or a database error.
func (s *Store) PromoteAllSubprojects(ctx context.Context, parentName string) ([]string, error) {
	return s.promoteSubprojects(ctx, parentName, false)
}

// PromoteSubprojectsAndDelete moves every subproject of the top-level
// project parentName to the vault root, as PromoteAllSubprojects does, and
// then deletes parentName with its own documents and entries, all in one
// transaction: either both happen or nothing changes. It returns the
// promoted names (nil when there are none). It returns an error wrapping
// ErrNotFound when the project does not exist, an error wrapping
// ErrAlreadyExists when a subproject name collides with a top-level
// project, or a database error.
func (s *Store) PromoteSubprojectsAndDelete(ctx context.Context, parentName string) ([]string, error) {
	return s.promoteSubprojects(ctx, parentName, true)
}

// promoteSubprojects implements PromoteAllSubprojects and, when
// deleteParent is set, PromoteSubprojectsAndDelete.
func (s *Store) promoteSubprojects(ctx context.Context, parentName string, deleteParent bool) ([]string, error) {
	parentName = normalizeName(parentName)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("promote subprojects of %q: %w", parentName, err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	var parentID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE parent_id IS NULL AND name = ?`, parentName).Scan(&parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("project not found: %q: %w", parentName, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("find project %q: %w", parentName, err)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, name, parent_id, created_at FROM projects WHERE parent_id = ? ORDER BY name`, parentID)
	if err != nil {
		return nil, fmt.Errorf("list subprojects: %w", err)
	}
	subs, err := scanProjects(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, sub := range subs {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET parent_id = NULL WHERE id = ?`, sub.ID); err != nil {
			if isUniqueConstraintErr(err) {
				return nil, fmt.Errorf("cannot promote %q: a top-level project named %q already exists: %w", sub.Name, sub.Name, ErrAlreadyExists)
			}
			return nil, fmt.Errorf("promote subprojects of %q: %w", parentName, err)
		}
		names = append(names, sub.Name)
	}
	if deleteParent {
		if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, parentID); err != nil {
			return nil, fmt.Errorf("delete project %q: %w", parentName, err)
		}
	} else if len(names) == 0 {
		return nil, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("promote subprojects of %q: %w", parentName, err)
	}
	return names, nil
}

// DeleteProject deletes a top-level project by name, cascading to its
// subprojects, documents and entries via ON DELETE CASCADE. It returns an
// error wrapping ErrNotFound when no such project exists, or a database
// error.
func (s *Store) DeleteProject(ctx context.Context, name string) error {
	name = normalizeName(name)
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE parent_id IS NULL AND name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete project %q: %w", name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project %q: %w", name, err)
	}
	if n == 0 {
		return fmt.Errorf("project not found: %q: %w", name, ErrNotFound)
	}
	return nil
}

// DeleteSubproject deletes the subproject subName of parentName, cascading to
// its documents and entries, in one transaction that also resolves both
// names. It returns an error wrapping ErrNotFound when the parent or
// subproject does not exist, or a database error.
func (s *Store) DeleteSubproject(ctx context.Context, parentName, subName string) error {
	parentName, subName = normalizeName(parentName), normalizeName(subName)
	return s.withProjectTx(ctx, parentName, subName, func(tx *sql.Tx, subID int64) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, subID); err != nil {
			return fmt.Errorf("delete subproject %q/%q: %w", parentName, subName, err)
		}
		return nil
	})
}

// RenameProject renames a top-level project (subproject == "") or a specific
// subproject in place to newName (normalized), in one transaction that also
// resolves the names. It returns an error wrapping ErrNotFound when the
// project or subproject does not exist, an error wrapping ErrAlreadyExists
// when a sibling in the same scope already has newName, or a database
// error. Only the database is changed.
func (s *Store) RenameProject(ctx context.Context, project, subproject, newName string) error {
	project, subproject, newName = normalizeName(project), normalizeName(subproject), normalizeName(newName)
	return s.withProjectTx(ctx, project, subproject, func(tx *sql.Tx, targetID int64) error {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET name = ? WHERE id = ?`, newName, targetID); err != nil {
			if isUniqueConstraintErr(err) {
				return fmt.Errorf("cannot rename %q to %q: a project named %q already exists in the same scope: %w", label(project, subproject), newName, newName, ErrAlreadyExists)
			}
			return fmt.Errorf("rename %s: %w", label(project, subproject), err)
		}
		return nil
	})
}
