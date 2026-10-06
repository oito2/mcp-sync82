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

// Package tools implements sync82's MCP tools — one exported type per
// tool (CreateProjectTool, ReadMemoryTool, WriteMemoryTool, and so on),
// each implementing the common Tool interface: Name, Description,
// InputSchema, and a Validate/Execute pair. The server turns an error from
// either step into a tool result with IsError set, so the calling agent
// can read the message and retry with corrected input.
//
// Resolver implements the three-tier project-resolution cascade (explicit
// "project" argument, .sync82.json at workspace_root — or, opt-in,
// discovered by walking up from it — and the global config's last-used
// project) that every project-scoped tool relies on. Logic shared by more
// than one tool lives in one place: kind-name validation and the six standard
// kinds (validateKind, standardKinds), the "## YYYY-MM-DD" header
// append-only kinds use (dateHeaderPattern), the write/append logic shared
// by write_memory, append_memory, and update_project_memory
// (writeMemoryCore, appendMemoryCore), and export/import (ExportProject,
// ImportProject, also called directly by the "sync82" CLI's export/import
// subcommands).
package tools
