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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/selfupdate"
)

// TestValidVersion_StrictByDefault checks that only vMAJOR.MINOR.PATCH is
// accepted without -allow-prerelease, and that no accepted value can carry
// spaces or quotes into the -ldflags string.
func TestValidVersion_StrictByDefault(t *testing.T) {
	for _, v := range []string{"v0.1.0", "v1.2.3", "v10.20.30"} {
		if !validVersion(v, false) || !validVersion(v, true) {
			t.Errorf("expected %q to be accepted", v)
		}
	}

	for _, v := range []string{"v0.0.0-ci", "v1.2.3-rc.1", "v1.2.3-beta-2"} {
		if validVersion(v, false) {
			t.Errorf("expected %q to be rejected without -allow-prerelease", v)
		}
		if !validVersion(v, true) {
			t.Errorf("expected %q to be accepted with -allow-prerelease", v)
		}
	}

	invalid := []string{
		"0.1.0",
		"v1.2",
		"v1.2.3.4",
		"v1.2.3-",
		"v1.2.3+build",
		"v1.2.3 extra flags",
		`v1.2.3" -X foo=bar`,
		"v1.2.3-ci -X foo=bar",
		"vX.Y.Z",
		"",
	}
	for _, v := range invalid {
		if validVersion(v, false) || validVersion(v, true) {
			t.Errorf("expected %q to be rejected", v)
		}
	}
}

// TestReleaseLDFlags_StripsAndStampsVersion checks that release builds drop
// the symbol table and DWARF info, clear the build ID, and stamp the
// version into internal/version.Current.
func TestReleaseLDFlags_StripsAndStampsVersion(t *testing.T) {
	got := strings.Fields(releaseLDFlags("v1.2.3"))
	want := []string{"-s", "-w", "-buildid=", "-X", module + "/internal/version.Current=v1.2.3"}
	if !slices.Equal(got, want) {
		t.Errorf("releaseLDFlags = %q, want %q", got, want)
	}
}

// TestPlatforms_MatchSelfUpdateAssetNames checks that every release target
// yields a distinct asset name in the sync82_<goos>_<goarch> form
// self-update downloads.
func TestPlatforms_MatchSelfUpdateAssetNames(t *testing.T) {
	want := []string{
		"sync82_darwin_amd64", "sync82_darwin_arm64",
		"sync82_linux_amd64", "sync82_linux_arm64",
		"sync82_windows_amd64.exe", "sync82_windows_arm64.exe",
	}
	var got []string
	for _, p := range selfupdate.ReleasePlatforms() {
		got = append(got, selfupdate.AssetName(p[0], p[1]))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("release assets = %q, want %q", got, want)
	}
}

// TestWriteChecksumsFile_SHA256SumFormat checks that checksums.txt holds
// one "<64 hex digits>  <bare name>" line per file, sorted by name.
func TestWriteChecksumsFile_SHA256SumFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checksums.txt")
	sums := map[string]string{
		"sync82_linux_amd64":  strings.Repeat("a", 64),
		bundleName:            strings.Repeat("b", 64),
		"sync82_darwin_arm64": strings.Repeat("c", 64),
	}
	if err := writeChecksumsFile(path, sums); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	line := regexp.MustCompile(`^[0-9a-f]{64}  [^/ *]+$`)
	var names []string
	for _, l := range lines {
		if !line.MatchString(l) {
			t.Errorf("line %q is not \"<sha256>  <bare name>\"", l)
			continue
		}
		sum, name, _ := strings.Cut(l, "  ")
		if sums[name] != sum {
			t.Errorf("%s: sum %s, want %s", name, sum, sums[name])
		}
		names = append(names, name)
	}
	if len(names) != len(sums) || !slices.IsSorted(names) {
		t.Errorf("expected %d lines sorted by name, got %q", len(sums), names)
	}
}

// TestReleaseBuildEnv_OverridesTheShell checks that the pinned entries come
// after the caller's environment, so a GOAMD64 or GOFLAGS set in the shell
// doesn't change the release binaries: the last entry of a key wins.
func TestReleaseBuildEnv_OverridesTheShell(t *testing.T) {
	t.Setenv("GOAMD64", "v3")
	t.Setenv("GOFLAGS", "-tags=dev")
	env := append(os.Environ(), releaseBuildEnv("linux", "amd64")...)
	last := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		last[k] = v
	}
	for k, want := range map[string]string{"GOAMD64": "v1", "GOFLAGS": "", "CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "amd64", "GOARM64": "v8.0", "GOEXPERIMENT": ""} {
		if last[k] != want {
			t.Errorf("%s = %q, want %q", k, last[k], want)
		}
	}
}
