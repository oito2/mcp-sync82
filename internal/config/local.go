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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/oito2/mcp-sync82/internal/fsutil"
)

// localConfigFileName is the name of the per-workspace marker file.
const localConfigFileName = ".sync82.json"

// LocalConfig is the content of a workspace's .sync82.json: the
// project/subproject (and optionally the vault path) the workspace maps
// to, so tool calls need no explicit project argument.
type LocalConfig struct {
	Project    string `json:"project"`
	Subproject string `json:"subproject,omitempty"`
	Path       string `json:"path,omitempty"`
}

// LocalConfigResult bundles a found LocalConfig with the absolute
// directory it was found in (ConfigRoot), against which a relative Path
// is resolved.
type LocalConfigResult struct {
	Config     LocalConfig
	ConfigRoot string
}

// maxLocalConfigWalkDepth bounds how many directories ReadLocalConfig
// checks when searchAncestors is true. A real path reaches the filesystem
// root in far fewer steps; the cap keeps a path that never reaches the
// root, such as one resolved through a symlink cycle, from looping
// indefinitely.
const maxLocalConfigWalkDepth = 64

// ReadLocalConfig looks for a .sync82.json file at workspaceRoot. When
// searchAncestors is true, it also walks upward through parent
// directories (up to maxLocalConfigWalkDepth directories in total, or the
// filesystem root, whichever comes first). Ancestor search is opt-in
// because a file found in an ancestor can redirect where memory is stored
// through its "path" field, so otherwise only workspaceRoot is trusted.
//
// It returns (nil, nil) when no file is found. It returns an error when
// workspaceRoot cannot be made absolute, or a candidate file cannot be
// read (other than not existing) or is not valid JSON.
func ReadLocalConfig(workspaceRoot string, searchAncestors bool) (*LocalConfigResult, error) {
	dir, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root %s: %w", workspaceRoot, err)
	}

	maxDepth := 1
	if searchAncestors {
		maxDepth = maxLocalConfigWalkDepth
	}

	for depth := 0; depth < maxDepth; depth++ {
		candidate := filepath.Join(dir, localConfigFileName)
		data, err := os.ReadFile(candidate)
		switch {
		case err == nil:
			var cfg LocalConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parse local config %s: %w", candidate, err)
			}
			return &LocalConfigResult{Config: cfg, ConfigRoot: dir}, nil
		case errors.Is(err, os.ErrNotExist):
			// Not here; continue upward when allowed.
		default:
			return nil, fmt.Errorf("read local config %s: %w", candidate, err)
		}

		if !searchAncestors {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil // reached the filesystem root without finding one
		}
		dir = parent
	}
	return nil, nil // gave up after maxLocalConfigWalkDepth parent directories
}

// WriteLocalConfig atomically writes cfg as .sync82.json directly in
// workspaceRoot, without searching ancestors. It returns an error when the
// path cannot be made absolute, cfg cannot be encoded, or the file cannot
// be written.
func WriteLocalConfig(workspaceRoot string, cfg LocalConfig) error {
	dir, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace root %s: %w", workspaceRoot, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode local config: %w", err)
	}
	return fsutil.AtomicWriteFile(filepath.Join(dir, localConfigFileName), data, 0o644)
}
