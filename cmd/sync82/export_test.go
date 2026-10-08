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

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// TestRunExport_TwoArgs_ProjectOnly checks that exporting a project writes its
// documents as .md files in the output directory.
func TestRunExport_TwoArgs_ProjectOnly(t *testing.T) {
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
	if err := s.WriteDocument(ctx, "acme", "", "memory", "hello"); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"acme", outputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Exported 1 file") {
		t.Errorf("stdout = %q, want it to report 1 exported file", stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "hello" {
		t.Errorf("memory.md = %q, want %q", data, "hello")
	}
}

// TestRunExport_ThreeArgs_WithSubproject checks that exporting a subproject
// reports it as project/subproject.
func TestRunExport_ThreeArgs_WithSubproject(t *testing.T) {
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
	if _, _, err := s.EnsureProject(ctx, "acme", "plugin"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "plugin", "memory", "sub content"); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"acme", "plugin", outputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "acme/plugin") {
		t.Errorf("stdout = %q, want it to mention acme/plugin", stdout.String())
	}
}

// TestRunExport_WrongArgCount checks that a wrong number of positional
// arguments is a usage error: exit code 2 and a usage message.
func TestRunExport_WrongArgCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"only-one-arg"}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(stderr.String(), "usage:") || !strings.Contains(stderr.String(), "--help' for usage.") {
		t.Errorf("stderr = %q, want a usage message", stderr.String())
	}
}

// TestRunExport_ProjectNotFound checks that exporting an unknown project exits
// with code 1.
func TestRunExport_ProjectNotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"ghost", t.TempDir()}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// TestRunExport_All_ExportsEveryProjectAndSubproject checks that --all exports
// every project and subproject into its own subfolder and prints a total.
func TestRunExport_All_ExportsEveryProjectAndSubproject(t *testing.T) {
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
	if err := s.WriteDocument(ctx, "acme", "", "memory", "acme content"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "oito2", "sync82"); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "oito2", "sync82", "memory", "sub content"); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"--all", outputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}

	acmeData, err := os.ReadFile(filepath.Join(outputDir, "acme", "memory.md"))
	if err != nil {
		t.Fatalf("acme/memory.md: %v", err)
	}
	if strings.TrimSpace(string(acmeData)) != "acme content" {
		t.Errorf("acme/memory.md = %q, want %q", acmeData, "acme content")
	}

	subData, err := os.ReadFile(filepath.Join(outputDir, "oito2", "sync82", "memory.md"))
	if err != nil {
		t.Fatalf("oito2/sync82/memory.md: %v", err)
	}
	if strings.TrimSpace(string(subData)) != "sub content" {
		t.Errorf("oito2/sync82/memory.md = %q, want %q", subData, "sub content")
	}

	// 3, not 2: "acme" and "oito2/sync82" both have content, but the
	// top-level "oito2" (the subproject's parent) is a project entry too —
	// it just has zero files of its own, same as list_projects would show
	// it.
	if !strings.Contains(stdout.String(), "Exported 3 project(s), 2 file(s) total") {
		t.Errorf("stdout = %q, want a summary mentioning 3 projects and 2 files", stdout.String())
	}
}

// TestRunExport_All_EmptyVault checks that --all on a vault without projects
// succeeds and reports that there is nothing to export.
func TestRunExport_All_EmptyVault(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))
	if _, err := mgr.Get(context.Background(), dbPath); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"--all", t.TempDir()}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Nothing to export") {
		t.Errorf("stdout = %q, want a \"nothing to export\" message", stdout.String())
	}
}

// TestRunExport_All_WrongArgCount checks that --all without exactly one output
// directory is a usage error: exit code 2 and a usage message.
func TestRunExport_All_WrongArgCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"--all"}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(stderr.String(), "usage:") || !strings.Contains(stderr.String(), "--help' for usage.") {
		t.Errorf("stderr = %q, want a usage message", stderr.String())
	}
}

// TestRunExport_CustomVaultPath checks that --path exports from the given vault
// instead of the resolver's default one.
func TestRunExport_CustomVaultPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	customVault := filepath.Join(t.TempDir(), "custom.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(filepath.Join(t.TempDir(), "default.db"), slog.New(slog.DiscardHandler))

	ctx := context.Background()
	s, err := mgr.Get(ctx, customVault)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "custom vault content"); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(t.TempDir(), "out")
	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"acme", outputDir, "--path", customVault}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(outputDir, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "custom vault content" {
		t.Errorf("memory.md = %q, want %q", data, "custom vault content")
	}
}

// TestRunExport_All_RefusesTraversalName checks that a project named
// "../escape" (written directly at the store level, bypassing name
// validation) makes "export --all" fail instead of writing outside the
// output directory.
func TestRunExport_All_RefusesTraversalName(t *testing.T) {
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
	if _, _, err := s.EnsureProject(ctx, "../escape", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "../escape", "", "memory", "planted"); err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	outputDir := filepath.Join(base, "backup")
	var stdout, stderr bytes.Buffer
	code := RunExport(context.Background(), []string{"--all", outputDir}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(base, "escape")); !os.IsNotExist(err) {
		t.Fatalf("export wrote outside the output directory (err=%v)", err)
	}
}

// TestRunExport_MissingVaultFails checks that exporting from a path where no
// vault exists fails instead of creating an empty vault and reporting
// "nothing to export", which would look like a successful backup.
func TestRunExport_MissingVaultFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vault.db")
	mgr := store.NewManager()
	t.Cleanup(func() { mgr.Close() })
	resolver := tools.NewResolver(dbPath, slog.New(slog.DiscardHandler))

	var stdout, stderr bytes.Buffer
	if code := RunExport(context.Background(), []string{"--all", t.TempDir()}, memoryCmdDeps{Stdout: &stdout, Stderr: &stderr, Resolver: resolver, Stores: mgr}); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("a vault file was created at %s (err=%v)", dbPath, err)
	}
}
