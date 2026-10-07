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

package version

import "testing"

// TestGet_PrefersCurrentWhenSet verifies that Get returns Current when it
// differs from "dev".
func TestGet_PrefersCurrentWhenSet(t *testing.T) {
	old := Current
	t.Cleanup(func() { Current = old })
	Current = "v1.2.3"
	if got := Get(); got != "v1.2.3" {
		t.Fatalf("Get() = %q, want %q", got, "v1.2.3")
	}
}

// TestGet_DevBuildIsNotEmpty verifies that Get never returns an empty string.
func TestGet_DevBuildIsNotEmpty(t *testing.T) {
	if Get() == "" {
		t.Fatal("Get() returned an empty version")
	}
}

// TestReleaseVersion verifies which build-info versions name a release.
func TestReleaseVersion(t *testing.T) {
	for v, want := range map[string]string{
		"v1.1.0":                               "v1.1.0",
		"v1.2.0-rc.1":                          "v1.2.0-rc.1",
		"":                                     "",
		"(devel)":                              "",
		"v1.0.0+dirty":                         "",
		"v1.0.1-0.20261007120000-abcdef123456": "",
		"v0.0.0-20261007120000-abcdef123456":   "",
		"v1.2.0-rc.1.0.20261007120000-abcdef123456+dirty": "",
	} {
		if got := releaseVersion(v); got != want {
			t.Errorf("releaseVersion(%q) = %q, want %q", v, got, want)
		}
	}
}
