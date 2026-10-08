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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestMain runs the package's tests with an empty PATH, so the default
// LookCosign never finds a cosign installed on the machine running them.
func TestMain(m *testing.M) {
	if err := os.Setenv("PATH", ""); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// testBundle is the content the signed release server serves as the
// signature bundle.
const testBundle = `{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}`

// fakeSignedReleaseServer starts an httptest server like fakeReleaseServer,
// for tag and a binary assetName holding assetBytes with its real checksum,
// whose release also lists signatureBundleName, served as testBundle. The
// server is closed when the test ends.
func fakeSignedReleaseServer(t *testing.T, tag string, assetBytes []byte, assetName string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		release := githubRelease{TagName: tag, Assets: []githubAsset{
			{Name: assetName, BrowserDownloadURL: serverURL + "/assets/" + assetName},
			{Name: "checksums.txt", BrowserDownloadURL: serverURL + "/assets/checksums.txt"},
			{Name: signatureBundleName, BrowserDownloadURL: serverURL + "/assets/" + signatureBundleName},
		}}
		if err := json.NewEncoder(w).Encode(release); err != nil {
			t.Errorf("encode release: %v", err)
		}
	})
	mux.HandleFunc("/assets/"+assetName, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(assetBytes) })
	mux.HandleFunc("/assets/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", sha256Hex(assetBytes), assetName)
	})
	mux.HandleFunc("/assets/"+signatureBundleName, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(testBundle)) })
	srv := httptest.NewServer(mux)
	serverURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

// fakeCosign is a cosign stand-in for Deps: version is what "cosign
// version --json" reports, and verifyErr, when set, fails "verify-blob"
// with verifyOutput. calls records every verify-blob call's arguments, and
// bundle the content of the bundle file it was given.
type fakeCosign struct {
	version      string
	verifyErr    error
	verifyOutput string
	calls        [][]string
	bundle       string
}

// run implements Deps.RunCosign for f.
func (f *fakeCosign) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	switch args[0] {
	case "version":
		return fmt.Appendf(nil, `{"gitVersion": %q, "platform": "linux/amd64"}`, f.version), nil
	case "verify-blob":
		f.calls = append(f.calls, args)
		if i := slices.Index(args, "--bundle"); i >= 0 {
			raw, _ := os.ReadFile(args[i+1])
			f.bundle = string(raw)
		}
		if f.verifyErr != nil {
			return []byte(f.verifyOutput), f.verifyErr
		}
		return []byte("Verified OK"), nil
	}
	return nil, fmt.Errorf("unexpected cosign call %v", args)
}

// signedUpdateDeps returns Deps for an update from v1.0.0 to v2.0.0 through
// srv, replacing the binary at exePath, with cosign as the cosign stand-in
// (nil for none on PATH).
func signedUpdateDeps(t *testing.T, srv *httptest.Server, exePath string, cosign *fakeCosign) Deps {
	t.Helper()
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
		RunVersion:     reportVersion("v2.0.0"),
		LookCosign:     func() (string, error) { return "", errors.New("not found") },
	}
	if cosign != nil {
		deps.LookCosign = func() (string, error) { return "/fake/cosign", nil }
		deps.RunCosign = cosign.run
	}
	return deps
}

// writeOldBinary writes "old" as the current binary in a new temporary
// directory and returns its path.
func writeOldBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sync82")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunSelfUpdate_VerifiesTheSignature checks that, with cosign v3 found,
// the update runs "cosign verify-blob" on checksums.txt with the downloaded
// bundle, the release workflow's identity for exactly the new tag and the
// GitHub Actions issuer, reports it, and then updates.
func TestRunSelfUpdate_VerifiesTheSignature(t *testing.T) {
	srv := fakeSignedReleaseServer(t, "v2.0.0", []byte("new"), "sync82_linux_amd64")
	exePath := writeOldBinary(t)
	cosign := &fakeCosign{version: "v3.0.6"}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--yes", "--require-signature"}, signedUpdateDeps(t, srv, exePath, cosign), strings.NewReader(""), &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Signature verified (cosign).") {
		t.Fatalf("exit code = %d, stdout = %q, stderr = %q; want a verified update", code, stdout.String(), stderr.String())
	}
	if len(cosign.calls) != 1 {
		t.Fatalf("verify-blob calls = %v, want one", cosign.calls)
	}
	args := cosign.calls[0]
	if filepath.Base(args[1]) != "checksums.txt" || cosign.bundle != testBundle {
		t.Errorf("verify-blob args = %v, bundle %q; want checksums.txt and the release bundle", args, cosign.bundle)
	}
	for _, want := range [][2]string{
		{"--certificate-identity", "https://github.com/oito2/mcp-sync82/.github/workflows/release.yml@refs/tags/v2.0.0"},
		{"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com"},
	} {
		if i := slices.Index(args, want[0]); i < 0 || i+1 >= len(args) || args[i+1] != want[1] {
			t.Errorf("verify-blob args = %v, want %s %s", args, want[0], want[1])
		}
	}
	if data, _ := os.ReadFile(exePath); string(data) != "new" {
		t.Errorf("binary = %q, want the new one", data)
	}
}

// TestRunSelfUpdate_FailedSignatureAborts checks that a failing cosign
// verification aborts with exit code 1, shows cosign's output and leaves
// the binary unchanged.
func TestRunSelfUpdate_FailedSignatureAborts(t *testing.T) {
	srv := fakeSignedReleaseServer(t, "v2.0.0", []byte("new"), "sync82_linux_amd64")
	exePath := writeOldBinary(t)
	cosign := &fakeCosign{version: "v3.1.0", verifyErr: errors.New("exit status 1"), verifyOutput: "Error: no matching CertificateIdentity found"}
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--yes"}, signedUpdateDeps(t, srv, exePath, cosign), strings.NewReader(""), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "Signature verification failed — aborting, nothing was changed") ||
		!strings.Contains(stderr.String(), "no matching CertificateIdentity found") {
		t.Fatalf("exit code = %d, stderr = %q; want the verification failure with cosign's output", code, stderr.String())
	}
	if data, _ := os.ReadFile(exePath); string(data) != "old" {
		t.Errorf("binary = %q, want it unchanged", data)
	}
}

// TestRunSelfUpdate_WithoutCosign checks that without a usable cosign (none
// on PATH, or one older than v3) the update warns and goes on with the
// checksum, while --require-signature refuses before downloading anything.
func TestRunSelfUpdate_WithoutCosign(t *testing.T) {
	for _, c := range []struct {
		name    string
		cosign  *fakeCosign
		warning string
	}{
		{"missing", nil, "cosign was not found on PATH"},
		{"old", &fakeCosign{version: "v2.6.1"}, `is cosign "v2.6.1", older than the v3.0.0 needed`},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := fakeSignedReleaseServer(t, "v2.0.0", []byte("new"), "sync82_linux_amd64")

			exePath := writeOldBinary(t)
			var stdout, stderr bytes.Buffer
			code := RunSelfUpdate(context.Background(), []string{"--yes"}, signedUpdateDeps(t, srv, exePath, c.cosign), strings.NewReader(""), &stdout, &stderr)
			if code != 0 || !strings.Contains(stderr.String(), "Warning: ") || !strings.Contains(stderr.String(), c.warning) ||
				!strings.Contains(stderr.String(), "the signature will not be verified, only the checksum") {
				t.Fatalf("exit code = %d, stderr = %q; want the warning and an update", code, stderr.String())
			}
			if data, _ := os.ReadFile(exePath); string(data) != "new" {
				t.Errorf("binary = %q, want the new one", data)
			}
			if c.cosign != nil && len(c.cosign.calls) != 0 {
				t.Errorf("verify-blob ran with an old cosign: %v", c.cosign.calls)
			}

			exePath = writeOldBinary(t)
			deps := signedUpdateDeps(t, srv, exePath, c.cosign)
			stdout.Reset()
			stderr.Reset()
			code = RunSelfUpdate(context.Background(), []string{"--yes", "--require-signature"}, deps, strings.NewReader(""), &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), "--require-signature refuses to update") {
				t.Fatalf("with --require-signature: exit code = %d, stderr = %q; want a refusal", code, stderr.String())
			}
			if entries, _ := os.ReadDir(deps.TempDir); len(entries) != 0 {
				t.Errorf("with --require-signature, something was staged: %v", entries)
			}
			if data, _ := os.ReadFile(exePath); string(data) != "old" {
				t.Errorf("with --require-signature: binary = %q, want it unchanged", data)
			}
		})
	}
}

// TestRunSelfUpdate_SignedReleaseWithoutBundle checks that, with cosign
// found, a release that has no signature bundle is refused.
func TestRunSelfUpdate_SignedReleaseWithoutBundle(t *testing.T) {
	srv := fakeReleaseServer(t, "v2.0.0", []byte("new"), "sync82_linux_amd64", sha256Hex([]byte("new")))
	exePath := writeOldBinary(t)
	var stdout, stderr bytes.Buffer
	code := RunSelfUpdate(context.Background(), []string{"--yes"}, signedUpdateDeps(t, srv, exePath, &fakeCosign{version: "v3.0.6"}), strings.NewReader(""), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "No checksums.txt.sigstore.json found") {
		t.Fatalf("exit code = %d, stderr = %q; want the missing bundle refused", code, stderr.String())
	}
	if data, _ := os.ReadFile(exePath); string(data) != "old" {
		t.Errorf("binary = %q, want it unchanged", data)
	}
}

// TestRunSelfUpdate_CheckExitCodes checks that --check exits with 0 when up
// to date, and that --require-signature can't be combined with --check or
// --rollback.
func TestRunSelfUpdate_CheckExitCodes(t *testing.T) {
	srv := fakeSignedReleaseServer(t, "v1.0.0", []byte("same"), "sync82_linux_amd64")
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--check"}, signedUpdateDeps(t, srv, "/unused", nil), strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Errorf("--check when up to date = %d, want 0", code)
	}
	for _, args := range [][]string{{"--check", "--require-signature"}, {"--rollback", "--require-signature"}} {
		stderr.Reset()
		if code := RunSelfUpdate(context.Background(), args, Deps{Version: "v1.0.0"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--require-signature") {
			t.Errorf("%v = %d, stderr %q; want a usage error", args, code, stderr.String())
		}
	}
}

// TestRunSelfUpdate_SwapFailureRestoresTheBinary checks that, when moving
// the new binary into place fails, the previous binary is moved back and
// the error is reported; and that when moving it back fails too, the
// message says where the previous binary is.
func TestRunSelfUpdate_SwapFailureRestoresTheBinary(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("restoreFails=%v", restoreFails), func(t *testing.T) {
			srv := fakeSignedReleaseServer(t, "v2.0.0", []byte("new"), "sync82_linux_amd64")
			exePath := writeOldBinary(t)
			deps := signedUpdateDeps(t, srv, exePath, nil)
			deps.Rename = func(oldpath, newpath string) error {
				if newpath == exePath && filepath.Base(oldpath) == "sync82_linux_amd64" {
					return errors.New("swap blocked")
				}
				if restoreFails && oldpath == exePath+".bak" {
					return errors.New("restore blocked")
				}
				return os.Rename(oldpath, newpath)
			}
			var stdout, stderr bytes.Buffer
			code := RunSelfUpdate(context.Background(), []string{"--yes"}, deps, strings.NewReader(""), &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), "swap blocked") {
				t.Fatalf("exit code = %d, stderr = %q; want the swap failure", code, stderr.String())
			}
			if restoreFails {
				if !strings.Contains(stderr.String(), "restoring the previous binary also failed") || !strings.Contains(stderr.String(), exePath+".bak") {
					t.Errorf("stderr = %q, want it to name the backup", stderr.String())
				}
				if data, _ := os.ReadFile(exePath + ".bak"); string(data) != "old" {
					t.Errorf("backup = %q, want the previous binary", data)
				}
				return
			}
			if data, _ := os.ReadFile(exePath); string(data) != "old" {
				t.Errorf("binary = %q, want the previous one restored", data)
			}
			if _, err := os.Stat(exePath + ".bak"); err == nil {
				t.Error("a backup was left behind after the restore")
			}
		})
	}
}

// TestRunSelfUpdate_RollbackSwapFailureKeepsTheBackup checks that, when the
// rollback's swap fails, the staged backup is moved back to "<binary>.bak"
// and the running binary is unchanged.
func TestRunSelfUpdate_RollbackSwapFailureKeepsTheBackup(t *testing.T) {
	exePath := filepath.Join(t.TempDir(), "sync82")
	for path, content := range map[string]string{exePath: "v2", exePath + ".bak": "v1"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	deps := Deps{
		ExecutablePath: func() (string, error) { return exePath, nil },
		RunVersion:     reportVersion("v1.0.0"),
		Rename: func(oldpath, newpath string) error {
			if newpath == exePath+".bak" && oldpath == exePath {
				return errors.New("swap blocked")
			}
			return os.Rename(oldpath, newpath)
		},
	}
	var stdout, stderr bytes.Buffer
	if code := RunSelfUpdate(context.Background(), []string{"--rollback"}, deps, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if data, _ := os.ReadFile(exePath); string(data) != "v2" {
		t.Errorf("binary = %q, want it unchanged", data)
	}
	if data, _ := os.ReadFile(exePath + ".bak"); string(data) != "v1" {
		t.Errorf("backup = %q, want it kept", data)
	}
	if _, err := os.Stat(exePath + ".rollback"); err == nil {
		t.Error("the staged rollback file was left behind")
	}
}
