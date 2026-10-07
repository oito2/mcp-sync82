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

import "github.com/oito2/mcp-sync82/internal/store"

// Registered returns every Tool the sync82 server exposes, each wired to
// resolver (project resolution) and stores (vault access). It is the single
// list of tools the server registers.
func Registered(resolver *Resolver, stores *store.Manager) []Tool {
	return []Tool{
		&ListProjectsTool{Resolver: resolver, Stores: stores},
		&CreateProjectTool{Resolver: resolver, Stores: stores},
		&DeleteProjectTool{Resolver: resolver, Stores: stores},
		&RenameProjectTool{Resolver: resolver, Stores: stores},
		&GetVaultConfigTool{Resolver: resolver, Stores: stores},
		&ListFilesTool{Resolver: resolver, Stores: stores},
		&ReadMemoryTool{Resolver: resolver, Stores: stores},
		&WriteMemoryTool{Resolver: resolver, Stores: stores},
		&AppendMemoryTool{Resolver: resolver, Stores: stores},
		&DeleteMemoryTool{Resolver: resolver, Stores: stores},
		&EditEntryTool{Resolver: resolver, Stores: stores},
		&SearchMemoryTool{Resolver: resolver, Stores: stores},
		&LoadProjectContextTool{Resolver: resolver, Stores: stores},
		&CheckProjectHealthTool{Resolver: resolver, Stores: stores},
		&InitProjectMemoryTool{Resolver: resolver, Stores: stores},
		&UpdateProjectMemoryTool{Resolver: resolver, Stores: stores},
		&ArchiveMemoryTool{Resolver: resolver, Stores: stores},
		&ExportMemoryTool{Resolver: resolver, Stores: stores},
		&ImportMemoryTool{Resolver: resolver, Stores: stores},
	}
}
