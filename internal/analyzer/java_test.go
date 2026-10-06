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
	"slices"
	"testing"
)

// TestDetectJava_ViaPomXML verifies Java and framework detection from pom.xml.
func TestDetectJava_ViaPomXML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", `<project>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>`)

	var r Result
	detectJava(dir, &r)
	if !slices.Contains(r.Languages, "Java") {
		t.Errorf("Languages = %v, want Java", r.Languages)
	}
	if slices.Contains(r.Languages, "Kotlin") {
		t.Errorf("Languages = %v, want no Kotlin from pom.xml alone", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Spring Boot") {
		t.Errorf("Frameworks = %v, want Spring Boot", r.Frameworks)
	}
}

// TestDetectJava_ViaBuildGradleKts verifies that build.gradle.kts yields Kotlin and framework detection.
func TestDetectJava_ViaBuildGradleKts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle.kts", `dependencies {
    implementation("io.ktor:ktor-server-core:2.3.0")
}`)

	var r Result
	detectJava(dir, &r)
	if !slices.Contains(r.Languages, "Kotlin") {
		t.Errorf("Languages = %v, want Kotlin", r.Languages)
	}
	if slices.Contains(r.Languages, "Java") {
		t.Errorf("Languages = %v, want no Java from build.gradle.kts alone", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Ktor") {
		t.Errorf("Frameworks = %v, want Ktor", r.Frameworks)
	}
}

// TestDetectJava_ViaBuildGradle verifies that build.gradle yields Java and framework detection.
func TestDetectJava_ViaBuildGradle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle", `dependencies {
    implementation 'io.quarkus:quarkus-resteasy'
}`)

	var r Result
	detectJava(dir, &r)
	if !slices.Contains(r.Languages, "Java") {
		t.Errorf("Languages = %v, want Java", r.Languages)
	}
	if !slices.Contains(r.Frameworks, "Quarkus") {
		t.Errorf("Frameworks = %v, want Quarkus", r.Frameworks)
	}
}

// TestDetectJava_MissingAllMarkersIsNoOp verifies that detectJava changes nothing when no build file exists.
func TestDetectJava_MissingAllMarkersIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var r Result
	detectJava(dir, &r)
	if len(r.Languages) != 0 {
		t.Fatalf("expected no signal when no Java/Kotlin marker file exists, got %+v", r)
	}
}

// TestDetectJava_MatchesCoordinatesNotSubstrings guards against reporting
// a framework name that only appears inside another coordinate (ktorm
// contains "ktor").
func TestDetectJava_MatchesCoordinatesNotSubstrings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle.kts", `dependencies {
    implementation("org.ktorm:ktorm-core:3.6.0")
    // implementation("io.quarkus:quarkus-core:3.0.0")
    implementation("com.example:spring-boot-helper:1.0")
}`)
	writeFile(t, dir, "pom.xml", `<project>
  <groupId>com.example.micronaut</groupId>
  <!-- <dependency><groupId>io.ktor</groupId></dependency> -->
  <dependencies>
    <dependency>
      <groupId>io.quarkusx</groupId>
      <artifactId>quarkus-core</artifactId>
    </dependency>
  </dependencies>
</project>`)

	var r Result
	detectJava(dir, &r)
	if !slices.Equal(r.Languages, []string{"Java", "Kotlin"}) {
		t.Errorf("Languages = %v, want [Java Kotlin]", r.Languages)
	}
	if len(r.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want none", r.Frameworks)
	}
}

// TestDetectJava_GroupsPluginsAndMapNotation verifies that parent groups, Gradle plugin ids and map-notation groups
// are matched.
func TestDetectJava_GroupsPluginsAndMapNotation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle", `plugins {
    id 'io.micronaut.application' version '4.0.0'
}
dependencies {
    implementation group: 'io.ktor', name: 'ktor-server-core', version: '2.3.0'
}`)
	writeFile(t, dir, "pom.xml", `<project>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
  </parent>
  <dependencies>
    <dependency>
      <groupId>io.quarkus.platform</groupId>
      <artifactId>quarkus-bom</artifactId>
    </dependency>
  </dependencies>
</project>`)

	var r Result
	detectJava(dir, &r)
	want := []string{"Spring Boot", "Quarkus", "Micronaut", "Ktor"}
	if !slices.Equal(r.Frameworks, want) {
		t.Errorf("Frameworks = %v, want %v", r.Frameworks, want)
	}
}
