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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// ErrManagerClosed is returned by Get once Close has been called, so a
// goroutine still running during shutdown cannot open a connection that
// Close would never release.
var ErrManagerClosed = errors.New("store manager is closed")

// Manager caches one Store per vault path, so repeated calls against the
// same vault reuse one connection pool instead of reopening the database and
// re-running the migration check each time. A process may use several vault
// paths, hence one Store per path. A Manager is safe for concurrent use.
type Manager struct {
	mu     sync.Mutex
	stores map[string]*Store
	closed bool
}

// NewManager returns an empty, open Manager.
func NewManager() *Manager {
	return &Manager{stores: make(map[string]*Store)}
}

// Get returns the Store for path, opening and caching it on first use. path
// is made absolute first, so equivalent spellings share one Store. It returns
// ErrManagerClosed once Close has run, and any error from Open (wrapping
// ErrOpenFailed) when the vault cannot be opened.
//
// Open runs outside m.mu because it performs file I/O and migrations; a slow
// first open of one vault must not block calls against other vaults. The lock
// only guards the cache lookup and the reconciliation after Open returns.
func (m *Manager) Get(ctx context.Context, path string) (*Store, error) {
	// One cache entry per file: "./v.db", "v.db" and its absolute form
	// share a Store.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrManagerClosed
	}
	if s, ok := m.stores[path]; ok {
		m.mu.Unlock()
		return s, nil
	}
	m.mu.Unlock()

	s, err := Open(ctx, path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		s.Close()
		return nil, ErrManagerClosed
	}
	if existing, ok := m.stores[path]; ok {
		// Another goroutine cached a Store for this path while Open ran
		// without the lock; keep that one and discard ours.
		s.Close()
		return existing, nil
	}
	m.stores[path] = s
	return s, nil
}

// GetExisting is Get for callers that must not create a vault: when no file
// exists at path it returns an error wrapping ErrVaultNotFound instead of
// creating an empty vault there. Other errors are those of Get.
func (m *Manager) GetExisting(ctx context.Context, path string) (*Store, error) {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrVaultNotFound, path)
	}
	return m.Get(ctx, path)
}

// Close closes every cached Store, empties the cache and marks the Manager
// closed, so later Get calls fail with ErrManagerClosed. It returns the first
// error reported while closing a Store, after attempting to close all of
// them.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	var firstErr error
	for path, s := range m.stores {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(m.stores, path)
	}
	return firstErr
}
