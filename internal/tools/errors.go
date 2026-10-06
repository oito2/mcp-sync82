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

package tools

import (
	"errors"
	"fmt"

	"github.com/oito2/mcp-sync82/internal/store"
)

// wrapNotFound turns a store.ErrNotFound caused by a missing project into
// an action-oriented error that names label and suggests create_project.
// Any other err is returned unchanged, and a nil err stays nil.
//
// Only call this where the store operation's sole possible ErrNotFound
// cause is a missing project row: write_memory's WriteDocument/
// ReplaceAllEntries and append_memory's AppendEntry all resolve the
// project first and have no separate "kind not found" error path. By
// contrast, read_memory's ReadContent reports a missing kind via
// ok=false (not an error) and delete_memory's DeleteKind returns the same
// error for a missing project and a missing kind — neither should be
// rewritten this way, since it would mislabel a missing kind as a
// missing project.
func wrapNotFound(err error, label string) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("project not found: %q; use create_project first", label)
	}
	return err
}
