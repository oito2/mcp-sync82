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

package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestAtomicWriteFile_CreatesFileWithContent verifies that a file is created,
// along with its missing parent directory, holding the written data.
func TestAtomicWriteFile_CreatesFileWithContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "file.txt")
	if err := AtomicWriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

// TestAtomicWriteFile_OverwritesExistingFile verifies that a second write
// replaces the content of an existing file.
func TestAtomicWriteFile_OverwritesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := AtomicWriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}
}

// TestAtomicWriteFile_NoTempFileLeftBehindOnSuccess verifies that the
// destination file is the only entry left in the directory after a write.
func TestAtomicWriteFile_NoTempFileLeftBehindOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := AtomicWriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "file.txt" {
		t.Errorf("expected only file.txt in %s, found %v", dir, entries)
	}
}

// TestAtomicWriteFile_SetsRequestedPermissions verifies that a newly created
// file gets exactly the requested permission bits.
func TestAtomicWriteFile_SetsRequestedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := AtomicWriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0o600", info.Mode().Perm())
	}
}

// TestAtomicWriteFile_CreatesMissingParentDirectories verifies that several
// levels of missing parent directories are created.
func TestAtomicWriteFile_CreatesMissingParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "file.txt")
	if err := AtomicWriteFile(path, []byte("nested"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "nested" {
		t.Errorf("content = %q, want %q", got, "nested")
	}
}

// TestAtomicWriteFile_PreservesExistingMode verifies that overwriting an
// existing 0600 file keeps that mode even when a more permissive perm is
// passed.
func TestAtomicWriteFile_PreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600 kept", got)
	}
}

// TestAtomicWriteFile_WritesThroughSymlink verifies that writing to a symlink
// updates the link's target and leaves the link itself in place.
func TestAtomicWriteFile_WritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.json")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := AtomicWriteFile(link, []byte("new"), 0o644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link is no longer a symlink (info=%v, err=%v)", info, err)
	}
	if data, _ := os.ReadFile(target); string(data) != "new" {
		t.Fatalf("target = %q, want %q", data, "new")
	}
}

// TestAtomicWriteFile_FailsWhenParentIsAFile verifies that an error is
// returned when the parent path is a regular file.
func TestAtomicWriteFile_FailsWhenParentIsAFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(filepath.Join(parent, "file.txt"), []byte("x"), 0o644); err == nil {
		t.Fatal("expected an error when the parent path is a regular file")
	}
}

// TestAtomicWriteFile_LeavesNoTempFileBehind verifies that a directory holds
// a single entry after a successful write.
func TestAtomicWriteFile_LeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	if err := AtomicWriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only file.txt", len(entries))
	}
}
