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

// Package version holds the sync82 release version.
package version

import "runtime/debug"

// Current is the release version, set at build time with
//
//	-ldflags "-X github.com/oito2/mcp-sync82/internal/version.Current=vX.Y.Z"
//
// A build without that flag leaves it "dev".
var Current = "dev"

// Get returns the running binary's version: Current when it was set at
// build time, otherwise the main module version embedded in the build info
// (as recorded by "go install <module>@vX.Y.Z"), otherwise "dev". It never
// returns an empty string.
func Get() string {
	if Current != "dev" {
		return Current
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Current
}
