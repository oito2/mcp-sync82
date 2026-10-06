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

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/oito2/mcp-sync82/internal/store"
	"github.com/oito2/mcp-sync82/internal/tools"
)

// memoryCmdDeps bundles the dependencies shared by RunExport and RunImport:
// the output streams, the resolver that picks the vault path, and the
// manager that opens vault stores.
type memoryCmdDeps struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Resolver *tools.Resolver
	Stores   *store.Manager
}

// parsedArgs is a subcommand's command line split by parseArgs. Flag names
// are stored without their leading dashes.
type parsedArgs struct {
	flags      map[string]bool   // boolean flags present, by name ("all")
	values     map[string]string // value flags present, by name ("path")
	positional []string          // non-flag arguments, in command-line order
}

// parseArgs splits args into the boolean flags in boolFlags, the value
// flags in valueFlags (given as "--name value" or "--name=value") and the
// positional arguments, in order. Flags may appear anywhere. Any other
// argument starting with "-", a value flag without a value or with an
// empty one, and a flag given twice are errors — so a typo is reported
// instead of being taken as a project or directory name. It returns the
// split command line, or the first such error found.
func parseArgs(args []string, boolFlags, valueFlags []string) (parsedArgs, error) {
	p := parsedArgs{flags: map[string]bool{}, values: map[string]string{}}
	isBool := map[string]bool{}
	for _, f := range boolFlags {
		isBool[f] = true
	}
	isValue := map[string]bool{}
	for _, f := range valueFlags {
		isValue[f] = true
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			p.positional = append(p.positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		switch {
		case isBool[name] && !hasValue:
			if p.flags[name] {
				return parsedArgs{}, fmt.Errorf("--%s given twice", name)
			}
			p.flags[name] = true
		case isValue[name]:
			if _, dup := p.values[name]; dup {
				return parsedArgs{}, fmt.Errorf("--%s given twice", name)
			}
			if !hasValue {
				if i+1 >= len(args) {
					return parsedArgs{}, fmt.Errorf("--%s requires a value", name)
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return parsedArgs{}, fmt.Errorf("--%s requires a non-empty value", name)
			}
			p.values[name] = value
		default:
			return parsedArgs{}, fmt.Errorf("unknown option %q", arg)
		}
	}
	return p, nil
}
