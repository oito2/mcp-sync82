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
	"fmt"
	"regexp"
)

// entryMarkerPattern matches a whole line holding only an entry id marker
// such as "<!-- entry:42 -->", together with its line break.
var entryMarkerPattern = regexp.MustCompile(`(?m)^[ \t]*<!-- entry:\d+ -->[ \t]*(\r?\n|$)`)

// entryMarker returns the HTML comment line that read_memory puts before
// the entry with the given id when asked for entry ids.
func entryMarker(id int64) string {
	return fmt.Sprintf("<!-- entry:%d -->", id)
}

// stripEntryMarkers returns content without the entry id marker lines
// read_memory adds, so content read with ids and written back never stores
// them.
func stripEntryMarkers(content string) string {
	return entryMarkerPattern.ReplaceAllString(content, "")
}
