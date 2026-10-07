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

// TestReadLocalConfig_WalksUpDirectories verifies that, with ancestor search
// enabled, a .sync82.json in a parent directory is found and its directory
// reported as the config root.
func TestReadLocalConfig_WalksUpDirectories(t *testing.T) {
	root := t.TempDir()
	if err := WriteLocalConfig(root, LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	result, err := ReadLocalConfig(deep, true)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if result == nil {
		t.Fatal("expected .sync82.json to be discovered by walking up")
	}
	if result.Config.Project != "oito2" || result.Config.Subproject != "sync82" {
		t.Fatalf("got config %+v", result.Config)
	}

	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if result.ConfigRoot != wantRoot {
		t.Fatalf("ConfigRoot = %q, want %q", result.ConfigRoot, wantRoot)
	}
}

// TestReadLocalConfig_NotFound verifies that (nil, nil) is returned when no
// .sync82.json exists.
func TestReadLocalConfig_NotFound(t *testing.T) {
	// Use a fresh temp dir so no real .sync82.json above it can be found.
	root := t.TempDir()

	result, err := ReadLocalConfig(root, true)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no local config to be found, got %+v", result)
	}
}

// TestReadLocalConfig_DoesNotSearchAncestorsByDefault verifies that without
// searchAncestors only the given directory is checked, so a .sync82.json in
// a parent directory is ignored, while the same lookup with searchAncestors
// finds it.
func TestReadLocalConfig_DoesNotSearchAncestorsByDefault(t *testing.T) {
	root := t.TempDir()
	if err := WriteLocalConfig(root, LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	result, err := ReadLocalConfig(deep, false)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no config to be found without searchAncestors, got %+v", result)
	}

	// With searchAncestors the same file is found, so the miss above is not
	// caused by a broken lookup.
	result, err = ReadLocalConfig(deep, true)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if result == nil {
		t.Fatal("expected .sync82.json to be discovered when searchAncestors=true")
	}
}

// TestReadLocalConfig_StopsAtMaxWalkDepth verifies that the upward walk is
// bounded by maxLocalConfigWalkDepth: with a .sync82.json only reachable
// beyond that many directories, the lookup gives up with (nil, nil).
func TestReadLocalConfig_StopsAtMaxWalkDepth(t *testing.T) {
	root := t.TempDir()
	if err := WriteLocalConfig(root, LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	deep := root
	for i := 0; i < maxLocalConfigWalkDepth+10; i++ {
		deep = filepath.Join(deep, "d")
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	result, err := ReadLocalConfig(deep, true)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if result != nil {
		t.Fatalf("expected the walk to give up before reaching the config at depth %d beyond the cap, got %+v", maxLocalConfigWalkDepth+10, result)
	}
}

// TestWriteLocalConfig_WritesAtWorkspaceRootNotWalkedUp verifies that the file is
// written in the given directory and not in a parent.
func TestWriteLocalConfig_WritesAtWorkspaceRootNotWalkedUp(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := WriteLocalConfig(sub, LocalConfig{Project: "acme"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, localConfigFileName)); !os.IsNotExist(err) {
		t.Fatal("expected no .sync82.json at the parent — WriteLocalConfig must not walk up")
	}
	if _, err := os.Stat(filepath.Join(sub, localConfigFileName)); err != nil {
		t.Fatalf("expected .sync82.json at the given workspace root: %v", err)
	}
}

// TestLocalConfig_RefusesSymlink verifies that a .sync82.json symlink is
// refused on read and on write, and that the file it points to is never
// changed.
func TestLocalConfig_RefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "claude_desktop_config.json")
	original := `{"mcpServers":{"other":{"command":"x"}}}`
	if err := os.WriteFile(victim, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(dir, localConfigFileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := ReadLocalConfig(dir, false); err == nil {
		t.Error("ReadLocalConfig should refuse a symlinked .sync82.json")
	}
	if err := WriteLocalConfig(dir, LocalConfig{Project: "demo"}); err == nil {
		t.Error("WriteLocalConfig should refuse a symlinked .sync82.json")
	}
	if data, _ := os.ReadFile(victim); string(data) != original {
		t.Fatalf("the symlink target was changed to %q", data)
	}
}
