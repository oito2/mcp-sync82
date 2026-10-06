#!/bin/sh
# Copyright (C) 2026  OITO2
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

# Linux entry point of the sync82 MCPB bundle: replaces itself (exec) with
# the sync82 binary next to it that matches this machine's CPU
# architecture, passing every argument through.
#
# Inputs: every argument is forwarded unchanged to the selected binary
# (sync82-linux-amd64 or sync82-linux-arm64, located in this script's
# directory); the architecture is read from `uname -m`.
# Exit behavior: on a supported architecture the process becomes the sync82
# binary, so its exit status is the script's. On any other architecture it
# prints an error to stderr and exits with status 1. Nothing is written to
# stdout, which carries the MCP stdio stream.

dir=$(dirname -- "$0")
arch=$(uname -m)

case "$arch" in
x86_64 | amd64) exec "$dir/sync82-linux-amd64" "$@" ;;
aarch64 | arm64) exec "$dir/sync82-linux-arm64" "$@" ;;
*)
	echo "sync82: unsupported CPU architecture: $arch (supported: x86_64, aarch64)" >&2
	exit 1
	;;
esac
