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
	"errors"
	"os"
	"syscall"
)

// lockFile blocks until it holds a flock(2) lock on f, exclusive or shared,
// retrying when the wait is interrupted by a signal. Closing f also
// releases it. It returns the flock error otherwise.
func lockFile(f *os.File, exclusive bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// unlockFile releases the flock(2) lock held on f. It returns the flock
// error, if any.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// isReadOnlyFS reports whether err comes from a write to a read-only
// filesystem (EROFS).
func isReadOnlyFS(err error) bool {
	return errors.Is(err, syscall.EROFS)
}
