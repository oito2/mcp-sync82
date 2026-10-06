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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/mcp-sync82/internal/config"
)

// TestInitProjectMemoryTool_ManualCreatesAllFourDocuments verifies that
// initializing a project from manual answers writes the memory, architecture,
// stack and next_steps documents with the given fields and does not seed the
// append-only kinds (decisions, progress).
func TestInitProjectMemoryTool_ManualCreatesAllFourDocuments(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(mustJSON(t, map[string]string{
		"project": "acme", "description": "does things", "goal": "be useful",
		"phase": "mvp", "next_steps": "ship v1",
	}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, `Project "acme" initialized`) {
		t.Fatalf("unexpected message: %s", result.Text)
	}
	for _, k := range []string{"memory", "architecture", "stack", "next_steps"} {
		if !strings.Contains(result.Text, k) {
			t.Errorf("expected %q listed as written, got: %s", k, result.Text)
		}
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	content, ok, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil || !ok {
		t.Fatalf("ReadDocument(memory): ok=%v err=%v", ok, err)
	}
	if !strings.Contains(content, "does things") || !strings.Contains(content, "be useful") {
		t.Fatalf("memory content missing expected fields: %s", content)
	}

	// The append-only kinds decisions and progress must not be seeded.
	exists, err := s.KindExists(ctx, "acme", "", "decisions")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected decisions to stay unseeded (append-only kinds start empty)")
	}
}

// TestInitProjectMemoryTool_Validate_RejectsMalformedProjectName verifies that
// validation rejects a malformed project or subproject name, as create_project
// and delete_project do, so a name like "../evil" never reaches the store,
// which performs no name check.
func TestInitProjectMemoryTool_Validate_RejectsMalformedProjectName(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "../evil"})); err == nil {
		t.Fatal("expected a validation error for a malformed project name")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "subproject": "bad name"})); err == nil {
		t.Fatal("expected a validation error for a malformed subproject name")
	}
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme-web_v2"})); err != nil {
		t.Fatalf("expected a well-formed project name to pass validation, got: %v", err)
	}
}

// TestInitProjectMemoryTool_DoesNotOverwriteAlreadyFilledFiles verifies that a
// document that already has content is not overwritten by initialization and
// is not listed as written.
func TestInitProjectMemoryTool_DoesNotOverwriteAlreadyFilledFiles(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EnsureProject(ctx, "acme", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(ctx, "acme", "", "memory", "hand-written content, do not clobber"); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "description": "new description"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// memory already has content, so it must not be listed as written.
	if strings.Contains(result.Text, "Files written: memory") {
		t.Fatalf("expected memory NOT to be overwritten, got: %s", result.Text)
	}

	content, ok, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil || !ok {
		t.Fatalf("ReadDocument: ok=%v err=%v", ok, err)
	}
	if content != "hand-written content, do not clobber" {
		t.Fatalf("memory content was clobbered: %s", content)
	}
}

// TestInitProjectMemoryTool_AutoDetectRequiresWorkspaceRoot verifies that
// validation rejects auto_detect without a workspace_root.
func TestInitProjectMemoryTool_AutoDetectRequiresWorkspaceRoot(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}

	if _, err := tool.Validate(mustJSON(t, map[string]any{"project": "acme", "auto_detect": true})); err == nil {
		t.Fatal("expected a validation error when auto_detect is true without workspace_root")
	}
}

// TestInitProjectMemoryTool_AutoDetectMergesWithExplicitArgsWinning verifies
// that auto-detected values (language from go.mod, description from README.md)
// fill the fields the caller left empty, while an explicit argument wins over
// the detected value.
func TestInitProjectMemoryTool_AutoDetectMergesWithExplicitArgsWinning(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/app\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("# App\nDetected description from README.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	// The explicit description must win over the detected one; languages has
	// no explicit value, so the detected "Go" must be used.
	parsed, err := tool.Validate(mustJSON(t, map[string]any{
		"project": "acme", "workspace_root": workspace, "auto_detect": true,
		"description": "explicit description wins",
	}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, "Auto-detected fields:") {
		t.Fatalf("expected an auto-detected fields summary, got: %s", result.Text)
	}

	s, err := mgr.Get(ctx, r.DefaultDBPath)
	if err != nil {
		t.Fatal(err)
	}
	memoryContent, _, err := s.ReadDocument(ctx, "acme", "", "memory")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(memoryContent, "explicit description wins") {
		t.Fatalf("expected explicit description to win, got: %s", memoryContent)
	}
	if strings.Contains(memoryContent, "Detected description from README") {
		t.Fatalf("detected description should have been overridden, got: %s", memoryContent)
	}

	stackContent, _, err := s.ReadDocument(ctx, "acme", "", "stack")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stackContent, "Go") {
		t.Fatalf("expected detected language Go in stack, got: %s", stackContent)
	}
}

// TestInitProjectMemoryTool_CreatesLocalConfig verifies that initializing with
// a workspace_root writes a .sync82.json with the project and subproject and
// mentions it in the result.
func TestInitProjectMemoryTool_CreatesLocalConfig(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "oito2", "subproject": "sync82", "workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, ".sync82.json") {
		t.Fatalf("expected the message to mention .sync82.json, got: %s", result.Text)
	}

	local, err := config.ReadLocalConfig(workspace, false)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if local == nil || local.Config.Project != "oito2" || local.Config.Subproject != "sync82" {
		t.Fatalf("unexpected local config: %+v", local)
	}
}

// TestInitProjectMemoryTool_UnreadableLocalConfigIsReported verifies that a
// .sync82.json that cannot be parsed yields an error result and is left
// byte-for-byte unchanged instead of being overwritten.
func TestInitProjectMemoryTool_UnreadableLocalConfigIsReported(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()
	configPath := filepath.Join(workspace, ".sync82.json")
	corrupt := []byte(`{"project": "oito2", "path": "/custom/vault.db"`)
	if err := os.WriteFile(configPath, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "oito2", "workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Text, "could not be read") {
		t.Fatalf("result = %+v, want an error result saying .sync82.json could not be read", result)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(corrupt) {
		t.Fatalf(".sync82.json = %q, want it unchanged (%q)", got, corrupt)
	}
}

// TestInitProjectMemoryTool_PreservesExistingCustomPathOnReinit verifies that
// re-running initialization with only a workspace_root keeps the custom vault
// path already recorded in .sync82.json and writes the documents to that
// vault.
func TestInitProjectMemoryTool_PreservesExistingCustomPathOnReinit(t *testing.T) {
	// WriteLocalConfig overwrites the whole file, so the tool itself must
	// carry the existing path forward when "path" is not passed again.
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()
	customVault := filepath.Join(t.TempDir(), "custom.db")

	if err := config.WriteLocalConfig(workspace, config.LocalConfig{
		Project: "acme", Path: customVault,
	}); err != nil {
		t.Fatalf("seed WriteLocalConfig: %v", err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	local, err := config.ReadLocalConfig(workspace, false)
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if local == nil || local.Config.Path != customVault {
		t.Fatalf("expected .sync82.json to still have path %q after reinit, got: %+v", customVault, local)
	}

	// The documents must be written to the custom vault, not only the path
	// preserved in the file.
	custom, err := mgr.Get(ctx, customVault)
	if err != nil {
		t.Fatalf("open custom vault: %v", err)
	}
	if exists, err := custom.KindExists(ctx, "acme", "", "memory"); err != nil || !exists {
		t.Fatalf("expected memory.md to exist in the custom vault: exists=%v err=%v", exists, err)
	}
}

// TestInitProjectMemoryTool_NoProjectResolvable verifies that, when no project
// can be resolved, the tool returns needsProjectMessage as a non-error result.
func TestInitProjectMemoryTool_NoProjectResolvable(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}

	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(context.Background(), parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Text != needsProjectMessage {
		t.Fatalf("expected needsProjectMessage, got: %s", result.Text)
	}
	if result.IsError {
		t.Fatal("needsProjectMessage must not be an error result")
	}
}

// TestInitProjectMemoryTool_PicksUpSubprojectFromLocalConfigEvenWithExplicitPr
// oject verifies that project and subproject are resolved independently: an
// explicit project without a subproject takes the subproject from
// .sync82.json, unlike the all-or-nothing tiers of Resolver.Resolve.
func TestInitProjectMemoryTool_PicksUpSubprojectFromLocalConfigEvenWithExplicitProject(t *testing.T) {
	// init_project_memory fills project and subproject independently,
	// unlike the all-or-nothing tiers of Resolver.Resolve.
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "oito2", Subproject: "sync82"}); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	// The explicit project matches the local config's project and the
	// subproject is omitted, so it must come from the local config.
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "oito2", "workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, `"oito2/sync82"`) {
		t.Fatalf("expected subproject picked up from local config, got: %s", result.Text)
	}
}

// TestInitProjectMemoryTool_GlobalConfigFallback verifies that, with no
// arguments, the project is taken from the last project in the global config.
func TestInitProjectMemoryTool_GlobalConfigFallback(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme"}); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(nil)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, `"acme"`) {
		t.Fatalf("expected project resolved from global config, got: %s", result.Text)
	}
}

// TestInitProjectMemoryTool_DifferentProjectLeavesLocalConfigAlone verifies
// that an explicit project different from the one in .sync82.json does not
// inherit that file's subproject and does not rewrite the file.
func TestInitProjectMemoryTool_DifferentProjectLeavesLocalConfigAlone(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()
	if err := config.WriteLocalConfig(workspace, config.LocalConfig{Project: "alpha", Subproject: "api"}); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "beta", "workspace_root": workspace}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result.Text, `Project "beta" initialized`) || !strings.Contains(result.Text, "left unchanged") {
		t.Fatalf("Text = %q, want beta initialized and .sync82.json left unchanged", result.Text)
	}

	local, err := config.ReadLocalConfig(workspace, false)
	if err != nil || local == nil {
		t.Fatalf("ReadLocalConfig: %+v, %v", local, err)
	}
	if local.Config.Project != "alpha" || local.Config.Subproject != "api" {
		t.Fatalf(".sync82.json = %+v, want alpha/api unchanged", local.Config)
	}
}

// TestInitProjectMemoryTool_RecordsPortableVaultPathAsGiven verifies that a
// "HOME/..." vault path is written to .sync82.json as given, not expanded to
// this machine's absolute home directory.
func TestInitProjectMemoryTool_RecordsPortableVaultPathAsGiven(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	workspace := t.TempDir()

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "workspace_root": workspace, "path": "HOME/vaults/team.db"}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if _, err := tool.Execute(ctx, parsed); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	local, err := config.ReadLocalConfig(workspace, false)
	if err != nil || local == nil {
		t.Fatalf("ReadLocalConfig: %+v, %v", local, err)
	}
	if local.Config.Path != "HOME/vaults/team.db" {
		t.Fatalf(".sync82.json path = %q, want %q", local.Config.Path, "HOME/vaults/team.db")
	}
}

// TestInitProjectMemoryTool_WorkspaceRootDoesNotUseLastProject verifies that a
// workspace_root without a .sync82.json never picks up the last project from
// the global config and yields needsProjectMessage.
func TestInitProjectMemoryTool_WorkspaceRootDoesNotUseLastProject(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	ctx := context.Background()
	if err := config.WriteGlobalConfig(config.GlobalConfig{LastProject: "acme"}); err != nil {
		t.Fatal(err)
	}

	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	parsed, err := tool.Validate(mustJSON(t, map[string]string{"workspace_root": t.TempDir()}))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	result, err := tool.Execute(ctx, parsed)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Text != needsProjectMessage {
		t.Fatalf("Text = %q, want needsProjectMessage", result.Text)
	}
}

// TestInitProjectMemoryTool_BoundsAnswers verifies that validation rejects an
// answer field larger than maxContentSize, as every other write path does.
func TestInitProjectMemoryTool_BoundsAnswers(t *testing.T) {
	r, mgr := newToolTestEnv(t)
	tool := &InitProjectMemoryTool{Resolver: r, Stores: mgr}
	big := strings.Repeat("x", maxContentSize+1)
	if _, err := tool.Validate(mustJSON(t, map[string]string{"project": "acme", "description": big})); err == nil {
		t.Fatal("expected an oversized description to be rejected")
	}
}
