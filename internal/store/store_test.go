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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newTestStore opens a new Store on a vault file in a temporary directory and
// registers a cleanup that closes it. It fails the test when opening fails.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s
}

// TestOpen_DirectoryCreationFailure_WrapsErrOpenFailed verifies that an Open
// failure caused by an unwritable parent directory returns an error wrapping
// ErrOpenFailed.
func TestOpen_DirectoryCreationFailure_WrapsErrOpenFailed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits don't block directory creation")
	}

	readOnlyParent := t.TempDir()
	if err := os.Chmod(readOnlyParent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(readOnlyParent, 0o755); err != nil {
			t.Error(err)
		}
	})

	path := filepath.Join(readOnlyParent, "nested", "vault.db")
	_, err := Open(context.Background(), path)
	if err == nil {
		t.Fatal("expected Open to fail under an unwritable parent directory")
	}
	if !errors.Is(err, ErrOpenFailed) {
		t.Errorf("Open error = %v, want it to wrap ErrOpenFailed", err)
	}
	if !strings.Contains(err.Error(), readOnlyParent) {
		t.Errorf("Open error = %v, want it to still contain the path for server-side logging", err)
	}
}

// TestOpen_RespectsCancelledContext verifies that Open fails with an error
// wrapping context.Canceled when ctx is already cancelled.
func TestOpen_RespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	path := filepath.Join(t.TempDir(), "vault.db")
	_, err := Open(ctx, path)
	if err == nil {
		t.Fatal("expected Open to fail against an already-cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Open error = %v, want it to wrap context.Canceled", err)
	}
}

// TestOpen_RejectsDSNBreakingCharacters verifies that Open rejects paths
// containing "?" or "#", which could alter the DSN's pragma parameters.
func TestOpen_RejectsDSNBreakingCharacters(t *testing.T) {
	for _, bad := range []string{
		filepath.Join(t.TempDir(), "vault.db?_pragma=foreign_keys(0)"),
		filepath.Join(t.TempDir(), "vault.db#fragment"),
	} {
		_, err := Open(context.Background(), bad)
		if err == nil {
			t.Fatalf("Open(%q) succeeded, want it rejected", bad)
		}
		if !errors.Is(err, ErrOpenFailed) {
			t.Errorf("Open(%q) error = %v, want it to wrap ErrOpenFailed", bad, err)
		}
	}
}

// TestOpen_CreatesPrivateVault verifies that a new vault, its directory and
// its WAL files are not accessible to group or others.
func TestOpen_CreatesPrivateVault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := filepath.Join(t.TempDir(), "vaultdir")
	path := filepath.Join(dir, "vault.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if _, _, err := s.EnsureProject(context.Background(), "acme", ""); err != nil {
		t.Fatal(err)
	}

	for p, want := range map[string]os.FileMode{dir: 0o700, path: 0o600, path + "-wal": 0o600} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s mode = %v, want %v", filepath.Base(p), got, want)
		}
	}
}
