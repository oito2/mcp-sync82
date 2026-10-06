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
		t.Setenv(dbPathEnvVar, "")
		want := filepath.Join(home, defaultVaultDir, defaultVaultFile)
		if got := DefaultVaultPath(); got != want {
			t.Errorf("DefaultVaultPath() = %q, want %q", got, want)
		}
	})

	t.Run("env override wins", func(t *testing.T) {
		t.Setenv(dbPathEnvVar, "HOME/custom-vault/knowledge.db")
		want := filepath.Join(home, "custom-vault", "knowledge.db")
		if got := DefaultVaultPath(); got != want {
			t.Errorf("DefaultVaultPath() = %q, want %q", got, want)
		}
	})
}
