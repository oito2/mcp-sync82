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

// Package fsutil provides small filesystem helpers shared across packages.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// AtomicWriteFile writes data to path using a write-to-temp-then-rename
// pattern in path's directory: readers see either the old content or the
// new one, never a half-written file, and the data and the rename are
// flushed to disk before it returns, so a crash or power loss leaves one
// of the two versions in place.
//
// path is the destination file, data its full new content, and perm the mode
// for a newly created file. A path that is a symlink is written through to its target, keeping the
// link; a dangling symlink, whose target doesn't exist, is replaced by a
// regular file instead. An existing file keeps its permission bits; perm only applies to a
// newly created file, and is applied exactly (not masked by the umask).
// Missing parent directories are created private to the user (0700).
//
// It returns an error when the path cannot be resolved, the directory or
// temporary file cannot be created, or writing, syncing, closing, chmod or
// renaming fails; the temporary file is removed in every failure case. A
// failure to sync the directory after the rename is ignored.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("resolve %s: %w", path, err)
	}
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	return writeAndRename(path, data, perm)
}

// ErrNotRegularFile is returned by AtomicReplaceFile when path exists and
// is not a regular file: a symlink, a directory or a special file.
var ErrNotRegularFile = errors.New("not a regular file")

// AtomicReplaceFile is AtomicWriteFile for a path that must not be
// redirected: it never follows a symlink at path. It writes data to path
// atomically, as AtomicWriteFile does, when path does not exist or is a
// regular file (keeping its permission bits). It returns an error wrapping
// ErrNotRegularFile, without writing anything, when path is a symlink,
// directory or special file, and otherwise the errors of AtomicWriteFile.
func AtomicReplaceFile(path string, data []byte, perm os.FileMode) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil && !info.Mode().IsRegular():
		return fmt.Errorf("refusing to replace %s: %w", path, ErrNotRegularFile)
	case err == nil:
		perm = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	return writeAndRename(path, data, perm)
}

// writeAndRename writes data to a temporary file in path's directory,
// creating missing directories private to the user (0700), syncs it, sets
// perm on it and renames it onto path with renameFile, which replaces
// whatever entry path names (a symlink itself, not its target). It then syncs the directory,
// ignoring a failure. The temporary file is removed on every failure.
func writeAndRename(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}
	if err := renameFile(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}
	// Persist the rename itself: the new directory entry must reach the
	// disk too. Best-effort — the data is already written.
	SyncDir(dir)
	return nil
}

// Rename renames oldpath to newpath like os.Rename. On Windows, a rename
// blocked by another process holding either file open (an antivirus scan,
// a reader) is retried for up to a second.
func Rename(oldpath, newpath string) error {
	return renameFile(oldpath, newpath)
}

// SyncDir flushes dir's entries to disk, so a rename or a new file in it
// survives a crash. It is best-effort: a failure is ignored, and it does
// nothing on Windows, where a directory can't be synced this way.
func SyncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}
