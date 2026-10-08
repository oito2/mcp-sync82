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
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/oito2/mcp-sync82/internal/config"
	"github.com/oito2/mcp-sync82/internal/store"
)

// ContextArgs holds the subset of a tool call's arguments relevant to
// context resolution. Every field is the empty string when the calling
// agent omitted it — exactly how a parsed JSON-RPC request would leave an
// absent optional field.
type ContextArgs struct {
	Project          string
	Subproject       string
	Path             string
	WorkspaceRoot    string
	SearchParentDirs bool
}

// targetArgs holds the project-resolution arguments of the project-scoped
// tools. Embedded in a tool's args struct, its fields are decoded from the
// top level of the call's arguments, like the tool's own fields.
type targetArgs struct {
	Project          string `json:"project,omitempty"`
	Subproject       string `json:"subproject,omitempty"`
	Path             string `json:"path,omitempty"`
	WorkspaceRoot    string `json:"workspace_root,omitempty"`
	SearchParentDirs bool   `json:"search_parent_dirs,omitempty"`
}

// contextArgs returns a's fields as the ContextArgs the Resolver takes.
func (a targetArgs) contextArgs() ContextArgs {
	return ContextArgs(a)
}

// ContextSource records which resolution tier produced a ResolvedContext,
// so ContextNote can explain it back to the calling agent.
type ContextSource string

// Values of ContextSource: an explicit project argument, a .sync82.json
// file, and the global config's last-used project.
const (
	SourceProvided     ContextSource = "provided"
	SourceLocalConfig  ContextSource = "local_config"
	SourceGlobalConfig ContextSource = "global_config"
)

// ResolvedContext is the outcome of resolving a tool call's target
// project. OK is false when no resolution tier produced an answer —
// callers must return NeedsInput() as a normal (non-error) tool result in
// that case, not fail the call. Problem, when set, explains why a given
// workspace_root could not be used, or — with InvalidName — why the
// resolved project or subproject name was rejected.
type ResolvedContext struct {
	OK          bool
	Project     string
	Subproject  string
	DBPath      string
	Source      ContextSource
	Problem     string
	InvalidName bool
}

// NeedsInputMessage is returned as a normal tool result (not an error
// result) when no resolution tier finds a project. It is instructional
// text for the calling agent.
const NeedsInputMessage = `Could not determine the active project or vault path automatically.

Please provide one of the following:
- "workspace_root": path to your project folder — the tool will read .sync82.json if present
- "project": explicit project name (optionally with "path" for a custom vault location)

Tip: run init_project_memory with workspace_root to create .sync82.json and enable automatic discovery in future sessions.`

// NeedsInput returns the tool result text for an unresolved context:
// NeedsInputMessage, preceded by Problem when one is set.
func (c ResolvedContext) NeedsInput() string {
	if c.Problem == "" {
		return NeedsInputMessage
	}
	return c.Problem + "\n\n" + NeedsInputMessage
}

// Resolver implements the three-tier context resolution every tool relies
// on: an explicit project argument wins outright; otherwise, when
// workspace_root is given, the .sync82.json there (or, opt-in via
// SearchParentDirs, discovered by walking up from it) and nothing else;
// otherwise the global config's last-used project. DefaultDBPath is the
// vault used when nothing else names one, and Logger receives diagnostics.
// SkipRemember, when set, keeps RememberIfExists from recording the
// last-used project, for callers such as the read-only CLI commands that
// must leave the global config as it is.
type Resolver struct {
	DefaultDBPath string
	Logger        *slog.Logger
	SkipRemember  bool
}

// NewResolver returns a Resolver that falls back to defaultDBPath as the
// vault and logs to logger. logger must not be nil; pass
// slog.New(slog.DiscardHandler) to silence logging.
func NewResolver(defaultDBPath string, logger *slog.Logger) *Resolver {
	return &Resolver{DefaultDBPath: defaultDBPath, Logger: logger}
}

// Resolve runs the three-tier resolution for a single tool call and
// returns the resolved context, with the project and subproject names
// validated and normalized. It never returns an error: a failure to
// resolve is represented by OK: false, with Problem set when the cause is
// an unusable workspace_root, an invalid name (InvalidName) or a last-used
// project remembered in another vault than the explicit path.
//
// An explicit subproject argument is kept in every tier. When
// workspace_root is given, tier 3 is never consulted: a missing or invalid
// .sync82.json there yields OK: false instead of silently falling back to
// whatever project another session used last. With an explicit path, tier
// 3 applies only when the last-used project was remembered in that same
// vault, or with no vault at all.
func (r *Resolver) Resolve(args ContextArgs) ResolvedContext {
	rctx := r.resolve(args)
	if rctx.OK {
		if err := ValidateTarget(rctx.Project, rctx.Subproject); err != nil {
			return ResolvedContext{Problem: err.Error(), InvalidName: true}
		}
		rctx.Project, rctx.Subproject = NormalizeName(rctx.Project), NormalizeName(rctx.Subproject)
	}
	return rctx
}

// resolve runs the resolution tiers for Resolve, without validating or
// normalizing the resulting names.
func (r *Resolver) resolve(args ContextArgs) ResolvedContext {
	// A workspace_root given but blank names no workspace: it is refused
	// rather than read as absent, which would fall back to the last-used
	// project.
	if args.WorkspaceRoot != "" && strings.TrimSpace(args.WorkspaceRoot) == "" {
		return ResolvedContext{Problem: `"workspace_root" is blank: pass the path of your project folder, or leave it out.`}
	}
	project := strings.TrimSpace(args.Project)
	subproject := strings.TrimSpace(args.Subproject)

	// Tier 1: explicit project argument wins outright.
	if project != "" {
		return ResolvedContext{
			OK:         true,
			Project:    project,
			Subproject: subproject,
			DBPath:     r.vaultPath(args.Path, "", ""),
			Source:     SourceProvided,
		}
	}

	// Tier 2: .sync82.json at workspace_root — or, when the caller opts in
	// via SearchParentDirs, discovered by walking up from it. Walking up
	// is opt-in (not the default) because a .sync82.json in an ancestor
	// directory the caller doesn't control could otherwise silently
	// redirect where memory is stored.
	if ws := strings.TrimSpace(args.WorkspaceRoot); ws != "" {
		result, err := config.ReadLocalConfig(ws, args.SearchParentDirs)
		if err != nil {
			r.Logger.Error("read local config", "workspace_root", ws, "error", err)
			return ResolvedContext{Problem: fmt.Sprintf("The .sync82.json for workspace_root %s could not be read: %v", ws, err)}
		}
		if result == nil || strings.TrimSpace(result.Config.Project) == "" {
			return ResolvedContext{}
		}
		r.Logger.Info("auto-discovered project from .sync82.json", "project", result.Config.Project)
		return ResolvedContext{
			OK:         true,
			Project:    strings.TrimSpace(result.Config.Project),
			Subproject: firstNonEmpty(subproject, strings.TrimSpace(result.Config.Subproject)),
			DBPath:     r.vaultPath(args.Path, result.Config.Path, result.ConfigRoot),
			Source:     SourceLocalConfig,
		}
	}

	// Tier 3: global config's last-used project, in the vault it was
	// last used in. An explicit path naming another vault doesn't reuse
	// it: the remembered name says nothing about that vault's projects.
	globalCfg, err := config.ReadGlobalConfig()
	if err != nil {
		r.Logger.Error("read global config", "error", err)
	} else if globalCfg.LastProject != "" {
		dbPath := r.vaultPath(args.Path, "", "")
		if globalCfg.LastVaultPath != "" {
			lastVault := absPath(config.ResolvePath(globalCfg.LastVaultPath))
			if args.Path == "" {
				dbPath = lastVault
			} else if !samePath(dbPath, lastVault) {
				return ResolvedContext{Problem: fmt.Sprintf("The last used project, %q, was used in another vault (%s), not in %s.",
					FormatLabel(globalCfg.LastProject, globalCfg.LastSubproject), lastVault, dbPath)}
			}
		}
		r.Logger.Info("using last known project from global config", "project", globalCfg.LastProject)
		return ResolvedContext{
			OK:         true,
			Project:    globalCfg.LastProject,
			Subproject: firstNonEmpty(subproject, globalCfg.LastSubproject),
			DBPath:     dbPath,
			Source:     SourceGlobalConfig,
		}
	}

	// Nothing resolved: the caller returns NeedsInput().
	return ResolvedContext{}
}

// resolveSearchScope resolves the project scope and vault of a
// search_memory call. Unlike Resolve, it never falls back to the global
// config's last-used project: with neither project nor workspace_root,
// the scope stays empty (search everything) rather than narrowing to the
// project last used, and a workspace_root without a .sync82.json searches
// the whole vault — unless a subproject was given, which then has no
// project to belong to. The returned context always carries the vault
// path; OK is true only when a project scope was resolved, and Problem is
// set when the workspace's .sync82.json cannot be read or names an invalid
// project, or a subproject was given without any project.
func (r *Resolver) resolveSearchScope(args ContextArgs) ResolvedContext {
	if strings.TrimSpace(args.Project) == "" && args.WorkspaceRoot != "" {
		rctx := r.Resolve(args)
		if !rctx.OK {
			if rctx.Problem == "" && strings.TrimSpace(args.Subproject) != "" {
				// Without a project the subproject can't narrow anything;
				// searching the whole vault instead would mix projects.
				rctx.Problem = fmt.Sprintf(`"subproject" was given, but no .sync82.json was found at workspace_root %s to tell which project it belongs to; pass "project" too`, args.WorkspaceRoot)
			}
			rctx.DBPath = r.DBPathOrDefault(args.Path)
		}
		return rctx
	}
	rctx := ResolvedContext{
		Project:    NormalizeName(args.Project),
		Subproject: NormalizeName(args.Subproject),
		DBPath:     r.DBPathOrDefault(args.Path),
		Source:     SourceProvided,
	}
	rctx.OK = rctx.Project != ""
	return rctx
}

// resolveInitTarget determines the project, subproject and vault path
// init_project_memory works on, from args. Unlike Resolver.Resolve, project
// and subproject are filled in independently from args → local config →
// global config, rather than all coming from a single winning tier. For
// example, an explicit "project" argument with no "subproject" still picks
// up a subproject from .sync82.json if one is found.
//
// A .sync82.json naming a different project than an explicit "project"
// argument contributes neither its subproject nor its vault path. When
// workspace_root is given, the global config's last-used project is never
// consulted, and with an explicit path only when it was remembered in that
// same vault, or with no vault at all. local is the .sync82.json found under workspace_root, if any;
// project is empty when nothing determined one. err is set when a
// .sync82.json under workspace_root exists but cannot be read or parsed.
func (r *Resolver) resolveInitTarget(args ContextArgs) (project, subproject, dbPath string, local *config.LocalConfigResult, err error) {
	project = NormalizeName(args.Project)
	subproject = NormalizeName(args.Subproject)

	if args.WorkspaceRoot != "" {
		localPath, localRoot := "", ""
		found, err := config.ReadLocalConfig(args.WorkspaceRoot, args.SearchParentDirs)
		if err != nil {
			return "", "", "", nil, err
		}
		if found != nil && strings.TrimSpace(found.Config.Project) != "" {
			local = found
			localProject := NormalizeName(found.Config.Project)
			if project == "" || project == localProject {
				project = localProject
				if subproject == "" {
					subproject = NormalizeName(found.Config.Subproject)
				}
				localPath, localRoot = found.Config.Path, found.ConfigRoot
			}
		}
		return project, subproject, r.vaultPath(args.Path, localPath, localRoot), local, nil
	}

	dbPath = r.vaultPath(args.Path, "", "")
	if project == "" {
		if globalCfg, err := config.ReadGlobalConfig(); err == nil && globalCfg.LastProject != "" {
			lastVault := ""
			if globalCfg.LastVaultPath != "" {
				lastVault = absPath(config.ResolvePath(globalCfg.LastVaultPath))
			}
			// As in Resolve, an explicit path naming another vault
			// doesn't reuse the last project.
			if args.Path != "" && lastVault != "" && !samePath(dbPath, lastVault) {
				return "", subproject, dbPath, nil, nil
			}
			project = globalCfg.LastProject
			if subproject == "" {
				subproject = globalCfg.LastSubproject
			}
			if args.Path == "" && lastVault != "" {
				dbPath = lastVault
			}
		}
	}
	return project, subproject, dbPath, nil, nil
}

// RememberIfExists records rctx as the last-used project, together with
// its vault path, when that project exists in s, so a call naming a
// project that doesn't exist (a typo) never becomes the default for later
// calls. A context resolved from tier 3 is already the remembered one and
// is left alone, and nothing is recorded when r.SkipRemember is set.
// Failures are logged and never returned.
func (r *Resolver) RememberIfExists(ctx context.Context, s *store.Store, rctx ResolvedContext) {
	if r.SkipRemember || !rctx.OK || rctx.Source == SourceGlobalConfig {
		return
	}
	exists, err := s.ProjectExists(ctx, rctx.Project, rctx.Subproject)
	if err != nil {
		r.Logger.Error("check project before remembering it", "project", rctx.Label(), "error", err)
		return
	}
	if !exists {
		return
	}
	if err := config.UpdateLastProject(rctx.Project, rctx.Subproject, rctx.DBPath); err != nil {
		r.Logger.Error("update last project", "project", rctx.Label(), "error", err)
	}
}

// ResolveStore resolves the call's context, opens the resulting vault and
// remembers the project as last used when it exists there — the "resolve
// context, then open the store" sequence every project-scoped memory tool
// runs first. args are the call's context arguments.
//
// Exactly one of three outcomes occurs:
//   - ready != nil: resolution failed (no project could be determined, or
//     its name is invalid). The caller must return *ready as-is
//     (NeedsInput() or an error result) and do nothing else.
//   - err != nil: resolution succeeded but opening the store failed. The
//     caller must return err from Execute.
//   - s != nil, ready == nil, err == nil: success — s is the resolved
//     project's store and rctx its resolved context.
//
// The vault must already exist: a path naming no vault yields an error
// result (vaultMissingResult) instead of a new, empty vault file — use
// ResolveStoreCreating for the tools that create projects.
func (r *Resolver) ResolveStore(ctx context.Context, stores *store.Manager, args ContextArgs) (s *store.Store, rctx ResolvedContext, ready *ToolResult, err error) {
	return r.resolveStore(ctx, stores, args, false)
}

// ResolveStoreCreating is ResolveStore for tools that create the resolved
// project: a missing vault is created instead of reported.
func (r *Resolver) ResolveStoreCreating(ctx context.Context, stores *store.Manager, args ContextArgs) (s *store.Store, rctx ResolvedContext, ready *ToolResult, err error) {
	return r.resolveStore(ctx, stores, args, true)
}

// resolveStore implements ResolveStore and ResolveStoreCreating; create
// selects whether a missing vault is created (true) or reported as an
// error result (false).
func (r *Resolver) resolveStore(ctx context.Context, stores *store.Manager, args ContextArgs, create bool) (s *store.Store, rctx ResolvedContext, ready *ToolResult, err error) {
	rctx = r.Resolve(args)
	if rctx.InvalidName {
		return nil, rctx, &ToolResult{Text: rctx.Problem, IsError: true}, nil
	}
	if !rctx.OK {
		return nil, rctx, &ToolResult{Text: rctx.NeedsInput()}, nil
	}
	if create {
		s, err = stores.Get(ctx, rctx.DBPath)
	} else {
		s, err = stores.GetExisting(ctx, rctx.DBPath)
	}
	if errors.Is(err, store.ErrVaultNotFound) {
		result := vaultMissingResult(rctx.DBPath)
		return nil, rctx, &result, nil
	}
	if err != nil {
		return nil, rctx, nil, err
	}
	r.RememberIfExists(ctx, s, rctx)
	return s, rctx, nil, nil
}

// refuseRememberedTarget returns an error result when rctx came from the
// global config's last-used project — which any session on this machine
// may have just changed — so an irreversible operation (described by
// action, e.g. "delete progress") never runs on a project the caller
// didn't name. It returns nil for any other source.
func refuseRememberedTarget(rctx ResolvedContext, action string) *ToolResult {
	if rctx.Source != SourceGlobalConfig {
		return nil
	}
	return &ToolResult{IsError: true, Text: fmt.Sprintf(
		"Refusing to %s in %q, which was only taken from the last session: pass \"project\" or \"workspace_root\" explicitly.",
		action, rctx.Label())}
}

// vaultMissingResult returns the error result for a call whose vault path,
// dbPath, holds no vault.
func vaultMissingResult(dbPath string) ToolResult {
	return ToolResult{IsError: true, Text: fmt.Sprintf(
		"No vault exists at %s. Check the \"path\" argument, the workspace's .sync82.json or \"sync82 config set-vault\"; a vault is created by create_project, init_project_memory or import_memory.",
		dbPath)}
}

// DBPathOrDefault resolves a tool call's optional "path" argument,
// rawPath, to a concrete vault path: the explicit path when given,
// otherwise the global config's vault path ("sync82 config set-vault"),
// otherwise DefaultDBPath. The tools that don't need full context resolution
// (list_projects, create_project, delete_project, rename_project,
// get_vault_config) and the export/import CLI commands use this directly.
func (r *Resolver) DBPathOrDefault(rawPath string) string {
	return r.vaultPath(rawPath, "", "")
}

// vaultPath applies the vault path precedence shared by every tier:
// explicit "path" argument > .sync82.json "path" > global config's vault
// path > DefaultDBPath (SYNC82_DB_PATH or ~/.sync82/knowledge.db). A
// relative .sync82.json path is resolved against localRoot, the directory
// holding that .sync82.json. Every returned path is absolute when it can
// be made so.
func (r *Resolver) vaultPath(explicit, local, localRoot string) string {
	if explicit != "" {
		return absPath(config.ResolvePath(explicit))
	}
	if local != "" {
		p := config.ResolvePath(local)
		if !filepath.IsAbs(p) && localRoot != "" {
			p = filepath.Join(localRoot, p)
		}
		return absPath(p)
	}
	cfg, err := config.ReadGlobalConfig()
	if err != nil {
		r.Logger.Error("read global config", "error", err)
	} else if cfg.VaultPath != "" {
		return absPath(config.ResolvePath(cfg.VaultPath))
	}
	return absPath(r.DefaultDBPath)
}

// absPath returns p made absolute against the current directory, or p
// unchanged when that is not possible.
func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// FormatLabel formats a project/subproject pair the way tool responses
// do: "project" alone when subproject is empty, otherwise
// "project/subproject".
func FormatLabel(project, subproject string) string {
	if subproject == "" {
		return project
	}
	return project + "/" + subproject
}

// Label returns the context's project and subproject formatted by
// FormatLabel.
func (c ResolvedContext) Label() string {
	return FormatLabel(c.Project, c.Subproject)
}

// ContextNote returns the " [project: x, from ..., vault: ...]" suffix
// tool responses append when the project was auto-discovered rather than
// passed explicitly. It always includes the resolved vault path, so a
// caller relying on auto-discovery can see which vault was used. It
// returns "" for SourceProvided, where the caller named the project.
func ContextNote(ctx ResolvedContext) string {
	switch ctx.Source {
	case SourceLocalConfig:
		return fmt.Sprintf(" [project: %s, from .sync82.json, vault: %s]", ctx.Label(), ctx.DBPath)
	case SourceGlobalConfig:
		return fmt.Sprintf(" [project: %s, from last session, vault: %s]", ctx.Label(), ctx.DBPath)
	default:
		return ""
	}
}

// samePath reports whether a and b name the same vault file once "~",
// "HOME" or "$HOME" is expanded and both are made absolute and cleaned.
func samePath(a, b string) bool {
	return filepath.Clean(absPath(config.ResolvePath(a))) == filepath.Clean(absPath(config.ResolvePath(b)))
}
