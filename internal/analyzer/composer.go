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

package analyzer

import "strings"

// phpPackages is the ordered table of Composer packages reported as frameworks.
var phpPackages = []depPattern{
	{"Laravel", []string{"laravel/framework"}},
	{"Symfony", []string{"symfony/symfony"}},
	{"Slim", []string{"slim/slim"}},
	{"Guzzle", []string{"guzzlehttp/guzzle"}},
	{"Moodle Coding Standard", []string{"moodle/moodle-cs"}},
}

// detectComposer fills Description (if unset), Languages += PHP, and
// Frameworks from the lowercased package names in composer.json's
// require + require-dev. Each field is decoded on its own, so a field
// with an unexpected type is ignored; only a file that is not a JSON
// object yields no signal.
func detectComposer(root string, r *Result) {
	content, ok := readMarkerFile(root, "composer.json")
	if !ok {
		return
	}

	description, deps, ok := decodeJSONManifest(content, strings.ToLower, "require", "require-dev")
	if !ok {
		return
	}

	setDescription(r, description)
	appendUnique(&r.Languages, "PHP")
	matchDeps(deps, phpPackages, &r.Frameworks)
}
