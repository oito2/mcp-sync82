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

package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// analyzeWithin runs AnalyzeProject(root) and returns its Result. It fails the
// test if the call has not returned within d.
func analyzeWithin(t *testing.T, root string, d time.Duration) Result {
	t.Helper()
	done := make(chan Result, 1)
	go func() { done <- AnalyzeProject(root) }()
	select {
	case r := <-done:
		return r
	case <-time.After(d):
		t.Fatalf("AnalyzeProject(%s) did not return within %s", root, d)
		return Result{}
	}
}

// TestReadMarkerFile_SkipsFIFO verifies that AnalyzeProject returns promptly when a marker file is a
// FIFO.
func TestReadMarkerFile_SkipsFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "package.json"), 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	r := analyzeWithin(t, root, 2*time.Second)
	if len(r.Languages) != 0 {
		t.Errorf("Languages = %v, want none from a FIFO", r.Languages)
	}
}

// TestReadMarkerFile_SkipsDevice verifies that a marker file symlinked to a device is skipped.
func TestReadMarkerFile_SkipsDevice(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip("/dev/zero unavailable")
	}
	root := t.TempDir()
	if err := os.Symlink("/dev/zero", filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	r := analyzeWithin(t, root, 2*time.Second)
	if len(r.Languages) != 0 {
		t.Errorf("Languages = %v, want none from a device", r.Languages)
	}
}

// TestOpenMarker_DoesNotBlockOnAFIFO verifies that opening a FIFO with no
// writer returns at once, so a FIFO swapped in after readMarkerFile's Stat
// can't block it; the file is then rejected as not regular.
func TestOpenMarker_DoesNotBlockOnAFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "Cargo.toml"), 0o644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	opened := make(chan error, 1)
	go func() {
		f, err := openMarker(r, "Cargo.toml")
		if err == nil {
			info, serr := f.Stat()
			if serr == nil && info.Mode().IsRegular() {
				err = os.ErrInvalid
			}
			f.Close()
		}
		opened <- err
	}()
	select {
	case err := <-opened:
		if err != nil {
			t.Fatalf("openMarker on a FIFO: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("openMarker blocked on a FIFO with no writer")
	}
}

// TestDetectComponents_IgnoresSymlinkedMonorepoDir checks that a packages/
// directory that is a symlink to somewhere outside the workspace is not
// listed.
func TestDetectComponents_IgnoresSymlinkedMonorepoDir(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "secret-client"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "packages")); err != nil {
		t.Fatal(err)
	}
	r := analyzeWithin(t, root, 2*time.Second)
	for _, c := range r.Components {
		if strings.Contains(c, "secret-client") {
			t.Fatalf("Components = %v, listing a directory outside the workspace", r.Components)
		}
	}
}
