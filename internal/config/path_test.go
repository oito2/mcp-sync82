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

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestResolvePath verifies the expansion of HOME, $HOME and ~ prefixes and that
// other values are returned unchanged.
func TestResolvePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"home token", "HOME", home},
		{"home token with subpath", "HOME/custom-vault", filepath.Join(home, "custom-vault")},
		{"dollar home token", "$HOME/custom-vault", filepath.Join(home, "custom-vault")},
		{"tilde alone", "~", home},
		{"tilde with subpath", "~/custom-vault", filepath.Join(home, "custom-vault")},
		{"absolute path passthrough", "/opt/vaults/team.db", "/opt/vaults/team.db"},
		{"relative path passthrough", "relative/vault.db", "relative/vault.db"},
		{"token prefix of a name", "HOMEWORK/vault.db", "HOMEWORK/vault.db"},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			raw  string
			want string
		}{"tilde with backslash", `~\custom-vault`, filepath.Join(home, "custom-vault")})
	} else {
		tests = append(tests, struct {
			name string
			raw  string
			want string
		}{"backslash is part of the name", `~\custom-vault`, `~\custom-vault`})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolvePath(tt.raw); got != tt.want {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestDefaultVaultPath verifies the default vault location and the
// SYNC82_DB_PATH override.
func TestDefaultVaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	t.Run("no env override", func(t *testing.T) {
		t.Setenv(DBPathEnvVar, "")
		want := filepath.Join(home, defaultVaultDir, defaultVaultFile)
		if got := DefaultVaultPath(); got != want {
			t.Errorf("DefaultVaultPath() = %q, want %q", got, want)
		}
	})

	t.Run("env override wins", func(t *testing.T) {
		t.Setenv(DBPathEnvVar, "HOME/custom-vault/knowledge.db")
		want := filepath.Join(home, "custom-vault", "knowledge.db")
		if got := DefaultVaultPath(); got != want {
			t.Errorf("DefaultVaultPath() = %q, want %q", got, want)
		}
	})
}

// TestDefaultVaultPath_RelativeEnvIsMadeAbsolute checks that a relative
// SYNC82_DB_PATH is made absolute against the current directory, so it
// names the same file after the directory changes.
func TestDefaultVaultPath_RelativeEnvIsMadeAbsolute(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv(DBPathEnvVar, "rel.db")
	got := DefaultVaultPath()
	want, _ := filepath.Abs("rel.db")
	if got != want || !filepath.IsAbs(got) {
		t.Errorf("DefaultVaultPath() = %q, want %q", got, want)
	}
}
