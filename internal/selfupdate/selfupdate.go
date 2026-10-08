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
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oito2/mcp-sync82/internal/fsutil"
	"github.com/oito2/mcp-sync82/internal/prompt"
)

// Repo is the GitHub repository self-update fetches releases from.
const Repo = "oito2/mcp-sync82"

// DefaultAPIURL is the GitHub Releases endpoint that returns the latest
// release of Repo. It is the default value for Deps.APIURL.
const DefaultAPIURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

// Deps bundles everything RunSelfUpdate needs from its environment, so each
// piece can be replaced to avoid real network access, the real executable
// path and writes outside a chosen directory.
type Deps struct {
	// Version is the currently running binary's version; GOOS and GOARCH
	// select the release asset to download.
	Version string
	GOOS    string
	GOARCH  string

	// APIURL is the releases endpoint queried for the latest release, and
	// HTTPClient performs every HTTP request.
	APIURL     string
	HTTPClient *http.Client

	// ExecutablePath resolves the path of the currently running binary.
	ExecutablePath func() (string, error)

	// TempDir is the directory the update is staged in. Defaults to the
	// running binary's own directory, so the final rename never crosses
	// filesystems.
	TempDir string

	// ValidateURL checks a download URL, and every redirect target, before
	// it is fetched. Defaults to validateAssetURL (HTTPS from github.com or
	// GitHub's release asset hosts only).
	ValidateURL func(rawURL string) error

	// ValidateAsset checks the download URL of a release asset against the
	// release's tag before it is fetched. Defaults to validateReleaseAsset
	// (the repository's own release of that tag only).
	ValidateAsset func(rawURL, tag string) error

	// RunVersion runs a binary with --version and returns its trimmed
	// output. Defaults to runVersion.
	RunVersion func(ctx context.Context, path string) (string, error)
}

// Timeouts for the release metadata request, the binary download and the
// "--version" check of a candidate binary.
const (
	apiTimeout       = 30 * time.Second
	downloadTimeout  = 5 * time.Minute
	smokeTestTimeout = 10 * time.Second
)

// RunSelfUpdate implements "sync82 self-update [--check] [--yes|-y]" and
// "sync82 self-update --rollback". args are the options after the
// subcommand name; deps supplies the environment; stdin is read for the
// confirmation answer; progress goes to stdout and errors to stderr.
//
// It returns the process exit code: 0 = up to date / update available
// with --check / aborted / updated / rolled back, 1 = error (development
// build, closed stdin at the confirmation prompt, failed check, download,
// verification or replacement), 2 = usage error (unknown option).
//
// An update downloads the new binary into a temporary directory next to
// the running one, checks it against the release's checksums.txt, runs it
// with --version and requires the release tag back, then swaps it in,
// keeping the replaced binary as "<binary>.bak" — restored automatically
// if the swap fails, and by hand with --rollback.
func RunSelfUpdate(ctx context.Context, args []string, deps Deps, stdin io.Reader, stdout, stderr io.Writer) int {
	seen := map[string]bool{}
	for _, a := range args {
		name := a
		if a == "-y" {
			name = "--yes"
		}
		switch name {
		case "--check", "--yes", "--rollback":
		default:
			fmt.Fprintf(stderr, "Error: unknown self-update option %q (usage: sync82 self-update [--check] [--yes] | --rollback)\nRun 'sync82 self-update --help' for usage.\n", a)
			return 2
		}
		if seen[name] {
			fmt.Fprintf(stderr, "Error: %s given twice\nRun 'sync82 self-update --help' for usage.\n", a)
			return 2
		}
		seen[name] = true
	}
	checkOnly, skipConfirm, rollback := seen["--check"], seen["--yes"], seen["--rollback"]
	if rollback && (checkOnly || skipConfirm) {
		fmt.Fprintln(stderr, "Error: --rollback can't be combined with --check or --yes\nRun 'sync82 self-update --help' for usage.")
		return 2
	}
	if deps.ValidateURL == nil {
		deps.ValidateURL = validateAssetURL
	}
	if deps.ValidateAsset == nil {
		deps.ValidateAsset = validateReleaseAsset
	}
	if deps.RunVersion == nil {
		deps.RunVersion = runVersion
	}

	if rollback {
		return runRollback(ctx, deps, stdout, stderr)
	}

	if deps.Version == "dev" {
		fmt.Fprintln(stderr, "Cannot self-update a development build (no version embedded).")
		return 1
	}

	fetchCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()
	release, err := fetchLatestRelease(fetchCtx, deps.HTTPClient, deps.APIURL, deps.Version)
	if err != nil {
		fmt.Fprintf(stderr, "Could not check for updates: %v\n", err)
		return 1
	}

	latest := release.TagName
	if !isValidReleaseTag(latest) {
		// The tag comes from the API response and is printed and compared as
		// a version, so anything other than a plain vMAJOR.MINOR.PATCH tag
		// is rejected.
		fmt.Fprintf(stderr, "Unexpected release tag format: %q\n", latest)
		return 1
	}
	if !isNewerVersion(latest, deps.Version) {
		fmt.Fprintf(stdout, "Already up to date (%s).\n", deps.Version)
		return 0
	}

	fmt.Fprintf(stdout, "Update available: %s → %s\n", deps.Version, latest)
	if checkOnly {
		return 0
	}

	if !skipConfirm {
		yes, err := prompt.AskYes(ctx, bufio.NewReader(stdin), stdout, "Update now? [y/N] ")
		switch {
		case errors.Is(err, context.Canceled):
			fmt.Fprintln(stderr, "Interrupted; nothing was changed.")
			return 1
		case err != nil:
			fmt.Fprintf(stderr, "Error: %v; pass --yes to update without asking.\n", err)
			return 1
		case !yes:
			fmt.Fprintln(stdout, "Aborted.")
			return 0
		}
	}

	name := AssetName(deps.GOOS, deps.GOARCH)
	assetURL, ok := findAssetURL(release, name)
	if !ok {
		fmt.Fprintf(stderr, "No release asset found for %s/%s.\n", deps.GOOS, deps.GOARCH)
		return 1
	}
	checksumsURL, ok := findAssetURL(release, "checksums.txt")
	if !ok {
		fmt.Fprintln(stderr, "No checksums.txt found in the release assets.")
		return 1
	}
	for _, u := range []string{assetURL, checksumsURL} {
		err := deps.ValidateAsset(u, latest)
		if err == nil {
			err = deps.ValidateURL(u)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	currentExePath, err := deps.ExecutablePath()
	if err != nil {
		fmt.Fprintf(stderr, "Could not resolve the current executable path: %v\n", err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(currentExePath); err == nil {
		currentExePath = resolved
	}

	stageDir := deps.TempDir
	if stageDir == "" {
		stageDir = filepath.Dir(currentExePath)
	}
	workDir, err := os.MkdirTemp(stageDir, ".sync82-update-*")
	if err != nil {
		reportReplaceError(stderr, currentExePath, err)
		return 1
	}
	defer os.RemoveAll(workDir)

	newBinaryPath := filepath.Join(workDir, name)
	downloadCtx, downloadCancel := context.WithTimeout(ctx, downloadTimeout)
	defer downloadCancel()
	if err := downloadToFile(downloadCtx, deps.HTTPClient, assetURL, newBinaryPath, deps.ValidateURL); err != nil {
		fmt.Fprintf(stderr, "Download failed: %v\n", err)
		return 1
	}

	checksumsPath := filepath.Join(workDir, "checksums.txt")
	checksumsCtx, checksumsCancel := context.WithTimeout(ctx, apiTimeout)
	defer checksumsCancel()
	if err := downloadToFile(checksumsCtx, deps.HTTPClient, checksumsURL, checksumsPath, deps.ValidateURL); err != nil {
		fmt.Fprintf(stderr, "Download failed: %v\n", err)
		return 1
	}

	expectedSum, err := findChecksum(checksumsPath, name)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	actualSum, err := fsutil.SHA256File(newBinaryPath)
	if err != nil {
		fmt.Fprintf(stderr, "Could not verify checksum: %v\n", err)
		return 1
	}
	if actualSum != expectedSum {
		fmt.Fprintln(stderr, "Checksum mismatch — aborting. The download may be corrupted or tampered with.")
		return 1
	}

	if err := os.Chmod(newBinaryPath, binaryMode(currentExePath)); err != nil {
		fmt.Fprintf(stderr, "Could not make the new binary executable: %v\n", err)
		return 1
	}
	if err := smokeTest(ctx, deps.RunVersion, newBinaryPath, latest); err != nil {
		fmt.Fprintf(stderr, "The downloaded binary failed its check, nothing was changed: %v\n", err)
		return 1
	}

	if err := replaceWithBackup(currentExePath, newBinaryPath); err != nil {
		reportReplaceError(stderr, currentExePath, err)
		return 1
	}

	fmt.Fprintf(stdout, "Updated to %s (previous version kept at %s; undo with \"sync82 self-update --rollback\"). Restart sync82 (or your MCP client) to use the new version.\n", latest, backupPath(currentExePath))
	return 0
}

// runRollback swaps the running binary with its "<binary>.bak" backup,
// after checking the backup still runs. Running it again swaps them back.
// It returns the exit code: 0 on success, 1 when the executable path cannot
// be resolved, no backup exists, the backup does not run or the swap fails.
func runRollback(ctx context.Context, deps Deps, stdout, stderr io.Writer) int {
	currentExePath, err := deps.ExecutablePath()
	if err != nil {
		fmt.Fprintf(stderr, "Could not resolve the current executable path: %v\n", err)
		return 1
	}
	if resolved, err := filepath.EvalSymlinks(currentExePath); err == nil {
		currentExePath = resolved
	}
	bak := backupPath(currentExePath)
	if _, err := os.Stat(bak); err != nil {
		fmt.Fprintf(stderr, "No previous version to roll back to: %s not found.\n", bak)
		return 1
	}
	previous, err := runWithTimeout(ctx, deps.RunVersion, bak)
	if err != nil {
		fmt.Fprintf(stderr, "The backup at %s does not run (%v); nothing was changed.\n", bak, err)
		return 1
	}

	staged := currentExePath + ".rollback"
	if err := fsutil.Rename(bak, staged); err != nil {
		reportReplaceError(stderr, currentExePath, err)
		return 1
	}
	if err := replaceWithBackup(currentExePath, staged); err != nil {
		_ = fsutil.Rename(staged, bak)
		reportReplaceError(stderr, currentExePath, err)
		return 1
	}
	fmt.Fprintf(stdout, "Rolled back to %s (the replaced version is kept at %s). Restart sync82 (or your MCP client) to use it.\n", previous, bak)
	return 0
}

// binaryMode returns the permission bits for a new binary replacing the one
// at currentPath: the current binary's own bits, with the owner's execute
// bit always set, so an update neither widens nor narrows who can run it;
// 0755 when the current binary can't be inspected.
func binaryMode(currentPath string) os.FileMode {
	info, err := os.Stat(currentPath)
	if err != nil {
		return 0o755
	}
	return info.Mode().Perm() | 0o100
}

// errBackupInUse reports a backup that can be neither removed nor moved
// aside, because a running process still holds it.
var errBackupInUse = errors.New("the previous version is still in use")

// clearBackup removes the backup at bak, if any, so the current binary can
// take its place. On Windows a backup that is still running (an MCP client
// started it before the previous update) can't be removed or replaced, but
// it can be renamed: it is then moved aside to "<bak>.old-<n>", which a
// later update removes once it no longer runs. Leftover "<bak>.old-*"
// files are removed on a best-effort basis. It returns an error wrapping
// errBackupInUse, telling the user to restart their MCP clients, when bak
// can be neither removed nor moved aside.
func clearBackup(bak string) error {
	// Listed by name prefix, not with a glob, so "[", "*" or "?" in the
	// install path are taken literally.
	dir, prefix := filepath.Dir(bak), filepath.Base(bak)+".old-"
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	err := os.Remove(bak)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	parked := fmt.Sprintf("%s.old-%d", bak, time.Now().UnixNano())
	if rerr := fsutil.Rename(bak, parked); rerr != nil {
		// err is reported with %v: wrapping it would make a Windows
		// access-denied error read as a missing privilege.
		return fmt.Errorf("%w: %s can't be removed or moved aside (%v); restart your MCP clients, then try again", errBackupInUse, bak, err)
	}
	return nil
}

// backupPath returns the path where the binary at exePath is kept when an
// update or rollback replaces it.
func backupPath(exePath string) string {
	return exePath + ".bak"
}

// replaceWithBackup moves currentPath to its backup path and newPath into
// its place, restoring currentPath if the second move fails. Both paths
// are in the same directory, so each move is an atomic rename. A running
// binary can be renamed on every supported OS, including Windows; each
// move goes through fsutil.Rename, which retries a move another process
// briefly blocks on Windows. An existing backup is cleared first by
// clearBackup, and the directory is synced once both moves are done, so
// the swap survives a crash. It returns an error when clearing the backup
// or either move fails; the error also reports when the restore itself
// failed.
func replaceWithBackup(currentPath, newPath string) error {
	bak := backupPath(currentPath)
	if err := clearBackup(bak); err != nil {
		return err
	}
	if err := fsutil.Rename(currentPath, bak); err != nil {
		return fmt.Errorf("move the current binary to %s: %w", bak, err)
	}
	if err := fsutil.Rename(newPath, currentPath); err != nil {
		if rerr := fsutil.Rename(bak, currentPath); rerr != nil {
			return fmt.Errorf("%w (restoring the previous binary also failed: %v; it is at %s)", err, rerr, bak)
		}
		return err
	}
	fsutil.SyncDir(filepath.Dir(currentPath))
	return nil
}

// reportReplaceError writes to stderr why currentExePath could not be
// replaced because of err, with a hint when the cause is missing write
// permission on its directory.
func reportReplaceError(stderr io.Writer, currentExePath string, err error) {
	if errors.Is(err, fs.ErrPermission) {
		fmt.Fprintf(stderr, "Permission denied updating %s — re-run with elevated privileges, or reinstall via `go install github.com/%s/cmd/sync82@latest`.\n", currentExePath, Repo)
		return
	}
	fmt.Fprintf(stderr, "Could not replace the running binary: %v\n", err)
}

// smokeTest runs the binary at path with --version and requires it to
// report want, the release tag being installed. It returns an error when
// the binary cannot be run or reports another version.
func smokeTest(ctx context.Context, run func(context.Context, string) (string, error), path, want string) error {
	got, err := runWithTimeout(ctx, run, path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("it reports version %q, expected %q", got, want)
	}
	return nil
}

// runWithTimeout calls run on path with ctx limited to smokeTestTimeout and
// returns its result.
func runWithTimeout(ctx context.Context, run func(context.Context, string) (string, error), path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, smokeTestTimeout)
	defer cancel()
	return run(ctx, path)
}

// runVersion runs the binary at path with --version and returns its
// trimmed standard output. It returns an error if the binary cannot be run
// or exits non-zero.
func runVersion(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("run %s --version: %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// assetHosts are the hosts a release download may be served from:
// github.com, which answers the asset URL, and the hosts it redirects
// release assets to. Other *.githubusercontent.com hosts serve user
// content (raw files, gists) and are refused.
var assetHosts = map[string]bool{
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// validateAssetURL accepts only HTTPS downloads from one of assetHosts,
// without credentials or an explicit port. It returns an error for an
// unparsable URL or any other scheme or host.
func validateAssetURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid download URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" || !assetHosts[u.Hostname()] || u.Port() != "" || u.User != nil {
		return fmt.Errorf("refusing to download from %q: only HTTPS URLs on GitHub's release hosts are accepted", rawURL)
	}
	return nil
}

// validateReleaseAsset accepts only the download URL of an asset of this
// repository's release tag: https://github.com/<Repo>/releases/download/<tag>/<name>,
// with no further path segment, query or fragment. It returns an error for
// any other URL, so a release response can't point the download anywhere
// else.
func validateReleaseAsset(rawURL, tag string) error {
	prefix := "https://github.com/" + Repo + "/releases/download/" + tag + "/"
	name, ok := strings.CutPrefix(rawURL, prefix)
	if !ok || name == "" || strings.ContainsAny(name, "/?#\\") {
		return fmt.Errorf("refusing to download %q: it is not an asset of the %s release of %s", rawURL, tag, Repo)
	}
	return nil
}

// githubAsset is one downloadable file attached to a GitHub release.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// githubRelease is the part of the GitHub release API response that
// self-update reads: the tag and its assets.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// maxReleaseMetadataSize caps how much of the release JSON response is
// decoded, so an unexpectedly large or malformed response (a misconfigured
// proxy or wrong endpoint) cannot consume unbounded memory.
const maxReleaseMetadataSize = 5 * 1024 * 1024 // 5MB

// fetchLatestRelease reads the latest release from apiURL using client,
// identifying itself as sync82/<version>. It returns an error for a
// transport failure, a rate-limit response, a non-200 status or an
// undecodable body.
func fetchLatestRelease(ctx context.Context, client *http.Client, apiURL, version string) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sync82/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests) && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return githubRelease{}, fmt.Errorf("GitHub API rate limit reached; try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, apiURL)
	}

	var release githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseMetadataSize)).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode release response: %w", err)
	}
	return release, nil
}

// findAssetURL returns the download URL of the release asset called name,
// and whether such an asset exists.
func findAssetURL(release githubRelease, name string) (string, bool) {
	for _, a := range release.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

// releaseTagPattern is the only accepted shape of a release tag:
// vMAJOR.MINOR.PATCH with no suffix. It is stricter than parseVersion,
// which accepts pre-release and build suffixes, because the whole tag
// string is external data and is validated before any use.
var releaseTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// isValidReleaseTag reports whether tag matches releaseTagPattern.
func isValidReleaseTag(tag string) bool {
	return releaseTagPattern.MatchString(tag)
}

// ReleasePlatforms returns the GOOS/GOARCH pairs every release is built
// for, each published as the asset AssetName names. Each call returns a
// new slice.
func ReleasePlatforms() [][2]string {
	return [][2]string{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}
}

// AssetName returns the release asset name of the sync82 binary for
// goos/goarch: sync82_<goos>_<goarch>, with an ".exe" suffix on Windows.
// The name carries no version, so the newest release is always reachable
// at the same releases/latest/download URL.
func AssetName(goos, goarch string) string {
	name := "sync82_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// semver is a parsed "vMAJOR.MINOR.PATCH[-pre-release][+build]" version.
// Build metadata is discarded; pre holds the dot-separated pre-release
// identifiers, empty for a normal release.
type semver struct {
	major, minor, patch int
	pre                 []string
}

// parseVersion parses v ("v1.2.3", "1.2.3", "v1.2.3-rc.1", "v1.2.3+build")
// into a semver. ok is false when the core is not three non-negative
// integers or a pre-release identifier is empty.
func parseVersion(v string) (sv semver, ok bool) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.Index(v, "+"); i != -1 {
		v = v[:i]
	}
	if i := strings.Index(v, "-"); i != -1 {
		sv.pre = strings.Split(v[i+1:], ".")
		for _, id := range sv.pre {
			if id == "" {
				return semver{}, false
			}
		}
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	nums := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semver{}, false
		}
		nums[i] = n
	}
	sv.major, sv.minor, sv.patch = nums[0], nums[1], nums[2]
	return sv, true
}

// isNumericID reports whether id is a non-empty string of ASCII digits.
func isNumericID(id string) bool {
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return id != ""
}

// comparePreRelease compares two pre-release identifier lists by semver
// precedence and returns -1, 0 or 1. An empty list (a normal release) ranks
// above any pre-release. Identifiers compare left to right: numeric ones
// numerically, alphanumeric ones in ASCII order, numeric below
// alphanumeric, and a shorter list below a longer one when every shared
// identifier is equal.
func comparePreRelease(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] == b[i] {
			continue
		}
		aNum, bNum := isNumericID(a[i]), isNumericID(b[i])
		switch {
		case aNum && bNum:
			// Longer numbers (without leading zeros) are larger, so no
			// integer conversion is needed for arbitrarily long identifiers.
			an, bn := strings.TrimLeft(a[i], "0"), strings.TrimLeft(b[i], "0")
			if len(an) != len(bn) {
				return cmp.Compare(len(an), len(bn))
			}
			return strings.Compare(an, bn)
		case aNum:
			return -1
		case bNum:
			return 1
		default:
			return strings.Compare(a[i], b[i])
		}
	}
	return cmp.Compare(len(a), len(b))
}

// isNewerVersion reports whether latest is greater than current by semver
// precedence: major, minor and patch first, then the pre-release
// identifiers, so "v1.2.3" is newer than "v1.2.3-rc.1" and "rc.10" newer
// than "rc.2". Build metadata is ignored. If either version does not
// parse, it falls back to a simple inequality check, so an unexpected
// format is reported as an update rather than silently ignored.
func isNewerVersion(latest, current string) bool {
	l, lok := parseVersion(latest)
	c, cok := parseVersion(current)
	if !lok || !cok {
		return latest != current
	}
	if l.major != c.major {
		return l.major > c.major
	}
	if l.minor != c.minor {
		return l.minor > c.minor
	}
	if l.patch != c.patch {
		return l.patch > c.patch
	}
	return comparePreRelease(l.pre, c.pre) > 0
}

// maxDownloadSize caps how much downloadToFile writes to disk for a single
// asset, so an unexpectedly large response (a misrouted URL or a proxy
// streaming forever) cannot fill the disk.
const maxDownloadSize = 200 * 1024 * 1024 // 200MB

// maxRedirects is how many redirects downloadToFile follows before failing.
const maxRedirects = 10

// downloadToFile fetches url with client and writes the body to destPath,
// creating or truncating the file. Every redirect target is checked with
// validate before it is followed, so the final download host is held to the
// same rule as url itself. It returns an error for a transport failure, a
// rejected redirect target, more than maxRedirects redirects, a non-200
// status, a file error, or a body larger than maxDownloadSize.
func downloadToFile(ctx context.Context, client *http.Client, url, destPath string, validate func(rawURL string) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	checked := *client
	checked.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return validate(req.URL.String())
	}
	resp, err := checked.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	// The file is flushed to disk before it is closed, and a failure to
	// close it is reported: the binary is about to replace the running one.
	n, err := io.Copy(out, io.LimitReader(resp.Body, maxDownloadSize+1))
	if err == nil && n > maxDownloadSize {
		err = fmt.Errorf("response from %s exceeded the %d byte download limit", url, maxDownloadSize)
	}
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// findChecksum parses checksums.txt's "<hash>  <filename>" lines
// (sha256sum's output format) looking for assetName and returns its
// lowercase hash. A leading "*" (the binary-mode marker) and a leading "./"
// on the filename are ignored. It returns an error if the file cannot be
// read or has no entry for assetName.
func findChecksum(checksumsPath, assetName string) (string, error) {
	f, err := os.Open(checksumsPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		name = strings.TrimPrefix(name, "./")
		if name == assetName {
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no checksum entry found for %s", assetName)
}
