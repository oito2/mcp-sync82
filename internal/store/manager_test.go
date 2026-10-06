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
	"path/filepath"
	"sync"
	"testing"
)

// TestManager_Get_ReturnsSameStoreForSamePath verifies that repeated Get calls
// for the same path return the same Store.
func TestManager_Get_ReturnsSameStoreForSamePath(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	s1, err := mgr.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	s2, err := mgr.Get(ctx, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s1 != s2 {
		t.Fatal("expected the same cached *Store for repeated Get calls with the same path")
	}
}

// TestManager_Get_ConcurrentSamePathConvergesOnOneStore verifies that
// goroutines racing to open the same uncached path all end up with one
// shared Store, the others being closed rather than leaked.
func TestManager_Get_ConcurrentSamePathConvergesOnOneStore(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")

	const goroutines = 8
	stores := make([]*Store, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			stores[i], errs[i] = mgr.Get(ctx, path)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Get[%d]: %v", i, err)
		}
		if stores[i] != stores[0] {
			t.Fatalf("Get[%d] returned a different *Store than Get[0] — every concurrent caller for the same path must converge on one", i)
		}
	}
}

// TestManager_Get_ConcurrentDifferentPathsAllSucceed verifies that distinct
// vault paths opened concurrently each get their own Store.
func TestManager_Get_ConcurrentDifferentPathsAllSucceed(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	ctx := context.Background()
	dir := t.TempDir()

	const goroutines = 8
	stores := make([]*Store, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			path := filepath.Join(dir, "vault-"+string(rune('a'+i))+".db")
			stores[i], errs[i] = mgr.Get(ctx, path)
		}(i)
	}
	wg.Wait()

	seen := make(map[*Store]bool, goroutines)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Get[%d]: %v", i, err)
		}
		if seen[stores[i]] {
			t.Fatalf("Get[%d] returned a *Store already seen — each distinct path must get its own Store", i)
		}
		seen[stores[i]] = true
	}
}

// TestManager_Get_AfterClose_ReturnsErrManagerClosed verifies that Get returns
// ErrManagerClosed after Close.
func TestManager_Get_AfterClose_ReturnsErrManagerClosed(t *testing.T) {
	mgr := NewManager()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vault.db")
	if _, err := mgr.Get(ctx, path); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := mgr.Get(ctx, path); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("Get after Close: err = %v, want ErrManagerClosed", err)
	}
}
