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
	"regexp"
	"strings"
)

// javaPackages matches Maven group IDs and Gradle plugin IDs. Every
// coordinate also contributes its parent groups (see javaCoordinateTokens),
// so "io.micronaut" matches "io.micronaut.data" and the
// "io.micronaut.application" plugin.
var javaPackages = []depPattern{
	{"Spring Boot", []string{"org.springframework.boot"}},
	{"Quarkus", []string{"io.quarkus"}},
	{"Micronaut", []string{"io.micronaut"}},
	{"Ktor", []string{"io.ktor"}},
}

// javaDetector reads pom.xml, build.gradle and build.gradle.kts; the Kotlin
// language is reported for build.gradle.kts and Java for the other files.
var javaDetector = manifestDetector{
	files: fixedFiles("pom.xml", "build.gradle", "build.gradle.kts"),
	language: func(file string) string {
		if file == "build.gradle.kts" {
			return "Kotlin"
		}
		return "Java"
	},
	deps:     javaDependencyTokens,
	patterns: javaPackages,
}

// Regular expressions used to extract coordinates from Maven and Gradle files:
// xmlComment matches an XML comment; pomCoordBlock matches a <dependency>,
// <plugin>, <parent> or <extension> element and captures its body;
// pomGroupID and pomArtifactID capture the groupId and artifactId values;
// gradleCoord captures "group:artifact[:version]" strings; gradleMapGroup
// captures a map-notation group; gradlePluginID captures a plugin id.
var (
	xmlComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	pomCoordBlock  = regexp.MustCompile(`(?s)<(?:dependency|plugin|parent|extension)>(.*?)</(?:dependency|plugin|parent|extension)>`)
	pomGroupID     = regexp.MustCompile(`<groupId>\s*([^<\s]+)\s*</groupId>`)
	pomArtifactID  = regexp.MustCompile(`<artifactId>\s*([^<\s]+)\s*</artifactId>`)
	gradleCoord    = regexp.MustCompile(`["']([A-Za-z0-9_.\-]+):([A-Za-z0-9_.\-]+)(?::[^"'\s]*)?["']`)
	gradleMapGroup = regexp.MustCompile(`\bgroup\s*[:=]\s*["']([^"']+)["']`)
	gradlePluginID = regexp.MustCompile(`\bid\s*\(?\s*["']([^"']+)["']`)
)

// detectJava fills Languages += Java when pom.xml or build.gradle is
// present, or += Kotlin when build.gradle.kts is present (both can be set
// at once), and Frameworks from the Maven coordinates and Gradle plugin
// IDs those files declare, compared as exact group/artifact tokens.
func detectJava(root string, r *Result) {
	javaDetector.detect(root, r)
}

// javaDependencyTokens returns the lowercased coordinate tokens declared
// in one Maven or Gradle build file: for pom.xml, the groupId/artifactId
// of every <dependency>, <plugin>, <parent> and <extension> element
// (XML comments removed); for Gradle scripts, every quoted
// "group:artifact[:version]" string, every map-notation group, and every
// plugin id (lines starting with "//" removed).
func javaDependencyTokens(file, content string) []string {
	var tokens []string
	if file == "pom.xml" {
		content = xmlComment.ReplaceAllString(content, "")
		for _, block := range pomCoordBlock.FindAllStringSubmatch(content, -1) {
			var group, artifact string
			if m := pomGroupID.FindStringSubmatch(block[1]); m != nil {
				group = m[1]
			}
			if m := pomArtifactID.FindStringSubmatch(block[1]); m != nil {
				artifact = m[1]
			}
			tokens = append(tokens, javaCoordinateTokens(group, artifact)...)
		}
		return tokens
	}

	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	content = b.String()

	for _, m := range gradleCoord.FindAllStringSubmatch(content, -1) {
		tokens = append(tokens, javaCoordinateTokens(m[1], m[2])...)
	}
	for _, m := range gradleMapGroup.FindAllStringSubmatch(content, -1) {
		tokens = append(tokens, javaCoordinateTokens(m[1], "")...)
	}
	for _, m := range gradlePluginID.FindAllStringSubmatch(content, -1) {
		tokens = append(tokens, javaCoordinateTokens(m[1], "")...)
	}
	return tokens
}

// javaCoordinateTokens returns the lowercased tokens of one coordinate:
// the group and each of its parent groups split on "." boundaries
// ("io.micronaut.data" yields "io.micronaut.data", "io.micronaut", "io"),
// the artifact, and "group:artifact". Empty parts are omitted.
func javaCoordinateTokens(group, artifact string) []string {
	group = strings.ToLower(group)
	artifact = strings.ToLower(artifact)

	var tokens []string
	for g := group; g != ""; {
		tokens = append(tokens, g)
		i := strings.LastIndexByte(g, '.')
		if i < 0 {
			break
		}
		g = g[:i]
	}
	if artifact != "" {
		tokens = append(tokens, artifact)
		if group != "" {
			tokens = append(tokens, group+":"+artifact)
		}
	}
	return tokens
}
