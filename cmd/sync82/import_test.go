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

package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// TestRunImport_TwoArgs_ProjectOnly checks that importing a directory of .md
// files creates the project with their content.
func TestRunImport_TwoArgs_ProjectOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := RunImport(context.Background(), []string{"acme", inputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Imported 1 file into acme") {
		t.Errorf("stdout = %q, want it to report 1 imported file", stdout.String())
	}

	ctx := context.Background()
	s, err := mgr.Get(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	content, ok, err := s.ReadContent(ctx, "acme", "", "memory")
	if err != nil || !ok || content != "hello" {
		t.Fatalf("content = %q ok=%v err=%v", content, ok, err)
	}
}

// TestRunImport_DryRunDoesNotCreateVault checks that --dry-run against a
// vault that does not exist yet reports every file as new and leaves no
// vault file behind.
func TestRunImport_DryRunDoesNotCreateVault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := RunImport(context.Background(), []string{"acme", inputDir, "--dry-run"}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would import 1 file") || !strings.Contains(stdout.String(), "New: memory.md") {
		t.Errorf("stdout = %q, want a dry-run report with memory.md as new", stdout.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("vault file exists after a dry run (stat err = %v)", err)
	}
}

// TestRunImport_UsesConfiguredGlobalVault checks that the import command
// writes to the vault set with "sync82 config set-vault" rather than the
// default vault.
func TestRunImport_UsesConfiguredGlobalVault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	defaultDB := filepath.Join(t.TempDir(), "default.db")
	customDB := filepath.Join(t.TempDir(), "custom.db")
	if err := config.WriteGlobalConfig(config.GlobalConfig{VaultPath: customDB}); err != nil {
		t.Fatal(err)
	}
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(defaultDB, slog.New(slog.DiscardHandler))

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := RunImport(context.Background(), []string{"acme", inputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr}); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	ctx := context.Background()
	s, err := mgr.Get(ctx, customDB)
	if err != nil {
		t.Fatal(err)
	}
	if content, ok, err := s.ReadContent(ctx, "acme", "", "memory"); err != nil || !ok || content != "hello" {
		t.Fatalf("custom vault content = %q ok=%v err=%v", content, ok, err)
	}
	if _, err := os.Stat(defaultDB); !os.IsNotExist(err) {
		t.Errorf("default vault %s should not have been created (err=%v)", defaultDB, err)
	}
}

// TestRunImport_ThreeArgs_WithSubproject checks that importing into a
// subproject reports it as project/subproject.
func TestRunImport_ThreeArgs_WithSubproject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("sub content"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := RunImport(context.Background(), []string{"acme", "plugin", inputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "acme/plugin") {
		t.Errorf("stdout = %q, want it to mention acme/plugin", stdout.String())
	}
}

// TestRunImport_WrongArgCount checks that a wrong number of positional
// arguments is a usage error: exit code 2 and a usage message.
func TestRunImport_WrongArgCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	code := RunImport(context.Background(), []string{"only-one-arg"}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(stderr.String(), "usage:") || !strings.Contains(stderr.String(), "Run 'sync82 --help' for usage.") {
		t.Errorf("stderr = %q, want a usage message", stderr.String())
	}
}

// TestRunImport_NonexistentInputDir checks that a missing input directory
// exits with code 1.
func TestRunImport_NonexistentInputDir(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	code := RunImport(context.Background(), []string{"acme", filepath.Join(t.TempDir(), "does-not-exist")}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// TestRunImport_RoundTripsWithExport checks that exporting a project and
// importing the result under a new name restores the original content.
func TestRunImport_RoundTripsWithExport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	ctx := context.Background()
	s, err := mgr.Get(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "original"); err != nil {
		t.Fatal(err)
	}

	exportDir := filepath.Join(t.TempDir(), "export")
	var exportOut, exportErr bytes.Buffer
	if code := RunExport(context.Background(), []string{"acme", exportDir}, memoryCmdDeps{Stdout: &exportOut, Stderr: &exportErr, Resolver: resolver, Stores: mgr}); code != 0 {
		t.Fatalf("RunExport: exit %d, stderr=%s", code, exportErr.String())
	}

	var importOut, importErr bytes.Buffer
	code := RunImport(context.Background(), []string{"acme-restored", exportDir}, memoryCmdDeps{Stdout: &importOut, Stderr: &importErr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("RunImport: exit %d, stderr=%s", code, importErr.String())
	}

	content, ok, err := s.ReadContent(ctx, "acme-restored", "", "memory")
	if err != nil || !ok {
		t.Fatalf("ReadContent: ok=%v err=%v", ok, err)
	}
	if strings.TrimSpace(content) != "original" {
		t.Errorf("content = %q, want %q", content, "original")
	}
}

// TestRunImport_RejectsInvalidProjectName checks that the CLI import
// refuses invalid project names, such as "--force" or "../escape", as a
// usage error (exit code 2) instead of creating the project.
func TestRunImport_RejectsInvalidProjectName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))
	inputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(inputDir, "memory.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"../escape", "--force", "a b"} {
		var stdout, stderr bytes.Buffer
		if code := RunImport(context.Background(), []string{name, inputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr}); code != usageExitCode {
			t.Errorf("import %q: exit code = %d, want %d", name, code, usageExitCode)
		}
	}
}
