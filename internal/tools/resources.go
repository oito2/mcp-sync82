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
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
)

// resourceScheme is the URI prefix of every sync82 resource.
const resourceScheme = "sync82://projects/"

// ResourceMIMEType is the MIME type of every sync82 resource.
const ResourceMIMEType = "text/markdown"

// ErrResourceNotFound is returned by Resources.Read when a URI is not a
// sync82 resource URI, names an invalid or missing project or file, or the
// default vault does not exist.
var ErrResourceNotFound = errors.New("resource not found")

// ResourceTemplate describes one family of sync82 resources: the URI
// template, with {project}, {subproject} and {file} variables, and the
// texts shown to the client.
type ResourceTemplate struct {
	URITemplate string
	Name        string
	Title       string
	Description string
}

// ResourceTemplates lists the resource templates the server exposes, all
// read from the default vault.
var ResourceTemplates = []ResourceTemplate{
	{
		URITemplate: resourceScheme + "{project}/context",
		Name:        "project-context",
		Title:       "Project context",
		Description: "A project's memory in one block, as load_project_context returns it by default: current-state files in full and the 10 most recent entries of each log.",
	},
	{
		URITemplate: resourceScheme + "{project}/files/{file}",
		Name:        "project-file",
		Title:       "Project memory file",
		Description: "One memory file of a project (memory, architecture, stack, decisions, progress, next_steps or a custom one), as read_memory returns it.",
	},
	{
		URITemplate: resourceScheme + "{project}/subprojects/{subproject}/context",
		Name:        "subproject-context",
		Title:       "Subproject context",
		Description: "A subproject's memory in one block, as load_project_context returns it by default.",
	},
	{
		URITemplate: resourceScheme + "{project}/subprojects/{subproject}/files/{file}",
		Name:        "subproject-file",
		Title:       "Subproject memory file",
		Description: "One memory file of a subproject, as read_memory returns it.",
	},
}

// ResourceInfo describes one concrete resource listed to clients.
type ResourceInfo struct {
	URI         string
	Name        string
	Title       string
	Description string
}

// Resources serves the memory of the default vault (the one
// Resolver.DBPathOrDefault picks without a path) as read-only MCP
// resources. Reading never creates the vault and never changes the last
// used project.
type Resources struct {
	Resolver *Resolver
	Stores   *store.Manager
}

// List returns the context resource of every project and subproject in
// the default vault, in name order, or none when the vault does not exist.
// Store failures are returned as errors.
func (r *Resources) List(ctx context.Context) ([]ResourceInfo, error) {
	s, err := r.Stores.GetExisting(ctx, r.Resolver.DBPathOrDefault(""))
	if errors.Is(err, store.ErrVaultNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tree, err := s.ProjectTree(ctx)
	if err != nil {
		return nil, err
	}
	var out []ResourceInfo
	for _, p := range tree {
		out = append(out, contextResource(p.Name, ""))
		for _, sub := range p.Subprojects {
			out = append(out, contextResource(p.Name, sub))
		}
	}
	return out, nil
}

// contextResource returns the ResourceInfo of the context resource of
// project and, when not empty, subproject.
func contextResource(project, subproject string) ResourceInfo {
	label := FormatLabel(project, subproject)
	uri := resourceScheme + project + "/context"
	if subproject != "" {
		uri = resourceScheme + project + "/subprojects/" + subproject + "/context"
	}
	return ResourceInfo{
		URI:         uri,
		Name:        label + "-context",
		Title:       label + " — memory",
		Description: "The sync82 memory of " + label + ": current-state files and recent history.",
	}
}

// resourceTarget is what a sync82 resource URI names: a project, an
// optional subproject, and a file, or the whole context when file is "".
type resourceTarget struct {
	project    string
	subproject string
	file       string
}

// parseResourceURI splits a sync82 resource URI into its resourceTarget.
// Names are lower-cased and must follow the project-name and kind rules,
// which rules out percent-encoding, "..", empty segments, a query and a
// fragment. ok is false for any URI that is not exactly one of the
// ResourceTemplates.
func parseResourceURI(uri string) (target resourceTarget, ok bool) {
	rest, found := strings.CutPrefix(uri, resourceScheme)
	if !found {
		return resourceTarget{}, false
	}
	seg := strings.Split(rest, "/")
	switch {
	case len(seg) == 2 && seg[1] == "context":
		target = resourceTarget{project: seg[0]}
	case len(seg) == 3 && seg[1] == "files":
		target = resourceTarget{project: seg[0], file: seg[2]}
	case len(seg) == 4 && seg[1] == "subprojects" && seg[3] == "context":
		target = resourceTarget{project: seg[0], subproject: seg[2]}
	case len(seg) == 5 && seg[1] == "subprojects" && seg[3] == "files":
		target = resourceTarget{project: seg[0], subproject: seg[2], file: seg[4]}
	default:
		return resourceTarget{}, false
	}
	if ValidateTarget(target.project, target.subproject) != nil {
		return resourceTarget{}, false
	}
	target.project, target.subproject = NormalizeName(target.project), NormalizeName(target.subproject)
	if target.file != "" {
		kind, err := validateKind(target.file)
		if err != nil {
			return resourceTarget{}, false
		}
		target.file = kind
	}
	return target, true
}

// Read returns the Markdown text of the resource at uri: the context block
// of load_project_context in summary mode, or the content of one file as
// read_memory returns it. It returns ErrResourceNotFound when uri is not a
// valid sync82 resource URI, the default vault does not exist, or the
// project, subproject or file does not; other store failures are returned
// as errors.
func (r *Resources) Read(ctx context.Context, uri string) (string, error) {
	target, ok := parseResourceURI(uri)
	if !ok {
		return "", ErrResourceNotFound
	}
	s, err := r.Stores.GetExisting(ctx, r.Resolver.DBPathOrDefault(""))
	if errors.Is(err, store.ErrVaultNotFound) {
		return "", ErrResourceNotFound
	}
	if err != nil {
		return "", err
	}

	var text string
	if target.file == "" {
		text, err = projectContext(ctx, s, target.project, target.subproject, summaryContextArgs())
	} else {
		var found bool
		text, found, err = readMemoryContent(ctx, s, target.project, target.subproject, target.file, false)
		if err == nil && !found {
			return "", ErrResourceNotFound
		}
		text = strings.TrimSpace(text)
	}
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrResourceNotFound
	}
	if err != nil {
		return "", err
	}
	return text, nil
}
