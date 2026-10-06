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

//go:build !unix && !windows

package config

import "os"

// lockFile is a no-op on platforms without a supported file-locking
// primitive, so only the in-process mutex serializes updates there. It
// always returns nil.
func lockFile(*os.File) error { return nil }

// unlockFile is the no-op counterpart of lockFile. It always returns nil.
func unlockFile(*os.File) error { return nil }
