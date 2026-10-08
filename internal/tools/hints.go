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

// ToolHints describes how a tool affects the vault, for clients deciding
// when to ask the user for confirmation: Title is a human-readable name,
// ReadOnly means the tool changes no memory, Destructive (meaningful only
// when not ReadOnly) that it may overwrite or delete existing memory rather
// than only add to it, Idempotent that repeating a call with the same
// arguments changes nothing more, and ChangesProjects that a successful
// call may create, delete or rename a project, changing the resource list.
// Every sync82 tool works on a closed domain (the local vault and files),
// never on an open world of external entities.
type ToolHints struct {
	Title           string
	ReadOnly        bool
	Destructive     bool
	Idempotent      bool
	ChangesProjects bool
}

// toolHints holds the ToolHints of every registered tool, by name.
// Recording the last used project is bookkeeping, not a change to memory,
// so the tools that only read memory are read-only.
var toolHints = map[string]ToolHints{
	"list_projects":         {Title: "List projects", ReadOnly: true},
	"create_project":        {Title: "Create a project", Idempotent: true, ChangesProjects: true},
	"delete_project":        {Title: "Delete a project", Destructive: true, Idempotent: true, ChangesProjects: true},
	"rename_project":        {Title: "Rename a project", ChangesProjects: true},
	"get_vault_config":      {Title: "Show the vault configuration", ReadOnly: true},
	"list_files":            {Title: "List memory files", ReadOnly: true},
	"read_memory":           {Title: "Read a memory file", ReadOnly: true},
	"write_memory":          {Title: "Overwrite a memory file", Destructive: true, Idempotent: true},
	"append_memory":         {Title: "Append a memory entry"},
	"delete_memory":         {Title: "Delete a memory file", Destructive: true, Idempotent: true},
	"edit_entry":            {Title: "Edit a memory entry", Destructive: true},
	"archive_memory":        {Title: "Archive old entries"},
	"search_memory":         {Title: "Search memory", ReadOnly: true},
	"load_project_context":  {Title: "Load the project context", ReadOnly: true},
	"check_project_health":  {Title: "Check the project memory", ReadOnly: true},
	"init_project_memory":   {Title: "Initialize project memory", Idempotent: true, ChangesProjects: true},
	"update_project_memory": {Title: "Save the session to memory", Destructive: true},
	"export_memory":         {Title: "Export memory to Markdown files", Destructive: true, Idempotent: true},
	"import_memory":         {Title: "Import memory from Markdown files", Destructive: true, Idempotent: true, ChangesProjects: true},
}

// HintsFor returns the ToolHints of the tool called name, and whether it
// has any.
func HintsFor(name string) (ToolHints, bool) {
	h, ok := toolHints[name]
	return h, ok
}
