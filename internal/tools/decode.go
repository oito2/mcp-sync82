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
	"bytes"
	"encoding/json"
	"fmt"
)

// decodeArgs unmarshals the raw JSON arguments of a tool call into dst, a
// pointer to a struct. Empty arguments leave dst as the zero value: every
// field is optional at the decoding level, and required fields and other
// business rules are checked by each tool's Validate. It returns an
// "invalid arguments" error when the JSON is malformed, a value has the
// wrong type, or an argument is not a field of dst — so a misspelled name
// (e.g. "keepDays" for "keep_days") is reported instead of silently
// falling back to a default.
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}
