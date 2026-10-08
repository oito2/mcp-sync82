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

package selfupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestParseVersion checks parsing of plain, prefixed, pre-release, build
// and malformed version strings.
func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want semver
		ok   bool
	}{
		{"v1.2.3", semver{major: 1, minor: 2, patch: 3}, true},
		{"1.2.3", semver{major: 1, minor: 2, patch: 3}, true},
		{"v0.1.0", semver{minor: 1}, true},
		{"v1.2.3-rc.1", semver{major: 1, minor: 2, patch: 3, pre: []string{"rc", "1"}}, true},
		{"v1.2.3+build5", semver{major: 1, minor: 2, patch: 3}, true},
		{"v1.2.3-beta+exp.sha.5114f85", semver{major: 1, minor: 2, patch: 3, pre: []string{"beta"}}, true},
		{"v1.2.3-", semver{}, false},
		{"v1.2.3-rc..1", semver{}, false},
		{"not-a-version", semver{}, false},
		{"v1.2", semver{}, false},
		{"v1.-2.3", semver{}, false},
	}
	for _, c := range cases {
		got, ok := parseVersion(c.in)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseVersion(%q) = (%+v, %v), want (%+v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestIsNewerVersion checks semver precedence, including pre-release
// identifiers and Go pseudo-versions, and the fallback for unparsable
// versions.
func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.3", "v1.2.2", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.4", false},
		{"v2.0.0", "v1.9.9", true},
		{"v1.3.0", "v1.2.9", true},
		{"v1.2.0", "v1.10.0", false},
		{"garbage", "v1.0.0", true}, // unparseable latest: fall back to "different" = update
		{"v1.0.0", "v1.0.0", false},
		// A release outranks its own pre-releases.
		{"v1.2.3", "v1.2.3-rc1", true},
		{"v1.2.3-rc1", "v1.2.3", false},
		// Pre-release identifiers: numeric ones numerically, numeric below
		// alphanumeric, a shorter list below a longer one.
		{"v1.2.3-rc.10", "v1.2.3-rc.2", true},
		{"v1.2.3-rc.2", "v1.2.3-rc.10", false},
		{"v1.2.3-alpha.beta", "v1.2.3-alpha.1", true},
		{"v1.2.3-alpha.1", "v1.2.3-alpha", true},
		{"v1.2.3-beta", "v1.2.3-alpha", true},
		// Build metadata is ignored.
		{"v1.2.3+build.2", "v1.2.3+build.1", false},
		// A Go pseudo-version for a build past v1.2.3 is a pre-release of
		// v1.2.4: newer than v1.2.3, older than v1.2.4.
		{"v1.2.3", "v1.2.4-0.20261006120000-abcdef123456", false},
		{"v1.2.4", "v1.2.4-0.20261006120000-abcdef123456", true},
	}
	for _, c := range cases {
		if got := isNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

// TestAssetName checks the release asset names, including the ".exe" suffix
// on Windows.
func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"linux", "amd64", "sync82_linux_amd64"},
		{"linux", "arm64", "sync82_linux_arm64"},
		{"darwin", "arm64", "sync82_darwin_arm64"},
		{"windows", "amd64", "sync82_windows_amd64.exe"},
	}
	for _, c := range cases {
		if got := AssetName(c.goos, c.goarch); got != c.want {
			t.Errorf("AssetName(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

// TestFindChecksum checks that findChecksum returns the hash of a listed
// file and an error for an unlisted one.
func TestFindChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	content := "abc123  sync82_linux_amd64\ndef456  sync82_darwin_amd64\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findChecksum(path, "sync82_linux_amd64")
	if err != nil {
		t.Fatalf("findChecksum: %v", err)
	}
	if got != "abc123" {
		t.Errorf("findChecksum() = %q, want %q", got, "abc123")
	}

	if _, err := findChecksum(path, "nonexistent"); err == nil {
		t.Fatal("expected an error for a filename not present in checksums.txt")
	}
}

// TestFindChecksum_SHA256SumOutputVariants covers the filename forms
// sha256sum emits: "./name" when run on a "./*" glob, and "*name" in
// binary mode. Hash case is normalized to lower case.
func TestFindChecksum_SHA256SumOutputVariants(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"abc123  ./sync82_linux_amd64", "abc123"},
		{"abc123 *sync82_linux_amd64", "abc123"},
		{"ABC123  sync82_linux_amd64", "abc123"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "checksums.txt")
		if err := os.WriteFile(path, []byte(c.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := findChecksum(path, "sync82_linux_amd64")
		if err != nil {
			t.Errorf("findChecksum(%q): %v", c.line, err)
			continue
		}
		if got != c.want {
			t.Errorf("findChecksum(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

// acceptAnyURL is a Deps.ValidateURL stub that accepts every URL, so tests
// can download from a local httptest server.
func acceptAnyURL(string) error { return nil }

// acceptAnyAsset is the Deps.ValidateAsset counterpart of acceptAnyURL.
func acceptAnyAsset(string, string) error { return nil }

// reportVersion returns a RunVersion stub reporting v for any binary.
func reportVersion(v string) func(context.Context, string) (string, error) {
	return func(context.Context, string) (string, error) { return v, nil }
}

// TestReplaceWithBackup_KeepsPreviousBinary checks that the new binary takes
// the current path and the old one is kept at its backup path.
func TestReplaceWithBackup_KeepsPreviousBinary(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "sync82")
	newPath := filepath.Join(dir, "sync82-new")
	if err := os.WriteFile(currentPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceWithBackup(currentPath, newPath); err != nil {
		t.Fatalf("replaceWithBackup: %v", err)
	}
	if data, _ := os.ReadFile(currentPath); string(data) != "new" {
		t.Errorf("current = %q, want %q", data, "new")
	}
	if data, _ := os.ReadFile(backupPath(currentPath)); string(data) != "old" {
		t.Errorf("backup = %q, want %q", data, "old")
	}
}

// TestReplaceWithBackup_RestoresOnFailure checks that when the new binary
// cannot be moved into place, the original binary is put back at its path.
func TestReplaceWithBackup_RestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "sync82")
	if err := os.WriteFile(currentPath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceWithBackup(currentPath, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected an error when the new binary doesn't exist")
	}
	if data, err := os.ReadFile(currentPath); err != nil || string(data) != "old" {
		t.Fatalf("current = %q, %v; want the original restored", data, err)
	}
}

// TestValidateAssetURL checks that GitHub HTTPS URLs are accepted and plain
// HTTP, foreign hosts, look-alike hosts and file URLs are rejected.
func TestValidateAssetURL(t *testing.T) {
	for _, ok := range []string{
		"https://github.com/oito2/mcp-sync82/releases/download/v1.0.0/sync82_linux_amd64",
		"https://objects.githubusercontent.com/github-production-release-asset/1",
		"https://release-assets.githubusercontent.com/github-production-release-asset/1",
	} {
		if err := validateAssetURL(ok); err != nil {
			t.Errorf("validateAssetURL(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{
		"http://github.com/x",
		"https://evil.example/sync82",
		"https://github.com.evil.example/x",
		"file:///etc/passwd",
		"https://raw.githubusercontent.com/someone/repo/main/sync82",
		"https://gist.githubusercontent.com/someone/1/raw/sync82",
		"https://github.com:8443/oito2/mcp-sync82/releases/download/v1.0.0/x",
		"https://user@github.com/oito2/mcp-sync82/releases/download/v1.0.0/x",
	} {
		if err := validateAssetURL(bad); err == nil {
			t.Errorf("validateAssetURL(%q) = nil, want an error", bad)
		}
	}
}

// TestSmokeTest_RequiresReleaseVersion checks that smokeTest fails for a
// binary reporting another version or failing to run.
func TestSmokeTest_RequiresReleaseVersion(t *testing.T) {
	ctx := context.Background()
	if err := smokeTest(ctx, reportVersion("v2.0.0"), "bin", "v2.0.0"); err != nil {
		t.Errorf("matching version: %v", err)
	}
	if err := smokeTest(ctx, reportVersion("dev"), "bin", "v2.0.0"); err == nil {
		t.Error("expected an error for a binary reporting another version")
	}
	failing := func(context.Context, string) (string, error) { return "", fmt.Errorf("exec format error") }
	if err := smokeTest(ctx, failing, "bin", "v2.0.0"); err == nil {
		t.Error("expected an error for a binary that doesn't run")
	}
}

// fakeReleaseServer starts an httptest server that serves, at "/release", a
// GitHub-Releases-shaped response for tag with assets assetName and
// checksums.txt, plus the downloads those assets point at: assetBytes and a
// checksums.txt line pairing checksum with assetName. The server is closed
// when the test ends.
func fakeReleaseServer(t *testing.T, tag string, assetBytes []byte, assetName string, checksum string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	var serverURL string
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		release := githubRelease{
			TagName: tag,
			Assets: []githubAsset{
				{Name: assetName, BrowserDownloadURL: serverURL + "/assets/" + assetName},
				{Name: "checksums.txt", BrowserDownloadURL: serverURL + "/assets/checksums.txt"},
			},
		}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encode release: %v", err)
		}
	})
	mux.HandleFunc("/assets/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(assetBytes); err != nil {
			t.Errorf("write asset: %v", err)
		}
	})
	mux.HandleFunc("/assets/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  ./%s\n", checksum, assetName)
	})

	srv := httptest.NewServer(mux)
	serverURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

// sha256Hex returns the lowercase hex SHA-256 digest of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestRunSelfUpdate_RefusesDevBuild checks that a development build exits
// with code 1 and a message about the missing version.
func TestRunSelfUpdate_RefusesDevBuild(t *testing.T) {
	var stdout, stderr bytes.Buffer
	deps := Deps{Version: "dev"}
	code := RunSelfUpdate(context.Background(), nil, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "development build") {
		t.Errorf("stderr = %q, want it to mention a development build", stderr.String())
	}
}

// TestIsValidReleaseTag checks which tag strings are accepted as release tags.
func TestIsValidReleaseTag(t *testing.T) {
	cases := []struct {
		tag  string
		want bool
	}{
		{"v1.2.3", true},
		{"v0.0.1", true},
		{"1.2.3", false},      // missing leading "v"
		{"v1.2.3-rc1", false}, // any suffix rejected outright
		{"v1.2.3-4-gabcdef", false},
		{"v1.2", false},
		{"v1.2.3/../../etc", false},
		{"garbage", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isValidReleaseTag(c.tag); got != c.want {
			t.Errorf("isValidReleaseTag(%q) = %v, want %v", c.tag, got, c.want)
		}
	}
}

// TestRunSelfUpdate_RejectsUnexpectedReleaseTagFormat checks that a malformed
// release tag makes the command exit with code 1.
func TestRunSelfUpdate_RejectsUnexpectedReleaseTagFormat(t *testing.T) {
	// A tag that is not a plain vMAJOR.MINOR.PATCH is rejected.
	srv := fakeReleaseServer(t, "v1.2.3/../../etc/passwd", []byte("irrelevant"), "sync82_linux_amd64", "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), nil, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 for a malformed release tag", code)
	}
	if !strings.Contains(stderr.String(), "Unexpected release tag format") {
		t.Errorf("stderr = %q, want it to reject the malformed tag", stderr.String())
	}
}

// TestRunSelfUpdate_AlreadyUpToDate checks that an equal version exits with
// code 0 without updating.
func TestRunSelfUpdate_AlreadyUpToDate(t *testing.T) {
	srv := fakeReleaseServer(t, "v1.0.0", []byte("irrelevant"), "sync82_linux_amd64", "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), nil, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Already up to date") {
		t.Errorf("stdout = %q, want it to report up to date", stdout.String())
	}
}

// TestRunSelfUpdate_CheckOnly_ReportsWithoutDownloading checks that --check
// reports an available update, exits with code 0 and downloads nothing.
func TestRunSelfUpdate_CheckOnly_ReportsWithoutDownloading(t *testing.T) {
	assetName := "sync82_linux_amd64"
	srv := fakeReleaseServer(t, "v2.0.0", []byte("should not be downloaded"), assetName, "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--check"}, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Update available: v1.0.0 → v2.0.0") {
		t.Errorf("stdout = %q, want it to report the available update", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(deps.TempDir, assetName)); err == nil {
		t.Error("--check should never download the binary")
	}
}

// TestRunSelfUpdate_DeclinesConfirmation checks that answering no aborts with
// exit code 0 and leaves the binary unchanged.
func TestRunSelfUpdate_DeclinesConfirmation(t *testing.T) {
	srv := fakeReleaseServer(t, "v2.0.0", []byte("should not be downloaded"), "sync82_linux_amd64", "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), nil, deps, strings.NewReader("n\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Aborted.") {
		t.Errorf("stdout = %q, want \"Aborted.\"", stdout.String())
	}
}

// TestRunSelfUpdate_ClosedStdinIsAnError checks that a closed stdin at the
// "Update now?" prompt exits with code 1 and suggests --yes, without
// downloading anything.
func TestRunSelfUpdate_ClosedStdinIsAnError(t *testing.T) {
	srv := fakeReleaseServer(t, "v2.0.0", []byte("should not be downloaded"), "sync82_linux_amd64", "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), nil, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "stdin is closed") || !strings.Contains(stderr.String(), "--yes") {
		t.Errorf("stderr = %q, want the closed-stdin error suggesting --yes", stderr.String())
	}
}

// TestRunSelfUpdate_ChecksumMismatch checks that a download whose hash differs
// from checksums.txt exits with code 1 and leaves the binary unchanged.
func TestRunSelfUpdate_ChecksumMismatch(t *testing.T) {
	assetName := "sync82_linux_amd64"
	assetBytes := []byte("binary content")
	srv := fakeReleaseServer(t, "v2.0.0", assetBytes, assetName, "0000000000000000000000000000000000000000000000000000000000000000")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Checksum mismatch") {
		t.Errorf("stderr = %q, want it to mention checksum mismatch", stderr.String())
	}
}

// TestRunSelfUpdate_FullSuccess checks a complete update: the binary is
// replaced, the previous one is kept as a backup and the staging directory is
// removed.
func TestRunSelfUpdate_FullSuccess(t *testing.T) {
	assetName := "sync82_linux_amd64"
	newBinaryContent := []byte("new binary bytes")
	checksum := sha256Hex(newBinaryContent)

	srv := fakeReleaseServer(t, "v2.0.0", newBinaryContent, assetName, checksum)

	currentExeDir := t.TempDir()
	currentExePath := filepath.Join(currentExeDir, "sync82")
	if err := os.WriteFile(currentExePath, []byte("old binary bytes"), 0o755); err != nil {
		t.Fatal(err)
	}

	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return currentExePath, nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("v2.0.0"),
	}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Updated to v2.0.0") {
		t.Errorf("stdout = %q, want it to report the update", stdout.String())
	}

	data, err := os.ReadFile(currentExePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, newBinaryContent) {
		t.Errorf("currentExePath content = %q, want %q", data, newBinaryContent)
	}
	if old, _ := os.ReadFile(currentExePath + ".bak"); string(old) != "old binary bytes" {
		t.Errorf("backup content = %q, want the previous binary", old)
	}

	// The staging directory is removed after the update.
	entries, err := os.ReadDir(deps.TempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected TempDir to be cleaned up, found: %v", entries)
	}
}

// TestDownloadToFile_RejectsResponseOverMaxDownloadSize checks that a body
// larger than maxDownloadSize makes downloadToFile fail.
func TestDownloadToFile_RejectsResponseOverMaxDownloadSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		buf := make([]byte, 1<<20) // 1MB chunks
		var written int64
		for written < maxDownloadSize+1<<20 {
			n, err := w.Write(buf)
			written += int64(n)
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	destPath := filepath.Join(t.TempDir(), "oversized.bin")
	err := downloadToFile(context.Background(), srv.Client(), srv.URL, destPath, acceptAnyURL)
	if err == nil {
		t.Fatal("expected an error for a response exceeding maxDownloadSize")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("err = %v, want it to mention the download limit was exceeded", err)
	}
}

// TestDownloadToFile_AllowsResponseAtOrUnderMaxDownloadSize checks that a small
// body is written to the destination file intact.
func TestDownloadToFile_AllowsResponseAtOrUnderMaxDownloadSize(t *testing.T) {
	content := []byte("small download, well under the cap")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(content); err != nil {
			t.Errorf("write body: %v", err)
		}
	}))
	defer srv.Close()

	destPath := filepath.Join(t.TempDir(), "small.bin")
	if err := downloadToFile(context.Background(), srv.Client(), srv.URL, destPath, acceptAnyURL); err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
}

// TestDownloadToFile_ValidatesRedirectTargets checks that downloadToFile
// runs validate on each redirect target and refuses a redirect it rejects
// without writing the destination file, while an accepted redirect is
// followed.
func TestDownloadToFile_ValidatesRedirectTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset":
			http.Redirect(w, r, "/blocked/asset", http.StatusFound)
		case "/allowed":
			http.Redirect(w, r, "/final", http.StatusFound)
		default:
			if _, err := w.Write([]byte("payload")); err != nil {
				t.Errorf("write payload: %v", err)
			}
		}
	}))
	defer srv.Close()
	rejectBlocked := func(rawURL string) error {
		if strings.Contains(rawURL, "/blocked/") {
			return fmt.Errorf("refusing to download from %q", rawURL)
		}
		return nil
	}

	destPath := filepath.Join(t.TempDir(), "asset.bin")
	err := downloadToFile(context.Background(), srv.Client(), srv.URL+"/asset", destPath, rejectBlocked)
	if err == nil || !strings.Contains(err.Error(), "refusing to download") {
		t.Fatalf("err = %v, want the redirect target to be refused", err)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("destination file exists after a refused redirect (stat err = %v)", statErr)
	}

	if err := downloadToFile(context.Background(), srv.Client(), srv.URL+"/allowed", destPath, rejectBlocked); err != nil {
		t.Fatalf("downloadToFile through an accepted redirect: %v", err)
	}
	if got, _ := os.ReadFile(destPath); string(got) != "payload" {
		t.Errorf("downloaded content = %q, want %q", got, "payload")
	}
}

// TestDownloadToFile_StopsAfterMaxRedirects checks that downloadToFile gives
// up on a redirect loop after maxRedirects hops, even when every target is
// accepted, and leaves no destination file.
func TestDownloadToFile_StopsAfterMaxRedirects(t *testing.T) {
	var hops atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hops.Add(1)
		http.Redirect(w, r, fmt.Sprintf("/hop/%d", n), http.StatusFound)
	}))
	defer srv.Close()

	destPath := filepath.Join(t.TempDir(), "asset.bin")
	err := downloadToFile(context.Background(), srv.Client(), srv.URL+"/start", destPath, acceptAnyURL)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("stopped after %d redirects", maxRedirects)) {
		t.Fatalf("err = %v, want the redirect limit error", err)
	}
	if got := hops.Load(); got != maxRedirects {
		t.Errorf("server saw %d requests, want %d", got, maxRedirects)
	}
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Errorf("destination file exists after the redirect limit (stat err = %v)", statErr)
	}
}

// TestFetchLatestRelease_LimitsResponseBodySize checks that
// fetchLatestRelease caps the response it decodes. The handler never closes
// the JSON object it starts writing, so without the cap decoding would hang
// or consume unbounded memory; with it, the decoder returns an error once
// the limit is reached.
func TestFetchLatestRelease_LimitsResponseBodySize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"tag_name":"v1.0.0","assets":[],"padding":"`)
		buf := make([]byte, 1<<20) // 1MB chunks
		for i := range buf {
			buf[i] = 'x'
		}
		var written int64
		for written < maxReleaseMetadataSize+1<<20 {
			n, err := w.Write(buf)
			written += int64(n)
			if err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	_, err := fetchLatestRelease(context.Background(), srv.Client(), srv.URL, "v1.0.0")
	if err == nil {
		t.Fatal("expected an error for a release response exceeding maxReleaseMetadataSize")
	}
}

// TestFetchLatestRelease_AllowsResponseAtOrUnderMaxReleaseMetadataSize checks
// that a normal release response is decoded.
func TestFetchLatestRelease_AllowsResponseAtOrUnderMaxReleaseMetadataSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(githubRelease{
			TagName: "v1.2.3",
			Assets:  []githubAsset{{Name: "sync82_linux_amd64", BrowserDownloadURL: "http://example.invalid/asset"}},
		}); err != nil {
			t.Errorf("encode release: %v", err)
		}
	}))
	defer srv.Close()

	release, err := fetchLatestRelease(context.Background(), srv.Client(), srv.URL, "v1.0.0")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if release.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v1.2.3")
	}
}

// TestRunSelfUpdate_StagesNextToBinaryByDefault checks that, without
// Deps.TempDir, the new binary is downloaded next to the installed one
// rather than into os.TempDir(), so the final rename never crosses
// filesystems.
func TestRunSelfUpdate_StagesNextToBinaryByDefault(t *testing.T) {
	assetName := "sync82_linux_amd64"
	newBinaryContent := []byte("new binary bytes")
	srv := fakeReleaseServer(t, "v2.0.0", newBinaryContent, assetName, sha256Hex(newBinaryContent))

	exeDir := t.TempDir()
	exePath := filepath.Join(exeDir, "sync82")
	if err := os.WriteFile(exePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stagedIn string
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return exePath, nil },
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion: func(_ context.Context, path string) (string, error) {
			stagedIn = filepath.Dir(filepath.Dir(path))
			return "v2.0.0", nil
		},
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	// Compared by identity, not by string: the code resolves the binary's
	// path, which may name the same directory differently (macOS /var vs
	// /private/var, Windows 8.3 short names).
	stagedInfo, err := os.Stat(stagedIn)
	if err != nil {
		t.Fatal(err)
	}
	exeDirInfo, err := os.Stat(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(stagedInfo, exeDirInfo) {
		t.Errorf("staged in %q, want the binary's directory %q", stagedIn, exeDir)
	}
	entries, err := os.ReadDir(exeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("binary directory holds %v, want only sync82 and sync82.bak", entries)
	}
}

// TestRunSelfUpdate_RefusesBinaryFailingSmokeTest checks that a download
// that doesn't report the release version leaves the installed binary
// untouched.
func TestRunSelfUpdate_RefusesBinaryFailingSmokeTest(t *testing.T) {
	assetName := "sync82_linux_amd64"
	content := []byte("broken binary")
	srv := fakeReleaseServer(t, "v2.0.0", content, assetName, sha256Hex(content))
	exePath := filepath.Join(t.TempDir(), "sync82")
	if err := os.WriteFile(exePath, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return exePath, nil },
		TempDir:        t.TempDir(),
		ValidateURL:    acceptAnyURL,
		ValidateAsset:  acceptAnyAsset,
		RunVersion:     reportVersion("dev"),
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if data, _ := os.ReadFile(exePath); string(data) != "old" {
		t.Fatalf("binary = %q, want it untouched", data)
	}
}

// TestRunSelfUpdate_RejectsUntrustedDownloadURL checks that asset URLs
// outside GitHub are refused before anything is downloaded.
func TestRunSelfUpdate_RejectsUntrustedDownloadURL(t *testing.T) {
	srv := fakeReleaseServer(t, "v2.0.0", []byte("x"), "sync82_linux_amd64", "irrelevant")
	deps := Deps{
		Version:        "v1.0.0",
		GOOS:           "linux",
		GOARCH:         "amd64",
		APIURL:         srv.URL + "/release",
		HTTPClient:     srv.Client(),
		ExecutablePath: func() (string, error) { return "/should/not/be/used", nil },
		TempDir:        t.TempDir(),
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to download") {
		t.Errorf("stderr = %q, want it to refuse the URL", stderr.String())
	}
}

// TestRunSelfUpdate_RollbackSwapsWithBackup checks that --rollback restores the
// backup and keeps the replaced binary as the new backup.
func TestRunSelfUpdate_RollbackSwapsWithBackup(t *testing.T) {
	exePath := filepath.Join(t.TempDir(), "sync82")
	if err := os.WriteFile(exePath, []byte("v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exePath+".bak", []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	deps := Deps{
		ExecutablePath: func() (string, error) { return exePath, nil },
		RunVersion:     reportVersion("v1.0.0"),
	}

	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--rollback"}, deps, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr.String())
	}
	if data, _ := os.ReadFile(exePath); string(data) != "v1" {
		t.Errorf("binary = %q, want the backup restored", data)
	}
	if data, _ := os.ReadFile(exePath + ".bak"); string(data) != "v2" {
		t.Errorf("backup = %q, want the replaced binary kept", data)
	}
	if !strings.Contains(stdout.String(), "Rolled back to v1.0.0") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

// TestRunSelfUpdate_RollbackWithoutBackupFails checks that --rollback without
// a backup exits with code 1 and leaves the binary untouched.
func TestRunSelfUpdate_RollbackWithoutBackupFails(t *testing.T) {
	exePath := filepath.Join(t.TempDir(), "sync82")
	if err := os.WriteFile(exePath, []byte("v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	deps := Deps{ExecutablePath: func() (string, error) { return exePath, nil }, RunVersion: reportVersion("v1.0.0")}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--rollback"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if data, _ := os.ReadFile(exePath); string(data) != "v2" {
		t.Errorf("binary = %q, want it untouched", data)
	}
}

// TestRunSelfUpdate_RejectsUnknownOption checks that an unknown option is a
// usage error: exit code 2 and a pointer to --help.
func TestRunSelfUpdate_RejectsUnknownOption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--chek"}, Deps{Version: "v1.0.0"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Run 'sync82 self-update --help' for usage.") {
		t.Errorf("stderr = %q, want the --help pointer", stderr.String())
	}
}

// TestRunSelfUpdate_RejectsConflictingOrRepeatedOptions checks that
// --rollback can't be combined with --check or --yes, and that an option
// given twice is refused, before anything is checked or changed: the
// binary and its backup stay as they are.
func TestRunSelfUpdate_RejectsConflictingOrRepeatedOptions(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "sync82")
	for path, content := range map[string]string{exe: "current", backupPath(exe): "previous"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deps := Deps{
		Version:        "v1.0.0",
		ExecutablePath: func() (string, error) { return exe, nil },
		RunVersion:     reportVersion("v0.9.0"),
	}
	for _, args := range [][]string{
		{"--check", "--rollback"}, {"--rollback", "--yes"}, {"--rollback", "-y"},
		{"--rollback", "--rollback"}, {"--check", "--check"}, {"--yes", "-y"},
	} {
		var stdout, stderr bytes.Buffer
		if code := RunSelfUpdate(context.Background(), args, deps, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("%q: exit code = %d, want 2; stdout=%q", args, code, stdout.String())
		}
	}
	if got, _ := os.ReadFile(exe); string(got) != "current" {
		t.Errorf("binary = %q after refused options, want it unchanged", got)
	}
}

// TestFetchLatestRelease_ReportsRateLimit checks that an exhausted rate limit
// is reported as an error and that the User-Agent carries the version.
func TestFetchLatestRelease_ReportsRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != "sync82/v1.0.0" {
			t.Errorf("User-Agent = %q, want %q", ua, "sync82/v1.0.0")
		}
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := fetchLatestRelease(context.Background(), srv.Client(), srv.URL, "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("err = %v, want a rate limit error", err)
	}
}

// TestHelperProcess_Sleep is not a real test: run as a child process with
// SYNC82_TEST_SLEEP=1, it keeps running so its executable stays in use.
func TestHelperProcess_Sleep(t *testing.T) {
	if os.Getenv("SYNC82_TEST_SLEEP") != "1" {
		t.Skip("helper process")
	}
	time.Sleep(time.Minute)
}

// TestReplaceWithBackup_BackupStillRunning starts a copy of the test
// binary from the backup path, as an MCP client still running the version
// replaced by the previous update would, and checks that the next update
// still succeeds. On Windows the running backup is moved aside to
// "<bak>.old-<n>", which a later update removes.
func TestReplaceWithBackup_BackupStillRunning(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "sync82.exe")
	newPath := filepath.Join(dir, "sync82-new.exe")
	bak := backupPath(currentPath)
	for path, content := range map[string][]byte{currentPath: []byte("old"), newPath: []byte("new"), bak: data} {
		if err := os.WriteFile(path, content, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(bak, "-test.run=^TestHelperProcess_Sleep$")
	cmd.Env = append(os.Environ(), "SYNC82_TEST_SLEEP=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	t.Cleanup(stop)

	if err := replaceWithBackup(currentPath, newPath); err != nil {
		t.Fatalf("replaceWithBackup with a running backup: %v", err)
	}
	if got, _ := os.ReadFile(currentPath); string(got) != "new" {
		t.Errorf("current = %q, want new", got)
	}
	if got, _ := os.ReadFile(bak); string(got) != "old" {
		t.Errorf("backup = %q, want old", got)
	}
	parked, _ := filepath.Glob(bak + ".old-*")
	if runtime.GOOS == "windows" && len(parked) != 1 {
		t.Fatalf("parked backups = %v, want the running one moved aside", parked)
	}

	stop()
	if err := clearBackup(bak); err != nil {
		t.Fatal(err)
	}
	if parked, _ := filepath.Glob(bak + ".old-*"); len(parked) != 0 {
		t.Errorf("leftover backups = %v, want them removed once no longer running", parked)
	}
}

// TestClearBackup_InUseIsReported checks that a backup that can be neither
// removed nor moved aside (here, in a read-only directory) is reported as
// errBackupInUse, not as a missing privilege.
func TestClearBackup_InUseIsReported(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs directory permissions that bind the current user")
	}
	dir := t.TempDir()
	bak := filepath.Join(dir, "sync82.bak")
	if err := os.WriteFile(bak, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	err := clearBackup(bak)
	if !errors.Is(err, errBackupInUse) || errors.Is(err, fs.ErrPermission) {
		t.Fatalf("clearBackup = %v, want errBackupInUse without a permission error", err)
	}
	var stderr strings.Builder
	reportReplaceError(&stderr, "/usr/local/bin/sync82", err)
	if !strings.Contains(stderr.String(), "restart your MCP clients") {
		t.Errorf("reported %q, want the advice to restart the MCP clients", stderr.String())
	}
}

// TestValidateReleaseAsset checks that only an asset of this repository's
// release of the given tag is accepted.
func TestValidateReleaseAsset(t *testing.T) {
	const tag = "v1.2.0"
	if err := validateReleaseAsset("https://github.com/oito2/mcp-sync82/releases/download/v1.2.0/sync82_linux_amd64", tag); err != nil {
		t.Errorf("own asset rejected: %v", err)
	}
	for _, bad := range []string{
		"https://github.com/oito2/mcp-sync82/releases/download/v1.1.0/sync82_linux_amd64",
		"https://github.com/someone/fork/releases/download/v1.2.0/sync82_linux_amd64",
		"https://github.com/oito2/mcp-sync82/releases/download/v1.2.0/",
		"https://github.com/oito2/mcp-sync82/releases/download/v1.2.0/../../../evil/x",
		"https://github.com/oito2/mcp-sync82/releases/download/v1.2.0/x?y=1",
		"https://objects.githubusercontent.com/github-production-release-asset/1",
		"http://github.com/oito2/mcp-sync82/releases/download/v1.2.0/sync82_linux_amd64",
	} {
		if err := validateReleaseAsset(bad, tag); err == nil {
			t.Errorf("validateReleaseAsset(%q) = nil, want an error", bad)
		}
	}
}

// TestDownloadToFile_RefusesARedirectToAnotherHost checks that a redirect
// target is held to the same rule as the first URL.
func TestDownloadToFile_RefusesARedirectToAnotherHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://raw.githubusercontent.com/someone/repo/main/sync82", http.StatusFound)
	}))
	defer srv.Close()
	validate := func(rawURL string) error {
		if strings.HasPrefix(rawURL, srv.URL) {
			return nil
		}
		return validateAssetURL(rawURL)
	}
	err := downloadToFile(context.Background(), srv.Client(), srv.URL+"/asset", filepath.Join(t.TempDir(), "out"), validate)
	if err == nil || !strings.Contains(err.Error(), "raw.githubusercontent.com") {
		t.Fatalf("downloadToFile = %v, want the redirect to raw.githubusercontent.com refused", err)
	}
}

// TestRunSelfUpdate_RollbackRefusesABackupThatDoesNotRun checks that a
// backup failing its --version check is not swapped in: the binary and
// the backup stay as they are.
func TestRunSelfUpdate_RollbackRefusesABackupThatDoesNotRun(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "sync82")
	for path, content := range map[string]string{exe: "current", backupPath(exe): "previous"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deps := Deps{
		Version:        "v1.0.0",
		ExecutablePath: func() (string, error) { return exe, nil },
		RunVersion:     func(context.Context, string) (string, error) { return "", errors.New("exec format error") },
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--rollback"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not run") {
		t.Errorf("stderr = %q, want the failed check reported", stderr.String())
	}
	current, _ := os.ReadFile(exe)
	previous, _ := os.ReadFile(backupPath(exe))
	if string(current) != "current" || string(previous) != "previous" {
		t.Errorf("binary = %q, backup = %q; want both unchanged", current, previous)
	}
}

// TestBinaryMode keeps the current binary's permission bits, with the
// owner's execute bit set, and falls back to 0755.
func TestBinaryMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not kept on Windows")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "sync82")
	if err := os.WriteFile(exe, []byte("x"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(exe, 0o750); err != nil {
		t.Fatal(err)
	}
	if got := binaryMode(exe); got != 0o750 {
		t.Errorf("binaryMode = %o, want 750", got)
	}
	if got := binaryMode(filepath.Join(dir, "missing")); got != 0o755 {
		t.Errorf("binaryMode of a missing binary = %o, want 755", got)
	}
}

// TestClearBackup_RemovesLeftoversInAPathWithGlobCharacters checks that
// leftover "<bak>.old-*" files are removed even when the install directory
// has characters a glob would read as a pattern.
func TestClearBackup_RemovesLeftoversInAPathWithGlobCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tools[2]")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bak := filepath.Join(dir, "sync82.bak")
	left := bak + ".old-123"
	for _, p := range []string{bak, left} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := clearBackup(bak); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Errorf("leftover %s still exists (stat err = %v)", left, err)
	}
}
