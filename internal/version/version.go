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

import (
	"regexp"
	"runtime/debug"
	"strings"
)

// Current is the release version, set at build time with
//
//	-ldflags "-X github.com/oito2/mcp-sync82/internal/version.Current=vX.Y.Z"
//
// A build without that flag leaves it "dev".
var Current = "dev"

// Get returns the running binary's version: Current when it was set at
// build time, otherwise the main module version embedded in the build info
// when it names a release (as recorded by "go install <module>@vX.Y.Z"),
// otherwise "dev". It never returns an empty string.
func Get() string {
	if Current != "dev" {
		return Current
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := releaseVersion(info.Main.Version); v != "" {
			return v
		}
	}
	return Current
}

// pseudoVersion matches the timestamp-and-commit part of a Go
// pseudo-version, such as the "0.20260101120000-abcdef123456" of
// "v1.0.1-0.20260101120000-abcdef123456".
var pseudoVersion = regexp.MustCompile(`[-.](0\.)?\d{14}-[0-9a-f]{12}`)

// releaseVersion returns v when it is the version of a tagged release, and
// "" otherwise: for an empty version, "(devel)", a build from a modified
// checkout ("+dirty", which "go build" stamps since Go 1.24) and a Go
// pseudo-version of an untagged commit. Those builds are not the release
// they would otherwise be mistaken for, so they report "dev".
func releaseVersion(v string) string {
	if v == "" || v == "(devel)" || strings.Contains(v, "+dirty") || pseudoVersion.MatchString(v) {
		return ""
	}
	return v
}
