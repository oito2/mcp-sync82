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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/oito2/mcp-sync82/internal/fsutil"
)

// File names inside the data directory: the global config and the lock
// file guarding its updates.
const (
	globalConfigFileName = "config.json"
	globalLockFileName   = "config.lock"
)

// GlobalConfig is the content of ~/.sync82/config.json: the last
// project/subproject used and the vault it lives in (the fallback tier of
// context resolution), plus an optional custom vault path set by
// "sync82 config set-vault".
type GlobalConfig struct {
	LastProject    string `json:"lastProject,omitempty"`
	LastSubproject string `json:"lastSubproject,omitempty"`
	LastVaultPath  string `json:"lastVaultPath,omitempty"`
	VaultPath      string `json:"vaultPath,omitempty"`
}

// GlobalConfigPath returns the path to ~/.sync82/config.json. It returns
// an error when the user's home directory cannot be determined.
func GlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, defaultVaultDir, globalConfigFileName), nil
}

// ReadGlobalConfig reads the global config. A missing file is not an
// error: it returns the zero-value config. The read holds a shared lock on
// ~/.sync82/config.lock, so it never has the file open while
// UpdateGlobalConfig, which holds the exclusive lock, renames a new file
// over it (Windows refuses that rename while the file is open); in a
// read-only ~/.sync82 or on a read-only filesystem, where the lock file
// can't be created, it reads without the lock. It returns an error when the
// path cannot be resolved, the lock cannot be taken, the file cannot be
// read, or its content is not valid JSON.
func ReadGlobalConfig() (GlobalConfig, error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return GlobalConfig{}, err
	}

	unlock, err := lockGlobalConfigShared(filepath.Dir(path))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return GlobalConfig{}, nil // no ~/.sync82 yet, so no config
	case errors.Is(err, os.ErrPermission) || isReadOnlyFS(err):
		// A read-only ~/.sync82, or a read-only filesystem, can't hold a
		// new lock file, and no update can rename the config there
		// either: read it unlocked.
	case err != nil:
		return GlobalConfig{}, err
	default:
		defer unlock()
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, nil
	}
	if err != nil {
		return GlobalConfig{}, fmt.Errorf("read global config %s: %w", path, err)
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("parse global config %s: %w", path, err)
	}
	return cfg, nil
}

// writeGlobalConfig overwrites the global config atomically with mode
// 0600. It does not take the update lock; use UpdateGlobalConfig for a
// read-modify-write. It returns an error when the path cannot be
// resolved, cfg cannot be encoded, or the file cannot be written.
func writeGlobalConfig(cfg GlobalConfig) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode global config: %w", err)
	}
	return fsutil.AtomicWriteFile(path, data, 0o600)
}

// globalConfigMu serializes the goroutines of this process that
// read-modify-write the global config file. Tool calls run concurrently
// and most end by calling UpdateLastProject; without the mutex, two calls
// could read the same on-disk config and the second write would discard
// the first's update.
var globalConfigMu sync.Mutex

// UpdateGlobalConfig performs a locked read-modify-write of the global
// config: it reads the current config, lets mutate adjust it in place,
// and writes the result back. It is the single entry point for
// read-modify-write changes to the file, such as UpdateLastProject and the
// vault path commands.
//
// Two locks are held for the whole read-modify-write: globalConfigMu,
// which serializes goroutines of this process, and an exclusive advisory
// lock on ~/.sync82/config.lock, which serializes separate processes
// (each MCP client runs its own server process, and the CLI is another
// one). Both are released on every return path.
//
// When mutate leaves the config unchanged, nothing is written, unless a
// corrupt file was just set aside.
//
// A config file that exists but is empty, whitespace-only, or not valid
// JSON is renamed to config.json.corrupt-<unix-timestamp> (keeping its
// permission bits) and the update continues from a zero-value config, so
// the file becomes writable again instead of failing every update.
// ReadGlobalConfig still reports such a file as a parse error.
//
// It returns an error when the path cannot be resolved, the lock cannot be
// taken, the file cannot be read or moved aside, or the write fails.
func UpdateGlobalConfig(mutate func(cfg *GlobalConfig)) error {
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()

	unlock, err := lockGlobalConfig()
	if err != nil {
		return err
	}
	defer unlock()

	cfg, recovered, err := readGlobalConfigForUpdate()
	if err != nil {
		return err
	}
	before := cfg
	mutate(&cfg)
	if cfg == before && !recovered {
		return nil
	}
	return writeGlobalConfig(cfg)
}

// readGlobalConfigForUpdate reads the global config like ReadGlobalConfig,
// except that an empty, whitespace-only, or unparsable file is moved aside
// to config.json.corrupt-<unix-timestamp> (copied there instead when
// config.json is a symlink, which keeps the link) and a zero-value config
// is returned in its place; recovered then reports it, so the caller writes
// a fresh config even when nothing else changes (for a symlink, only a
// write repairs the target). It returns an error when the path cannot be
// resolved or the file cannot be read or moved aside.
func readGlobalConfigForUpdate() (cfg GlobalConfig, recovered bool, err error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return GlobalConfig{}, false, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return GlobalConfig{}, false, nil
	}
	if err != nil {
		return GlobalConfig{}, false, fmt.Errorf("read global config %s: %w", path, err)
	}

	if len(bytes.TrimSpace(data)) > 0 && json.Unmarshal(data, &cfg) == nil {
		return cfg, false, nil
	}

	aside := path + ".corrupt-" + strconv.FormatInt(time.Now().Unix(), 10)
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		// Renaming a symlink (a dotfiles setup) would break the link: its
		// content is copied aside instead, and the next write goes through
		// the link to its target.
		if err := os.WriteFile(aside, data, 0o600); err != nil {
			return GlobalConfig{}, false, fmt.Errorf("copy corrupt global config %s aside: %w", path, err)
		}
		return GlobalConfig{}, true, nil
	}
	if err := os.Rename(path, aside); err != nil {
		return GlobalConfig{}, false, fmt.Errorf("move corrupt global config %s aside: %w", path, err)
	}
	return GlobalConfig{}, true, nil
}

// lockGlobalConfig opens ~/.sync82/config.lock (creating the directory
// with mode 0700 and the file with mode 0600 when missing) and blocks
// until it holds an exclusive advisory lock on it. The returned function
// releases the lock and closes the file. It returns an error when the
// path cannot be resolved, the directory or file cannot be created, or
// the lock cannot be taken.
func lockGlobalConfig() (func(), error) {
	path, err := GlobalConfigPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}

	return openLock(filepath.Join(dir, globalLockFileName), true)
}

// lockGlobalConfigShared takes a shared lock on the config.lock file in
// dir, creating the file with mode 0600 when missing but never dir itself,
// so reading the config never creates ~/.sync82. The returned function
// releases the lock and closes the file. It returns an error wrapping
// os.ErrNotExist when dir does not exist, or an error when the file cannot
// be opened or locked.
func lockGlobalConfigShared(dir string) (func(), error) {
	return openLock(filepath.Join(dir, globalLockFileName), false)
}

// openLock opens the lock file at lockPath, creating it with mode 0600
// when missing, and blocks until it holds a lock on it, exclusive or
// shared. The returned function releases the lock and closes the file.
func openLock(lockPath string, exclusive bool) (func(), error) {
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open global config lock %s: %w", lockPath, err)
	}
	if err := lockFile(f, exclusive); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock global config %s: %w", lockPath, err)
	}
	return func() {
		_ = unlockFile(f)
		f.Close()
	}, nil
}

// UpdateLastProject records the most recently used project/subproject and
// the vault path it lives in, so a later call that passes no project
// argument can fall back to them. Other fields of the config are kept.
// Nothing is written when the values are unchanged. It returns the
// errors of UpdateGlobalConfig; callers may treat them as best-effort.
func UpdateLastProject(project, subproject, vaultPath string) error {
	return UpdateGlobalConfig(func(cfg *GlobalConfig) {
		cfg.LastProject = project
		cfg.LastSubproject = subproject
		cfg.LastVaultPath = vaultPath
	})
}
