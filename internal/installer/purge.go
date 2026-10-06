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

package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/mcp-sync82/internal/config"
)

// dataDirName is sync82's data directory under the home directory.
const dataDirName = ".sync82"

// purgeFileNames are the fixed files of the data directory a purge
// deletes: the default vault with its SQLite WAL and shared-memory files,
// and the global config with its lock file.
var purgeFileNames = []string{
	"knowledge.db",
	"knowledge.db-wal",
	"knowledge.db-shm",
	"config.json",
	"config.lock",
}

// corruptConfigPrefix starts the name of a global config file moved
// aside because it could not be parsed.
const corruptConfigPrefix = "config.json.corrupt-"

// DataDir returns sync82's data directory, <homeDir>/.sync82.
func DataDir(homeDir string) string {
	return filepath.Join(homeDir, dataDirName)
}

// PurgeCandidates returns every existing file a purge deletes from the
// data directory under homeDir: the purgeFileNames entries plus every
// config.json.corrupt-* file. Nothing else in the directory is listed.
func PurgeCandidates(homeDir string) []string {
	dir := DataDir(homeDir)
	var files []string
	for _, name := range purgeFileNames {
		if p := filepath.Join(dir, name); fileExists(p) {
			files = append(files, p)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasPrefix(e.Name(), corruptConfigPrefix) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// ConfiguredVaults returns the vault paths configured away from the
// default <homeDir>/.sync82/knowledge.db, which a purge leaves untouched:
// the SYNC82_DB_PATH variable, the global config's vaultPath and
// lastVaultPath, and the "path" of a .sync82.json found at cwd or one of
// its parents. Paths are made absolute and listed once each.
func ConfiguredVaults(env Env, cwd string) []string {
	defaultVault := filepath.Join(DataDir(env.HomeDir), "knowledge.db")
	seen := map[string]bool{defaultVault: true}
	var vaults []string
	add := func(raw, base string) {
		if raw == "" {
			return
		}
		p := config.ResolvePath(raw)
		if !filepath.IsAbs(p) && base != "" {
			p = filepath.Join(base, p)
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if !seen[p] {
			seen[p] = true
			vaults = append(vaults, p)
		}
	}

	add(env.getenv("SYNC82_DB_PATH"), "")
	if raw, err := os.ReadFile(filepath.Join(DataDir(env.HomeDir), "config.json")); err == nil {
		var cfg config.GlobalConfig
		if json.Unmarshal(raw, &cfg) == nil {
			add(cfg.VaultPath, "")
			add(cfg.LastVaultPath, "")
		}
	}
	if cwd != "" {
		if local, err := config.ReadLocalConfig(cwd, true); err == nil && local != nil {
			add(local.Config.Path, local.ConfigRoot)
		}
	}
	return vaults
}

// DeleteFiles removes every file in files, then removes the data
// directory under homeDir when it is left empty. It returns one error per
// file that could not be removed; a file already gone is not an error.
func DeleteFiles(homeDir string, files []string) []error {
	var errs []error
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", f, err))
		}
	}
	// Fails, and is ignored, when the directory still holds other files.
	_ = os.Remove(DataDir(homeDir))
	return errs
}
