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
	"slices"
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
)

// maxCompletionValues is the most values a completion returns, the limit
// the protocol allows in one completion result.
const maxCompletionValues = 100

// Complete returns the values of the prompt or resource-template argument
// called argument that start with value (case-insensitive), from the
// default vault, sorted: project names for "project", the subprojects of
// the project in args for "subproject", and the files of that project
// (and subproject, when given), plus the six standard ones, for "file". It
// returns at most maxCompletionValues values, with the total number of
// matches. An unknown argument, a missing vault, or a project or
// subproject that doesn't exist gives no values; other store failures are
// returned as errors.
func (r *Resources) Complete(ctx context.Context, argument, value string, args map[string]string) (values []string, total int, err error) {
	s, err := r.Stores.GetExisting(ctx, r.Resolver.DBPathOrDefault(""))
	if errors.Is(err, store.ErrVaultNotFound) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	var candidates []string
	project, subproject := NormalizeName(args["project"]), NormalizeName(args["subproject"])
	switch argument {
	case "project":
		projects, err := s.ListTopLevelProjects(ctx)
		if err != nil {
			return nil, 0, err
		}
		for _, p := range projects {
			candidates = append(candidates, p.Name)
		}
	case "subproject":
		if project == "" {
			return nil, 0, nil
		}
		parent, err := s.FindProjectByName(ctx, project, nil)
		if err != nil || parent == nil {
			return nil, 0, err
		}
		subs, err := s.ListSubprojects(ctx, parent.ID)
		if err != nil {
			return nil, 0, err
		}
		for _, sub := range subs {
			candidates = append(candidates, sub.Name)
		}
	case "file":
		candidates = slices.Clone(standardKinds)
		if project != "" {
			kinds, err := s.ListKinds(ctx, project, subproject, false)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return nil, 0, err
			}
			candidates = append(candidates, kinds...)
		}
	default:
		return nil, 0, nil
	}

	prefix := strings.ToLower(value)
	slices.Sort(candidates)
	candidates = slices.Compact(candidates)
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			total++
			if len(values) < maxCompletionValues {
				values = append(values, c)
			}
		}
	}
	return values, total, nil
}
