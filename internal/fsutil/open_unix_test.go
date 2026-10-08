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

package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestOpenFlags checks that OpenNonblock opens a FIFO with no writer at
// once, and that OpenNoFollow refuses to open a symlink.
func TestOpenFlags(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(fifo, os.O_RDONLY|OpenNonblock|OpenNoFollow, 0)
		if err == nil {
			f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("open FIFO: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("opening a FIFO with no writer blocked")
	}

	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if f, err := os.OpenFile(link, os.O_RDONLY|OpenNoFollow, 0); err == nil {
		f.Close()
		t.Error("a symlink was opened with OpenNoFollow")
	}
}
