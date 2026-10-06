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

package binpath

import (
	"path/filepath"
	"testing"
)

// TestResolve_ReturnsAbsolutePath verifies that Resolve succeeds and returns
// an absolute path.
func TestResolve_ReturnsAbsolutePath(t *testing.T) {
	path, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("Resolve() = %q, want an absolute path", path)
	}
}
