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

//go:build unix

package tools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"
)

// TestImportProjectCore_SkipsSymlinksAndSpecialFiles verifies that import
// skips symlinks and special files instead of following a symlink to a file
// outside the input directory, reading a device such as /dev/zero without
// bound, or blocking forever on a FIFO. The import must return within 2
// seconds, import only the regular file, and not store the symlink target.
func TestImportProjectCore_SkipsSymlinksAndSpecialFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.md"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "stack.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("/dev/zero"); err == nil {
		if err := os.Symlink("/dev/zero", filepath.Join(dir, "architecture.md")); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "next_steps.md"), 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	type result struct {
		imported int
		skipped  []string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		report, err := importProjectCore(ctx, s, "acme", "", dir, false)
		imported, skipped := report.Imported(), report.Skipped
		done <- result{imported, skipped, err}
	}()
	var got result
	select {
	case got = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("import did not return within 2s")
	}
	if got.err != nil {
		t.Fatalf("import: %v", got.err)
	}
	if got.imported != 1 {
		t.Errorf("imported = %d, want 1 (memory.md only)", got.imported)
	}
	for _, name := range []string{"stack.md", "next_steps.md"} {
		if !slices.Contains(got.skipped, name) {
			t.Errorf("skipped = %v, want it to include %s", got.skipped, name)
		}
	}
	if _, ok, err := s.ReadContent(ctx, "acme", "", "stack"); err != nil || ok {
		t.Errorf("stack imported through a symlink: ok=%v err=%v", ok, err)
	}
}

// TestImportProjectCore_IsAllOrNothing verifies that an import failing on an
// unreadable file leaves the vault unchanged, so files processed before the
// failure are not written. The test is skipped when running as root, which can
// read any file.
func TestImportProjectCore_IsAllOrNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read a 0000 file")
	}
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "architecture", "original"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "architecture.md"), []byte("replaced"), 0o644); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(dir, "stack.md")
	if err := os.WriteFile(unreadable, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}

	if _, err := importProjectCore(ctx, s, "acme", "", dir, false); err == nil {
		t.Fatal("expected the unreadable file to fail the import")
	}
	if content, _, _ := s.ReadContent(ctx, "acme", "", "architecture"); content != "original" {
		t.Fatalf("architecture = %q, want the vault unchanged after a failed import", content)
	}
}
