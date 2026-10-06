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

// Package store implements sync82's SQLite-backed persistence layer: projects
// (each optionally holding subprojects), overwrite-style documents,
// append-only entries, and case-insensitive substring search across both.
// Manager caches one *Store per vault path so repeated calls against the
// same vault reuse a connection pool; Open handles per-vault schema
// migrations and connection setup. Every *Store method is safe for
// concurrent use. Lookup failures wrap ErrNotFound, naming collisions wrap
// ErrAlreadyExists, and Open failures wrap ErrOpenFailed; check them with
// errors.Is. Messages of all errors except ErrOpenFailed contain no
// filesystem paths.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Store is a SQLite-backed vault: one physical file holds every project's
// documents and entries. A Store is safe for concurrent use by multiple
// goroutines.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and
// applies pending schema migrations. ctx bounds the initial ping and the
// migration, so a cancelled or expired context stops Open from waiting on a
// slow disk or filesystem mount.
//
// It returns an error wrapping ErrOpenFailed when path contains '?' or '#',
// when the vault directory or file cannot be created, or when opening,
// pinging or migrating the database fails (including a vault whose schema
// is newer than this binary supports).
//
// SQLite pragmas are connection-scoped, not database-scoped, so
// foreign_keys and journal_mode are set via DSN query parameters
// (modernc.org/sqlite applies "_pragma" params to every new physical
// connection the pool creates) rather than a one-time PRAGMA statement —
// that guarantees every connection enforces foreign keys and uses WAL,
// including a connection database/sql reopens after closing one.
//
// The pool holds a single connection, so the operations of one Store run
// one at a time and only other processes contend for the file lock.
func Open(ctx context.Context, path string) (*Store, error) {
	// The DSN below is built by plain string concatenation, and
	// modernc.org/sqlite splits it on "?" to find the pragma query string,
	// so a caller-supplied path containing "?" or "#" would either break that
	// parsing or inject extra "_pragma" parameters that override the ones set
	// here (for example disabling foreign_keys). Such paths are rejected
	// rather than escaped.
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("%w: vault path must not contain '?' or '#': %q", ErrOpenFailed, path)
	}

	// The vault holds the user's project notes, so a new vault, its
	// directory and its -wal/-shm files (SQLite creates those with the
	// main file's permissions) are readable by the user only.
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("%w: create vault directory %s: %w", ErrOpenFailed, dir, err)
		}
	}
	if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
		f.Close()
	} else if !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%w: create vault %s: %w", ErrOpenFailed, path, err)
	}

	// _txlock=immediate makes every transaction take the write lock when it
	// begins, so busy_timeout applies to it. A deferred transaction that
	// reads first and writes later instead fails at once with SQLITE_BUSY
	// when another connection wrote in between.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: open vault %s: %w", ErrOpenFailed, path, err)
	}

	// One connection per pool: concurrent calls wait in database/sql for the
	// connection instead of contending for SQLite's write lock, where waiters
	// poll with growing sleeps and the slowest exceed busy_timeout.
	db.SetMaxOpenConns(1)

	if err := pingUntilNotBusy(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: open vault %s: %w", ErrOpenFailed, path, err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("%w: migrate vault %s: %w", ErrOpenFailed, path, err)
	}

	return &Store{db: db}, nil
}

// openBusyTimeout bounds how long pingUntilNotBusy keeps retrying a
// connection that fails with SQLITE_BUSY, matching the busy_timeout pragma.
const openBusyTimeout = 5 * time.Second

// openBusyRetryDelay is the pause between two connection attempts in
// pingUntilNotBusy.
const openBusyRetryDelay = 20 * time.Millisecond

// pingUntilNotBusy opens a first connection to db, retrying for up to
// openBusyTimeout while it fails with SQLITE_BUSY. Switching a new database
// to WAL, which the journal_mode pragma does on each new connection, needs
// an exclusive lock and reports SQLITE_BUSY at once, without waiting on
// busy_timeout, when another connection opens the same file at the same
// moment. It returns nil once a connection succeeds, the last error when
// the failure is not SQLITE_BUSY or the time is up, or ctx's error when ctx
// ends first.
func pingUntilNotBusy(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(openBusyTimeout)
	for {
		err := db.PingContext(ctx)
		if err == nil || !isBusyErr(err) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(openBusyRetryDelay):
		}
	}
}

// isBusyErr reports whether err wraps a modernc.org/sqlite error whose
// primary result code is SQLITE_BUSY.
func isBusyErr(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == sqlite3.SQLITE_BUSY
}

// Close closes the underlying database connection pool and returns any error
// reported by the driver.
func (s *Store) Close() error {
	return s.db.Close()
}

// now returns the current UTC instant as an RFC 3339 timestamp, the format
// used by every *_at column in the schema.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// isUniqueConstraintErr reports whether err wraps a modernc.org/sqlite error
// whose extended result code is SQLITE_CONSTRAINT_UNIQUE. Other constraint
// violations, such as NOT NULL, do not match. It lets callers turn a
// create-after-check race between concurrent writers into a defined outcome
// instead of a raw driver error.
func isUniqueConstraintErr(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

// label formats a project/subproject pair for error messages: "project" when
// subproject is empty, otherwise "project/subproject".
func label(project, subproject string) string {
	if subproject == "" {
		return project
	}
	return project + "/" + subproject
}

// nullableString converts an empty string to SQL NULL and any other string to
// itself, so an absent value (such as entries.entry_date) is stored as a real
// NULL rather than an empty string.
func nullableString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// resolveProjectID resolves a (project, subproject) name pair to the target
// project row's ID. Names are normalized first; subproject may be empty to
// target the top-level project itself. It returns an error wrapping
// ErrNotFound when the project or subproject does not exist, or a database
// error.
func (s *Store) resolveProjectID(ctx context.Context, project, subproject string) (int64, error) {
	project, subproject = normalizeName(project), normalizeName(subproject)
	parent, err := s.FindProjectByName(ctx, project, nil)
	if err != nil {
		return 0, err
	}
	if parent == nil {
		return 0, fmt.Errorf("project not found: %q: %w", project, ErrNotFound)
	}
	if subproject == "" {
		return parent.ID, nil
	}

	sub, err := s.FindProjectByName(ctx, subproject, &parent.ID)
	if err != nil {
		return 0, err
	}
	if sub == nil {
		return 0, fmt.Errorf("project not found: %q/%q: %w", project, subproject, ErrNotFound)
	}
	return sub.ID, nil
}

// normalizeName lower-cases a project, subproject or kind name. Every name is
// stored and looked up lower-cased, so names differing only in case refer to
// the same project or kind (and cannot collide on a case-insensitive
// filesystem when exported).
func normalizeName(name string) string {
	return strings.ToLower(name)
}
