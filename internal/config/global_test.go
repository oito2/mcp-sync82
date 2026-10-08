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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// TestReadGlobalConfig_MissingFileReturnsZeroValue verifies that a missing config
// file yields the zero-value config and no error.
func TestReadGlobalConfig_MissingFileReturnsZeroValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	cfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg != (GlobalConfig{}) {
		t.Fatalf("expected zero-value config for a fresh install, got %+v", cfg)
	}
}

// TestWriteGlobalConfig_RoundTrip verifies that a written config is read back
// unchanged.
func TestWriteGlobalConfig_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	want := GlobalConfig{LastProject: "oito2", LastSubproject: "sync82", VaultPath: "HOME/custom-vault"}
	if err := writeGlobalConfig(want); err != nil {
		t.Fatalf("writeGlobalConfig: %v", err)
	}

	got, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestUpdateLastProject verifies that UpdateLastProject sets the last-used fields
// and keeps the custom vault path.
func TestUpdateLastProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	if err := writeGlobalConfig(GlobalConfig{VaultPath: "HOME/custom-vault"}); err != nil {
		t.Fatalf("writeGlobalConfig (seed): %v", err)
	}

	if err := UpdateLastProject("oito2", "sync82", "/vaults/v.db"); err != nil {
		t.Fatalf("UpdateLastProject: %v", err)
	}

	cfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg.LastProject != "oito2" || cfg.LastSubproject != "sync82" || cfg.LastVaultPath != "/vaults/v.db" {
		t.Fatalf("got %+v, want LastProject=oito2 LastSubproject=sync82 LastVaultPath=/vaults/v.db", cfg)
	}
	// The seeded VaultPath is a separate field and must survive the update.
	if cfg.VaultPath != "HOME/custom-vault" {
		t.Fatalf("VaultPath was clobbered: got %q", cfg.VaultPath)
	}
}

// TestUpdateGlobalConfig_ConcurrentUpdatesToDifferentFieldsDoNotLoseWrites
// verifies that goroutines updating different fields of the global config
// concurrently never lose each other's writes, whatever the interleaving.
func TestUpdateGlobalConfig_ConcurrentUpdatesToDifferentFieldsDoNotLoseWrites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if err := UpdateGlobalConfig(func(cfg *GlobalConfig) {
				cfg.LastProject = "project-a"
			}); err != nil {
				t.Errorf("UpdateGlobalConfig (project): %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if err := UpdateGlobalConfig(func(cfg *GlobalConfig) {
				cfg.VaultPath = "vault-b"
			}); err != nil {
				t.Errorf("UpdateGlobalConfig (vault): %v", err)
			}
		}
	}()

	wg.Wait()

	cfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg.LastProject != "project-a" {
		t.Errorf("LastProject = %q, want %q — a concurrent write to a different field lost this update", cfg.LastProject, "project-a")
	}
	if cfg.VaultPath != "vault-b" {
		t.Errorf("VaultPath = %q, want %q — a concurrent write to a different field lost this update", cfg.VaultPath, "vault-b")
	}
}

// TestWriteGlobalConfig_IsPrivate verifies that the global config is created
// with mode 0600.
func TestWriteGlobalConfig_IsPrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	if err := writeGlobalConfig(GlobalConfig{LastProject: "acme"}); err != nil {
		t.Fatal(err)
	}
	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %v, want 0600", got)
	}
}

// TestUpdateGlobalConfig_UnchangedDoesNotRewrite verifies that
// UpdateLastProject leaves the file untouched when the values are
// unchanged and rewrites it when they differ.
func TestUpdateGlobalConfig_UnchangedDoesNotRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))

	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Compact JSON differs from writeGlobalConfig's indented output, so any
	// rewrite changes the bytes on disk.
	seed := []byte(`{"lastProject":"oito2","lastSubproject":"sync82","lastVaultPath":"/v.db"}`)
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := UpdateLastProject("oito2", "sync82", "/v.db"); err != nil {
		t.Fatalf("UpdateLastProject: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seed) {
		t.Fatalf("unchanged update rewrote the file:\n%s", got)
	}

	if err := UpdateLastProject("oito2", "other", "/v.db"); err != nil {
		t.Fatalf("UpdateLastProject: %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, seed) {
		t.Fatal("changed update did not rewrite the file")
	}
}

// TestUpdateGlobalConfig_RecoversCorruptFile verifies that an empty,
// whitespace-only or unparsable config.json is moved aside, keeping its
// mode, and that the update then succeeds.
func TestUpdateGlobalConfig_RecoversCorruptFile(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"whitespace": " \n\t\n",
		"invalid":    `{"lastProject": "oito2"`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("USERPROFILE", os.Getenv("HOME"))

			path, err := GlobalConfigPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o640); err != nil {
				t.Fatal(err)
			}

			if _, err := ReadGlobalConfig(); err == nil {
				t.Fatal("ReadGlobalConfig accepted a corrupt config")
			}

			if err := UpdateGlobalConfig(func(cfg *GlobalConfig) {
				cfg.VaultPath = "/vaults/v.db"
			}); err != nil {
				t.Fatalf("UpdateGlobalConfig: %v", err)
			}

			cfg, err := ReadGlobalConfig()
			if err != nil {
				t.Fatalf("ReadGlobalConfig after recovery: %v", err)
			}
			if cfg != (GlobalConfig{VaultPath: "/vaults/v.db"}) {
				t.Fatalf("got %+v, want only VaultPath set", cfg)
			}

			matches, err := filepath.Glob(path + ".corrupt-*")
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 1 {
				t.Fatalf("expected one moved-aside file, got %v", matches)
			}
			aside, err := os.ReadFile(matches[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(aside) != content {
				t.Fatalf("moved-aside content = %q, want %q", aside, content)
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(matches[0])
				if err != nil {
					t.Fatal(err)
				}
				if got := info.Mode().Perm(); got != 0o640 {
					t.Fatalf("moved-aside mode = %v, want 0640", got)
				}
			}
		})
	}
}

// helperProcessEnv is the environment variable that turns the cross-process
// helper test into a worker; helperProcesses is the number of worker
// processes and helperIncrements the increments each performs.
const (
	helperProcessEnv = "SYNC82_CONFIG_HELPER_PROCESS"
	helperProcesses  = 4
	helperIncrements = 40
)

// TestUpdateGlobalConfig_CrossProcessHelper is the body run by each
// subprocess spawned by TestUpdateGlobalConfig_CrossProcessNoLostUpdates:
// it increments the counter stored in LastSubproject helperIncrements
// times. It does nothing when run as a regular test.
func TestUpdateGlobalConfig_CrossProcessHelper(t *testing.T) {
	if os.Getenv(helperProcessEnv) != "1" {
		t.Skip("helper process only")
	}
	for i := 0; i < helperIncrements; i++ {
		var convErr error
		err := UpdateGlobalConfig(func(cfg *GlobalConfig) {
			n := 0
			if cfg.LastSubproject != "" {
				n, convErr = strconv.Atoi(cfg.LastSubproject)
			}
			cfg.LastSubproject = strconv.Itoa(n + 1)
			if i%2 == 0 {
				cfg.VaultPath = fmt.Sprintf("/vault-%d-%d", os.Getpid(), i)
			} else {
				cfg.LastProject = fmt.Sprintf("project-%d-%d", os.Getpid(), i)
			}
		})
		if err != nil {
			t.Fatalf("UpdateGlobalConfig: %v", err)
		}
		if convErr != nil {
			t.Fatalf("counter is not a number: %v", convErr)
		}
	}
}

// TestUpdateGlobalConfig_CrossProcessNoLostUpdates verifies that several
// processes incrementing a shared counter in the global config lose no
// update, which relies on the file lock.
func TestUpdateGlobalConfig_CrossProcessNoLostUpdates(t *testing.T) {
	if os.Getenv(helperProcessEnv) == "1" {
		t.Skip("running inside a helper process")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for p := 0; p < helperProcesses; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(exe, "-test.run=^TestUpdateGlobalConfig_CrossProcessHelper$", "-test.count=1")
			cmd.Env = append(os.Environ(), helperProcessEnv+"=1")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("helper process: %v\n%s", err, out)
			}
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}

	cfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	want := strconv.Itoa(helperProcesses * helperIncrements)
	if cfg.LastSubproject != want {
		t.Fatalf("counter = %q, want %s — a concurrent process lost an update", cfg.LastSubproject, want)
	}
}

// TestGlobalConfig_ConcurrentReadersAndWriters runs readers and writers of
// the global config at the same time. Every read sees a complete config
// and every write succeeds; on Windows this depends on readers holding the
// shared lock while the writer renames the file.
func TestGlobalConfig_ConcurrentReadersAndWriters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	if err := UpdateLastProject("p0", "", "/vaults/v.db"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 400)
	for w := range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := range 25 {
				errs <- UpdateLastProject(fmt.Sprintf("p%d-%d", w, i), "", "/vaults/v.db")
			}
		}()
		go func() {
			defer wg.Done()
			for range 25 {
				cfg, err := ReadGlobalConfig()
				if err == nil && (cfg.LastProject == "" || cfg.LastVaultPath != "/vaults/v.db") {
					err = fmt.Errorf("read an incomplete config: %+v", cfg)
				}
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestReadGlobalConfig_CreatesNoDirectory checks that reading the config
// of a user without ~/.sync82 returns the zero value and creates nothing.
func TestReadGlobalConfig_CreatesNoDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg, err := ReadGlobalConfig()
	if err != nil || cfg != (GlobalConfig{}) {
		t.Fatalf("ReadGlobalConfig = %+v, %v; want the zero config", cfg, err)
	}
	if _, err := os.Stat(filepath.Join(home, defaultVaultDir)); !os.IsNotExist(err) {
		t.Errorf("~/.sync82 was created by a read (stat err = %v)", err)
	}
}

// TestUpdateGlobalConfig_CorruptSymlinkKeepsTheLink checks that a corrupt
// config.json that is a symlink keeps its link: its content is copied
// aside and the update is written through the link to its target.
func TestUpdateGlobalConfig_CorruptSymlinkKeepsTheLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target := filepath.Join(t.TempDir(), "dotfiles-config.json")
	if err := os.WriteFile(target, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := UpdateLastProject("acme", "", "/vaults/v.db"); err != nil {
		t.Fatalf("UpdateLastProject: %v", err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config.json is no longer a symlink (err %v)", err)
	}
	if data, _ := os.ReadFile(target); !bytes.Contains(data, []byte(`"lastProject": "acme"`)) {
		t.Errorf("target = %s, want the update written through the link", data)
	}
	aside, _ := filepath.Glob(path + ".corrupt-*")
	if len(aside) != 1 {
		t.Fatalf("copies aside = %v, want one", aside)
	}
	if data, _ := os.ReadFile(aside[0]); string(data) != "{not json" {
		t.Errorf("copy aside = %q, want the corrupt content", data)
	}
}

// TestUpdateGlobalConfig_RepairsACorruptSymlinkOnANoOpUpdate checks that an
// update that changes nothing still repairs a corrupt config.json that is
// a symlink, writing a valid config through the link, so later reads work.
func TestUpdateGlobalConfig_RepairsACorruptSymlinkOnANoOpUpdate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks needs a privilege on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target := filepath.Join(t.TempDir(), "dotfiles-config.json")
	if err := os.WriteFile(target, []byte("garbage{"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	if err := UpdateGlobalConfig(func(cfg *GlobalConfig) { cfg.VaultPath = "" }); err != nil {
		t.Fatalf("no-op update: %v", err)
	}
	if _, err := ReadGlobalConfig(); err != nil {
		t.Fatalf("read after the repair: %v", err)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("config.json is no longer a symlink (err %v)", err)
	}
	if aside, _ := filepath.Glob(path + ".corrupt-*"); len(aside) != 1 {
		t.Errorf("copies aside = %v, want one", aside)
	}
	if err := UpdateGlobalConfig(func(cfg *GlobalConfig) { cfg.VaultPath = "" }); err != nil {
		t.Fatal(err)
	}
	if aside, _ := filepath.Glob(path + ".corrupt-*"); len(aside) != 1 {
		t.Errorf("a second no-op update made another copy: %v", aside)
	}
}
