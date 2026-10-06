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

// Package binpath resolves the path of the running sync82 binary.
package binpath

import (
	"fmt"
	"os"
	"path/filepath"
)

// Resolve returns the absolute path of the running executable with
// symlinks resolved, so it names the real file. When the symlinks cannot be
// resolved, the unresolved path is returned. It returns an error only when
// the operating system cannot report the executable path.
func Resolve() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved, nil
	}
	return p, nil
}
