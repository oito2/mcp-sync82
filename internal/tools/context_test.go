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

package tools

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// testResolver returns a Resolver whose default vault path is a placeholder
// that is never opened, with HOME isolated to a temp dir so the global config
// is never read from or written to the real home directory.
func testResolver(t *testing.T) *Resolver {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	return NewResolver("/default/vault.db", slog.New(slog.DiscardHandler))
}

// TestResolve_ExplicitProjectWins verifies that an explicit project and
// subproject are trimmed, reported with SourceProvided, and resolved against
// the default vault.
func TestResolve_ExplicitProjectWins(t *testing.T) {
	r := testResolver(t)

	ctx := r.Resolve(ContextArgs{Project: "  oito2  ", Subproject: "sync82"})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	if ctx.Project != "oito2" || ctx.Subproject != "sync82" {
		t.Fatalf("expected trimmed project/subproject, got %+v", ctx)
	}
	if ctx.Source != SourceProvided {
		t.Fatalf("Source = %q, want %q", ctx.Source, SourceProvided)
	}
	if want := absPath(r.DefaultDBPath); ctx.DBPath != want {
		t.Fatalf("DBPath = %q, want default %q", ctx.DBPath, want)
	}
}

// TestResolve_ExplicitProjectWithCustomPath verifies that an explicit "path"
// argument is expanded against the home directory to form the vault path.
func TestResolve_ExplicitProjectWithCustomPath(t *testing.T) {
	r := testResolver(t)
	home, _ := os.UserHomeDir()

	ctx := r.Resolve(ContextArgs{Project: "acme", Path: "HOME/custom-vault.db"})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	want := filepath.Join(home, "custom-vault.db")
	if ctx.DBPath != want {
		t.Fatalf("DBPath = %q, want %q", ctx.DBPath, want)
	}
}

// TestResolve_LocalConfigDiscovery verifies that, without an explicit project,
// Resolve reads project and subproject from the workspace root's .sync82.json
// and uses the default vault when the file sets no path.
func TestResolve_LocalConfigDiscovery(t *testing.T) {
	r := testResolver(t)

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	ctx := r.Resolve(ContextArgs{WorkspaceRoot: workspace})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	if ctx.Project != "oito2" || ctx.Subproject != "sync82" {
		t.Fatalf("got %+v", ctx)
	}
	if ctx.Source != SourceLocalConfig {
		t.Fatalf("Source = %q, want %q", ctx.Source, SourceLocalConfig)
	}
	if ctx.DBPath != absPath(r.DefaultDBPath) {
		t.Fatalf("expected default DBPath when local config has no path override, got %q", ctx.DBPath)
	}
}

// TestResolve_DoesNotSearchParentDirsByDefault verifies that Resolve ignores a
// .sync82.json located in an ancestor of the workspace root unless
// SearchParentDirs is set, because an ancestor file the caller does not
// control could redirect where memory is stored through its "path" field.
func TestResolve_DoesNotSearchParentDirsByDefault(t *testing.T) {
	r := testResolver(t)

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	nested := filepath.Join(workspace, "nested", "dir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	ctx := r.Resolve(ContextArgs{WorkspaceRoot: nested})
	if ctx.OK {
		t.Fatalf("expected OK=false — .sync82.json lives in an ancestor, and SearchParentDirs wasn't set, got %+v", ctx)
	}

	// With SearchParentDirs set, the same nested directory must find the
	// ancestor file, which shows the lookup itself works.
	ctx = r.Resolve(ContextArgs{WorkspaceRoot: nested, SearchParentDirs: true})
	if !ctx.OK {
		t.Fatal("expected OK=true with SearchParentDirs: true")
	}
	if ctx.Project != "oito2" || ctx.Subproject != "sync82" || ctx.Source != SourceLocalConfig {
		t.Fatalf("got %+v", ctx)
	}
}

// TestResolve_LocalConfigWithCustomVaultPath verifies that the "path" field of
// .sync82.json is expanded against the home directory and used as the vault
// path.
func TestResolve_LocalConfigWithCustomVaultPath(t *testing.T) {
	r := testResolver(t)
	home, _ := os.UserHomeDir()

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "acme", Path: "HOME/team-vault.db"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	ctx := r.Resolve(ContextArgs{WorkspaceRoot: workspace})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	want := filepath.Join(home, "team-vault.db")
	if ctx.DBPath != want {
		t.Fatalf("DBPath = %q, want %q", ctx.DBPath, want)
	}
}

// TestResolve_ExplicitPathWinsOverLocalConfigPath verifies that an explicit
// "path" argument overrides the path recorded in .sync82.json when the project
// comes from the local config, since every tool documents "path" as an
// unconditional override (PathDescription).
func TestResolve_ExplicitPathWinsOverLocalConfigPath(t *testing.T) {
	r := testResolver(t)

	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "acme", Path: "HOME/team-vault.db"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}
	explicitPath := filepath.Join(t.TempDir(), "explicit-override.db")

	ctx := r.Resolve(ContextArgs{WorkspaceRoot: workspace, Path: explicitPath})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	want := config.ResolvePath(explicitPath)
	if ctx.DBPath != want {
		t.Fatalf("DBPath = %q, want explicit override %q", ctx.DBPath, want)
	}
}

// TestResolve_ExplicitPathWinsOverGlobalConfigVaultPath verifies that an
// explicit "path" argument overrides the vaultPath of the global config when
// the project comes from the last session.
func TestResolve_ExplicitPathWinsOverGlobalConfigVaultPath(t *testing.T) {
	r := testResolver(t)

	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) {
		*c = config.GlobalConfig{LastProject: "oito2", VaultPath: "HOME/team-vault.db"}
	}); err != nil {
		t.Fatalf("UpdateGlobalConfig: %v", err)
	}
	explicitPath := filepath.Join(t.TempDir(), "explicit-override.db")

	ctx := r.Resolve(ContextArgs{Path: explicitPath})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	want := config.ResolvePath(explicitPath)
	if ctx.DBPath != want {
		t.Fatalf("DBPath = %q, want explicit override %q", ctx.DBPath, want)
	}
}

// TestResolve_GlobalConfigFallback verifies that, with no explicit project and
// no workspace root, Resolve falls back to the last project and subproject
// stored in the global config.
func TestResolve_GlobalConfigFallback(t *testing.T) {
	r := testResolver(t)

	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "oito2", LastSubproject: "perci"} }); err != nil {
		t.Fatalf("UpdateGlobalConfig: %v", err)
	}

	ctx := r.Resolve(ContextArgs{})
	if !ctx.OK {
		t.Fatal("expected OK=true")
	}
	if ctx.Project != "oito2" || ctx.Subproject != "perci" {
		t.Fatalf("got %+v", ctx)
	}
	if ctx.Source != SourceGlobalConfig {
		t.Fatalf("Source = %q, want %q", ctx.Source, SourceGlobalConfig)
	}
}

// TestResolve_NoneAvailable verifies that Resolve reports OK=false when no
// project, workspace root or global config is available.
func TestResolve_NoneAvailable(t *testing.T) {
	r := testResolver(t)

	ctx := r.Resolve(ContextArgs{})
	if ctx.OK {
		t.Fatalf("expected OK=false with no project/workspace_root/global config, got %+v", ctx)
	}
}

// TestResolve_TierPrecedence verifies the resolution order: an explicit
// project (tier 1) wins over the local config (tier 2), which wins over the
// global config (tier 3).
func TestResolve_TierPrecedence(t *testing.T) {
	// Tier 1 must win even when tiers 2 and 3 would also resolve.
	r := testResolver(t)

	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "from-global"} }); err != nil {
		t.Fatalf("UpdateGlobalConfig: %v", err)
	}
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "from-local"}); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	ctx := r.Resolve(ContextArgs{Project: "explicit", WorkspaceRoot: workspace})
	if ctx.Project != "explicit" || ctx.Source != SourceProvided {
		t.Fatalf("expected tier 1 to win, got %+v", ctx)
	}

	// Without an explicit project, tier 2 (local config) must win over
	// tier 3 (global config).
	ctx = r.Resolve(ContextArgs{WorkspaceRoot: workspace})
	if ctx.Project != "from-local" || ctx.Source != SourceLocalConfig {
		t.Fatalf("expected tier 2 to win over tier 3, got %+v", ctx)
	}
}

// openTestStore opens a store at path and, when project is not empty, ensures
// the project and subproject exist. The store is closed on test cleanup. It
// calls t.Fatal if opening the store or creating the project fails.
func openTestStore(t *testing.T, path, project, subproject string) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if project != "" {
		if _, _, err := s.EnsureProject(context.Background(), project, subproject); err != nil {
			t.Fatalf("EnsureProject: %v", err)
		}
	}
	return s
}

// TestRememberIfExists_RemembersExistingProjectWithItsVault verifies that
// RememberIfExists stores an existing project, its subproject and its vault
// path in the global config, and that a later argument-less Resolve returns
// them.
func TestRememberIfExists_RemembersExistingProjectWithItsVault(t *testing.T) {
	r := testResolver(t)
	vault := filepath.Join(t.TempDir(), "custom.db")
	s := openTestStore(t, vault, "oito2", "sync82")

	rctx := r.Resolve(ContextArgs{Project: "oito2", Subproject: "sync82", Path: vault})
	r.RememberIfExists(context.Background(), s, rctx)

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatalf("ReadGlobalConfig: %v", err)
	}
	if cfg.LastProject != "oito2" || cfg.LastSubproject != "sync82" || cfg.LastVaultPath != vault {
		t.Fatalf("expected oito2/sync82 in %s to be remembered, got %+v", vault, cfg)
	}

	// A later call with no arguments resolves through tier 3 to the same
	// project in the same vault, not the default one.
	ctx2 := r.Resolve(ContextArgs{})
	if !ctx2.OK || ctx2.Project != "oito2" || ctx2.Subproject != "sync82" || ctx2.DBPath != vault {
		t.Fatalf("expected tier 3 to resolve oito2/sync82 in %s, got %+v", vault, ctx2)
	}
}

// TestRememberIfExists_IgnoresMissingProject verifies that a call naming a
// project that does not exist (a typo) does not replace the remembered last
// project.
func TestRememberIfExists_IgnoresMissingProject(t *testing.T) {
	r := testResolver(t)
	vault := filepath.Join(t.TempDir(), "vault.db")
	s := openTestStore(t, vault, "acme", "")
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "acme", LastVaultPath: vault} }); err != nil {
		t.Fatal(err)
	}

	r.RememberIfExists(context.Background(), s, r.Resolve(ContextArgs{Project: "acmee", Path: vault}))

	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LastProject != "acme" {
		t.Fatalf("LastProject = %q, want the previous %q to survive a typo", cfg.LastProject, "acme")
	}
}

// TestResolve_GlobalVaultPathAppliesToEveryTier verifies that the vault path
// set in the global config (as by "sync82 config set-vault") is used by
// explicit-project and local-config resolution and by DBPathOrDefault, not
// only by the global-config fallback, so those calls do not write to the
// default vault.
func TestResolve_GlobalVaultPathAppliesToEveryTier(t *testing.T) {
	r := testResolver(t)
	home, _ := os.UserHomeDir()
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{VaultPath: "HOME/team-vault.db"} }); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "team-vault.db")

	if got := r.Resolve(ContextArgs{Project: "acme"}).DBPath; got != want {
		t.Errorf("tier 1 DBPath = %q, want %q", got, want)
	}
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "acme"}); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(ContextArgs{WorkspaceRoot: workspace}).DBPath; got != want {
		t.Errorf("tier 2 DBPath = %q, want %q", got, want)
	}
	if got := r.DBPathOrDefault(""); got != want {
		t.Errorf("DBPathOrDefault(\"\") = %q, want %q", got, want)
	}
}

// TestResolve_WorkspaceRootWithoutUsableConfigDoesNotFallBack verifies that a
// workspace root whose .sync82.json is missing, empty or malformed yields
// OK=false instead of falling back to the last project of another session, and
// that NeedsInput() reports the .sync82.json problem.
func TestResolve_WorkspaceRootWithoutUsableConfigDoesNotFallBack(t *testing.T) {
	r := testResolver(t)
	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "projectA"} }); err != nil {
		t.Fatal(err)
	}

	missing := t.TempDir()
	if ctx := r.Resolve(ContextArgs{WorkspaceRoot: missing}); ctx.OK {
		t.Errorf("missing .sync82.json: expected OK=false, got %+v", ctx)
	}

	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, ".sync82.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ctx := r.Resolve(ContextArgs{WorkspaceRoot: empty}); ctx.OK {
		t.Errorf("empty .sync82.json: expected OK=false, got %+v", ctx)
	}

	malformed := t.TempDir()
	if err := os.WriteFile(filepath.Join(malformed, ".sync82.json"), []byte("<<<<<<< HEAD"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := r.Resolve(ContextArgs{WorkspaceRoot: malformed})
	if ctx.OK {
		t.Fatalf("malformed .sync82.json: expected OK=false, got %+v", ctx)
	}
	if !strings.Contains(ctx.NeedsInput(), ".sync82.json") || !strings.Contains(ctx.NeedsInput(), NeedsInputMessage) {
		t.Errorf("NeedsInput() = %q, want the .sync82.json problem followed by NeedsInputMessage", ctx.NeedsInput())
	}
}

// TestResolve_ExplicitSubprojectKeptInTiers2And3 verifies that an explicit
// subproject argument is kept when the project comes from .sync82.json or from
// the global config.
func TestResolve_ExplicitSubprojectKeptInTiers2And3(t *testing.T) {
	r := testResolver(t)
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "acme"}); err != nil {
		t.Fatal(err)
	}
	if ctx := r.Resolve(ContextArgs{WorkspaceRoot: workspace, Subproject: "api"}); ctx.Project != "acme" || ctx.Subproject != "api" {
		t.Errorf("tier 2: got %+v, want acme/api", ctx)
	}

	if err := config.UpdateGlobalConfig(func(c *config.GlobalConfig) { *c = config.GlobalConfig{LastProject: "acme", LastSubproject: "web"} }); err != nil {
		t.Fatal(err)
	}
	if ctx := r.Resolve(ContextArgs{Subproject: "api"}); ctx.Project != "acme" || ctx.Subproject != "api" {
		t.Errorf("tier 3: got %+v, want acme/api", ctx)
	}
}

// TestResolve_RelativeLocalVaultPathResolvesAgainstConfigRoot verifies that a
// relative "path" in .sync82.json is resolved against the directory holding
// that file, not the process working directory.
func TestResolve_RelativeLocalVaultPathResolvesAgainstConfigRoot(t *testing.T) {
	r := testResolver(t)
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "acme", Path: "vault.db"}); err != nil {
		t.Fatal(err)
	}
	if got, want := r.Resolve(ContextArgs{WorkspaceRoot: workspace}).DBPath, filepath.Join(workspace, "vault.db"); got != want {
		t.Fatalf("DBPath = %q, want %q", got, want)
	}
}

// TestContextNote verifies the note ContextNote appends to a tool result:
// empty for an explicit project, and a bracketed project, source and vault
// description for local-config and global-config resolution.
func TestContextNote(t *testing.T) {
	tests := []struct {
		name string
		ctx  ResolvedContext
		want string
	}{
		{"provided", ResolvedContext{Project: "acme", Source: SourceProvided}, ""},
		{"local config, top-level", ResolvedContext{Project: "acme", DBPath: "/vaults/acme.db", Source: SourceLocalConfig}, " [project: acme, from .sync82.json, vault: /vaults/acme.db]"},
		{"local config, subproject", ResolvedContext{Project: "oito2", Subproject: "sync82", DBPath: "/vaults/oito2.db", Source: SourceLocalConfig}, " [project: oito2/sync82, from .sync82.json, vault: /vaults/oito2.db]"},
		{"global config", ResolvedContext{Project: "acme", DBPath: "/vaults/acme.db", Source: SourceGlobalConfig}, " [project: acme, from last session, vault: /vaults/acme.db]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContextNote(tt.ctx); got != tt.want {
				t.Errorf("ContextNote(%+v) = %q, want %q", tt.ctx, got, tt.want)
			}
		})
	}
}

// TestResolve_LastProjectOnlyInItsOwnVault verifies that the last-used
// project is reused with an explicit path only when that path is the vault
// it was remembered in.
func TestResolve_LastProjectOnlyInItsOwnVault(t *testing.T) {
	r := testResolver(t)
	remembered := filepath.Join(t.TempDir(), "a.db")
	if err := config.UpdateLastProject("acme", "", remembered); err != nil {
		t.Fatal(err)
	}

	same := r.Resolve(ContextArgs{Path: remembered})
	if !same.OK || same.Project != "acme" || same.DBPath != remembered {
		t.Errorf("same vault: %+v, want acme in %s", same, remembered)
	}

	other := filepath.Join(t.TempDir(), "b.db")
	got := r.Resolve(ContextArgs{Path: other})
	if got.OK || !strings.Contains(got.Problem, "another vault") {
		t.Errorf("other vault: %+v, want an unresolved context naming the other vault", got)
	}

	project, _, dbPath, _, err := r.resolveInitTarget(ContextArgs{Path: other})
	if err != nil || project != "" || dbPath != other {
		t.Errorf("resolveInitTarget(other vault) = %q, %q, %v; want no project in %s", project, dbPath, err, other)
	}
	project, _, dbPath, _, _ = r.resolveInitTarget(ContextArgs{Path: remembered})
	if project != "acme" || dbPath != remembered {
		t.Errorf("resolveInitTarget(same vault) = %q, %q; want acme in %s", project, dbPath, remembered)
	}
}

// TestResolve_BlankWorkspaceRootIsRefused checks that a workspace_root of
// only spaces is refused instead of being read as absent, which would fall
// back to the last-used project, in Resolve, search_memory and
// init_project_memory.
func TestResolve_BlankWorkspaceRootIsRefused(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	if err := config.UpdateLastProject("other", "", r.DefaultDBPath); err != nil {
		t.Fatal(err)
	}
	if got := r.Resolve(ContextArgs{WorkspaceRoot: "   "}); got.OK || !strings.Contains(got.Problem, "is blank") {
		t.Errorf("Resolve = %+v, want a blank workspace_root refused", got)
	}
	res := runTool(t, &SearchMemoryTool{Resolver: r, Stores: mgr}, map[string]any{"query": "x", "workspace_root": "   "})
	if !res.IsError || !strings.Contains(res.Text, "is blank") {
		t.Errorf("search_memory = %+v, want an error naming the blank workspace_root", res)
	}
	if _, err := (&InitProjectMemoryTool{Resolver: r, Stores: mgr}).Validate(mustJSON(t, map[string]any{"workspace_root": "  "})); err == nil {
		t.Error("init_project_memory accepted a blank workspace_root")
	}
}
