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

import (
	"os"
	"path/filepath"
)

// jsFrameworks is the ordered table of npm packages reported as frameworks.
var jsFrameworks = []depPattern{
	{"React", []string{"react"}},
	{"Next.js", []string{"next"}},
	{"Vue", []string{"vue"}},
	{"Nuxt", []string{"nuxt"}},
	{"Svelte", []string{"svelte"}},
	{"Express", []string{"express"}},
	{"Fastify", []string{"fastify"}},
	{"NestJS", []string{"@nestjs/core"}},
	{"Koa", []string{"koa"}},
	{"Angular", []string{"@angular/core"}},
	{"Hono", []string{"hono"}},
	{"Elysia", []string{"elysiajs"}},
}

// jsNotableLibraries is the ordered table of npm packages reported as
// notable libraries; they are listed after the frameworks.
var jsNotableLibraries = []depPattern{
	{"Zod", []string{"zod"}},
	{"Prisma", []string{"prisma", "@prisma/client"}},
	{"MCP SDK", []string{"@modelcontextprotocol/sdk"}},
	{"Drizzle", []string{"drizzle-orm"}},
	{"Axios", []string{"axios"}},
	{"tRPC", []string{"@trpc/server"}},
	{"GraphQL", []string{"graphql"}},
	{"Anthropic SDK", []string{"@anthropic-ai/sdk"}},
	{"OpenAI SDK", []string{"openai"}},
}

// detectPackageJSON fills Description (if unset); Languages with
// TypeScript if a "typescript" dependency or a tsconfig.json is present,
// JavaScript otherwise; and Frameworks (known frameworks first, then
// notable libraries) from the package names in package.json's
// dependencies + devDependencies. Each field is decoded on its own, so a
// field with an unexpected type is ignored; only a file that is not a
// JSON object yields no signal.
func detectPackageJSON(root string, r *Result) {
	content, ok := readMarkerFile(root, "package.json")
	if !ok {
		return
	}

	description, deps, ok := decodeJSONManifest(content, nil, "dependencies", "devDependencies")
	if !ok {
		return // malformed package.json — no signal, not fatal
	}

	setDescription(r, description)

	_, statErr := os.Stat(filepath.Join(root, "tsconfig.json"))
	if deps["typescript"] || statErr == nil {
		appendUnique(&r.Languages, "TypeScript")
	} else {
		appendUnique(&r.Languages, "JavaScript")
	}

	matchDeps(deps, jsFrameworks, &r.Frameworks)
	matchDeps(deps, jsNotableLibraries, &r.Frameworks)
}
