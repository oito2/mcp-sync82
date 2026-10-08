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
	"strings"
)

// DBPathEnvVar names the environment variable overriding the default vault
// path; defaultVaultDir and defaultVaultFile locate the default vault under
// the home directory.
const (
	DBPathEnvVar     = "SYNC82_DB_PATH"
	defaultVaultDir  = ".sync82"
	defaultVaultFile = "knowledge.db"
)

// ResolvePath expands a leading HOME, $HOME, or ~ token in raw (alone or
// followed by a slash, or on Windows also by a backslash) to the user's
// home directory. Any other value, or any value when the home directory
// cannot be determined, is returned unchanged.
func ResolvePath(raw string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return raw
	}

	for _, token := range []string{"HOME", "$HOME", "~"} {
		if raw == token {
			return home
		}
		rest, ok := strings.CutPrefix(raw, token)
		if ok && rest != "" && (rest[0] == '/' || rest[0] == filepath.Separator) {
			return filepath.Join(home, rest[1:])
		}
	}
	return raw
}

// DefaultVaultPath returns the vault path used when no tool argument,
// local config, or global config supplies one: the SYNC82_DB_PATH
// environment variable (with ResolvePath applied, then made absolute
// against the current directory) if set, otherwise
// ~/.sync82/knowledge.db. When the home directory is unknown, the default
// is relative to the current directory.
func DefaultVaultPath() string {
	if env := os.Getenv(DBPathEnvVar); env != "" {
		// Made absolute once, so every later use names the same file
		// whatever the working directory is then.
		p := ResolvePath(env)
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(defaultVaultDir, defaultVaultFile)
	}
	return filepath.Join(home, defaultVaultDir, defaultVaultFile)
}
