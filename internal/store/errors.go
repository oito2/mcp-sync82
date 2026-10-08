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

// ErrNoSearchTerms is returned by SearchText when a words or phrase query
// holds no letter or number to search for.
var ErrNoSearchTerms = errors.New("the query has no words to search for")

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

// ErrSchemaTooNew is wrapped, together with ErrOpenFailed, when the vault's
// schema version is newer than the latest one this binary knows: a newer
// sync82 has upgraded the vault, and this one refuses to read or write it.
var ErrSchemaTooNew = errors.New("the vault's schema is newer than this sync82 supports; upgrade sync82")

// ErrVaultNotFound is returned by Manager.GetExisting when no vault file
// exists at the requested path. Its message holds no path; callers know it.
var ErrVaultNotFound = errors.New("vault not found")

// ErrStorageConflict is wrapped by WriteKinds when a write would store a
// kind the other way it is stored already: as a document while it has
// entries, or as entries while it has a document. The message names the
// kind and its current storage.
var ErrStorageConflict = errors.New("storage conflict")
