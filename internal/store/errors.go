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

package store

import "errors"

// ErrNotFound is wrapped by any lookup that fails to find its target
// (project, subproject, document kind, or entries kind). Check for it with
// errors.Is; the error message describes what was not found without
// including filesystem paths.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is wrapped when an operation would create a naming
// collision, for example promoting a subproject to a name already used at
// the vault root.
var ErrAlreadyExists = errors.New("already exists")

// ErrOpenFailed is wrapped by Open when the vault directory, file, database
// connection or migration step fails. Unlike every other store error, its
// messages embed a raw filesystem path, so callers that return errors to
// remote clients should detect it with errors.Is and substitute a generic
// message.
var ErrOpenFailed = errors.New("could not open vault")

// ErrVaultNotFound is wrapped by Manager.GetExisting when no vault file
// exists at the requested path.
var ErrVaultNotFound = errors.New("vault not found")
