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

import "testing"

// TestToolHints_CoverEveryRegisteredTool verifies that every registered tool
// has hints with a title, that no hint names a tool that doesn't exist, and
// that a read-only tool is never marked destructive.
func TestToolHints_CoverEveryRegisteredTool(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	registered := map[string]bool{}
	for _, tool := range Registered(r, mgr) {
		registered[tool.Name()] = true
		h, ok := HintsFor(tool.Name())
		if !ok || h.Title == "" {
			t.Errorf("tool %q has no hints or no title: %+v", tool.Name(), h)
		}
		if h.ReadOnly && h.Destructive {
			t.Errorf("tool %q is both read-only and destructive", tool.Name())
		}
	}
	for name := range toolHints {
		if !registered[name] {
			t.Errorf("hints for %q, which is not a registered tool", name)
		}
	}
}
