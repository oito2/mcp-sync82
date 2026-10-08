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

package logging

import (
	"bytes"
	"strings"
	"testing"
)

// TestNew_WritesToTheGivenWriter verifies that New's logger writes its
// records to the writer it was given.
func TestNew_WritesToTheGivenWriter(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Info("smoke test", "key", "value")
	if got := buf.String(); !strings.Contains(got, "msg=\"smoke test\"") || !strings.Contains(got, "key=value") {
		t.Errorf("logged %q, want the record with its attribute", got)
	}
}
