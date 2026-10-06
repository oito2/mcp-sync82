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

// Package installer provides the mechanics behind "sync82 install
// [target]" and "sync82 uninstall [target] [--purge]": the Target type
// describing each supported MCP client, the fixed Targets list, the Env
// the targets resolve their paths against, InstallTarget and
// UninstallTarget, which register or remove sync82 in one target by
// invoking the client's own MCP subcommands (KindCLI) or editing its JSON
// config files (KindFile), and the purge helpers listing and deleting the
// files of sync82's data directory. The command-line flows — listing
// detected targets, confirming, dispatching across all of them, and summarizing
// results — live in cmd/sync82's RunInstall and RunUninstall.
package installer
