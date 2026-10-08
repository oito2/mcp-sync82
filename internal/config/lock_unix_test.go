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

package config

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"
)

// TestIsReadOnlyFS recognizes EROFS, also wrapped, and nothing else.
func TestIsReadOnlyFS(t *testing.T) {
	erofs := &fs.PathError{Op: "open", Path: "/ro/config.lock", Err: syscall.EROFS}
	if !isReadOnlyFS(erofs) || !isReadOnlyFS(fmt.Errorf("open lock: %w", erofs)) {
		t.Error("EROFS not recognized")
	}
	if isReadOnlyFS(&fs.PathError{Op: "open", Path: "x", Err: syscall.EACCES}) || isReadOnlyFS(os.ErrNotExist) {
		t.Error("another error taken for EROFS")
	}
}
