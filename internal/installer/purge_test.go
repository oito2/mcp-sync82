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
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestPurgeCandidates_ListsOnlySync82Files verifies that only the fixed data files
// and regular config.json.corrupt-* files are listed, and that an absent data
// directory yields none.
func TestPurgeCandidates_ListsOnlySync82Files(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".sync82")
	for _, name := range []string{"knowledge.db", "knowledge.db-wal", "config.json", "config.lock", "config.json.corrupt-1700000000", "notes.txt", "other.db"} {
		writeFile(t, filepath.Join(dir, name), "x", 0o600)
	}
	if err := os.MkdirAll(filepath.Join(dir, "config.json.corrupt-dir"), 0o700); err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "knowledge.db"),
		filepath.Join(dir, "knowledge.db-wal"),
		filepath.Join(dir, "config.json"),
		filepath.Join(dir, "config.lock"),
		filepath.Join(dir, "config.json.corrupt-1700000000"),
	}
	if got := PurgeCandidates(home); !slices.Equal(got, want) {
		t.Fatalf("PurgeCandidates() = %q, want %q", got, want)
	}
	if got := PurgeCandidates(t.TempDir()); len(got) != 0 {
		t.Errorf("PurgeCandidates(empty home) = %q, want none", got)
	}
}

// TestDeleteFiles_RemovesDataDirOnlyWhenEmpty verifies that unrelated files survive a
// purge and that the data directory is removed only once it is empty.
func TestDeleteFiles_RemovesDataDirOnlyWhenEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".sync82")
	writeFile(t, filepath.Join(dir, "knowledge.db"), "x", 0o600)
	writeFile(t, filepath.Join(dir, "notes.txt"), "x", 0o600)

	if errs := DeleteFiles(home, PurgeCandidates(home)); len(errs) != 0 {
		t.Fatalf("DeleteFiles() = %v", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "config.json"), "{}", 0o600)
	if errs := DeleteFiles(home, PurgeCandidates(home)); len(errs) != 0 {
		t.Fatalf("DeleteFiles() = %v", errs)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty data directory left behind: %v", err)
	}
}

// TestConfiguredVaults verifies that vaults from SYNC82_DB_PATH, the global config
// and a parent directory's .sync82.json are listed once each, with the default
// vault excluded and relative paths resolved against the config root.
func TestConfiguredVaults(t *testing.T) {
	home := t.TempDir()
	envVault := filepath.Join(home, "env", "vault.db")
	globalVault := filepath.Join(home, "global", "vault.db")
	writeFile(t, filepath.Join(home, ".sync82", "config.json"),
		`{"vaultPath":"`+filepath.ToSlash(globalVault)+`","lastVaultPath":"`+filepath.ToSlash(filepath.Join(home, ".sync82", "knowledge.db"))+`"}`, 0o600)
	workspace := filepath.Join(home, "work", "project")
	writeFile(t, filepath.Join(home, "work", ".sync82.json"), `{"project":"p","path":"vaults/local.db"}`, 0o644)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	env := testEnv("linux", home, map[string]string{"SYNC82_DB_PATH": envVault})
	got := ConfiguredVaults(env, workspace)
	want := []string{envVault, filepath.Clean(globalVault), filepath.Join(home, "work", "vaults", "local.db")}
	if !slices.Equal(got, want) {
		t.Fatalf("ConfiguredVaults() = %q, want %q", got, want)
	}
	if got := ConfiguredVaults(testEnv("linux", t.TempDir(), nil), ""); len(got) != 0 {
		t.Errorf("ConfiguredVaults(nothing configured) = %q, want none", got)
	}
}
