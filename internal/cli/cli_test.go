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

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestRunConfig_GetVault_DefaultsToNoneConfigured verifies that get-vault reports no configured vault on a fresh home.
func TestRunConfig_GetVault_DefaultsToNoneConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	t.Setenv(config.DBPathEnvVar, "")
	var stdout, stderr bytes.Buffer

	code := RunConfig([]string{"get-vault"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "No global vault configured. Using the default: " + config.DefaultVaultPath()
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

// TestRunConfig_GetVault_NamesTheEnvironmentVariable verifies that, with no
// global vault, get-vault reports the SYNC82_DB_PATH vault and names the
// variable.
func TestRunConfig_GetVault_NamesTheEnvironmentVariable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	envVault := filepath.Join(t.TempDir(), "env.db")
	t.Setenv(config.DBPathEnvVar, envVault)
	var stdout, stderr bytes.Buffer

	if code := RunConfig([]string{"get-vault"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "No global vault configured. Using SYNC82_DB_PATH: " + envVault
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

// TestRunConfig_SetVaultThenGetVault verifies that set-vault persists a path that get-vault then prints.
func TestRunConfig_SetVaultThenGetVault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	vault := filepath.Join(t.TempDir(), "vault.db")
	var stdout, stderr bytes.Buffer

	if code := RunConfig([]string{"set-vault", vault}, &stdout, &stderr); code != 0 {
		t.Fatalf("set-vault: code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), vault) {
		t.Errorf("set-vault stdout = %q, want it to echo the path", stdout.String())
	}

	stdout.Reset()
	if code := RunConfig([]string{"get-vault"}, &stdout, &stderr); code != 0 {
		t.Fatalf("get-vault: code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), vault) {
		t.Errorf("get-vault stdout = %q, want it to report the configured path", stdout.String())
	}

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg.VaultPath != vault {
		t.Errorf("cfg.VaultPath = %q, want %q", cfg.VaultPath, vault)
	}
}

// TestRunConfig_SetVault_ExpandsHomeToken verifies that a leading home token in the path is expanded before
// it is stored.
func TestRunConfig_SetVault_ExpandsHomeToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer

	if code := RunConfig([]string{"set-vault", "HOME/my-vault.db"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	want := filepath.Join(home, "my-vault.db")
	if cfg.VaultPath != want {
		t.Errorf("cfg.VaultPath = %q, want %q", cfg.VaultPath, want)
	}
}

// TestRunConfig_SetVault_StoresRelativePathAsAbsolute guards against a
// relative path being stored verbatim and then resolved against each MCP
// client's own working directory.
func TestRunConfig_SetVault_StoresRelativePathAsAbsolute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dir := t.TempDir()
	t.Chdir(dir)
	var stdout, stderr bytes.Buffer

	if code := RunConfig([]string{"set-vault", "./rel.db"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code = %d, want 0; stderr=%q", code, stderr.String())
	}

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if want := filepath.Join(dir, "rel.db"); cfg.VaultPath != want {
		t.Errorf("cfg.VaultPath = %q, want %q", cfg.VaultPath, want)
	}
}

// TestRunConfig_UnsetVault_ClearsConfiguredPath verifies that unset-vault removes the stored path.
func TestRunConfig_UnsetVault_ClearsConfiguredPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer

	if code := RunConfig([]string{"set-vault", "/custom/vault.db"}, &stdout, &stderr); code != 0 {
		t.Fatalf("set-vault: code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()

	if code := RunConfig([]string{"unset-vault"}, &stdout, &stderr); code != 0 {
		t.Fatalf("unset-vault: code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed") {
		t.Errorf("unset-vault stdout = %q, want it to confirm removal", stdout.String())
	}

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg.VaultPath != "" {
		t.Errorf("cfg.VaultPath = %q, want empty after unset-vault", cfg.VaultPath)
	}
}

// TestRunConfig_SetVault_RequiresPathArgument verifies that set-vault without
// a path is a usage error: exit code 2 and the set-vault usage.
func TestRunConfig_SetVault_RequiresPathArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer

	code := RunConfig([]string{"set-vault"}, &stdout, &stderr)
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d when the path argument is missing", code, usageExitCode)
	}
	if !strings.Contains(stderr.String(), "usage: sync82 config set-vault <path>") {
		t.Errorf("stderr = %q, want a usage message", stderr.String())
	}
}

// TestRunConfig_UnknownSubcommand verifies that an unknown subcommand is a
// usage error (exit code 2) that lists the available subcommands.
func TestRunConfig_UnknownSubcommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer

	code := RunConfig([]string{"bogus"}, &stdout, &stderr)
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d for an unknown subcommand", code, usageExitCode)
	}
	if !strings.Contains(stderr.String(), `unknown config subcommand "bogus" (available: set-vault, get-vault, unset-vault)`) {
		t.Errorf("stderr = %q, want it to report the unknown subcommand", stderr.String())
	}
}

// TestRunConfig_NoSubcommand verifies that a missing subcommand exits with 1.
func TestRunConfig_NoSubcommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	var stdout, stderr bytes.Buffer

	code := RunConfig(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected a non-zero exit code when no subcommand is given")
	}
}
