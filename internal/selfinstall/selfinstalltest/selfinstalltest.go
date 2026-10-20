// Package selfinstalltest fakes what the self-install tests need: a CLI
// binary that answers `version`, a release server laid out the way
// GoReleaser publishes, and a global npm install with its npm. The fake
// binaries are native executables built from package fakecli, so the tests
// run the same way on Windows as on macOS and Linux.
package selfinstalltest

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tollbit/cli/internal/selfinstall/selfinstalltest/fakecli"
)

const fakecliCommand = "github.com/tollbit/cli/internal/selfinstall/selfinstalltest/fakecli/cmd"

var (
	// goTool is looked up when the test binary starts, before any test
	// replaces PATH. go test puts its own go first on PATH.
	goTool, goToolErr = exec.LookPath("go")

	buildProgram = sync.OnceValues(func() ([]byte, error) {
		if goToolErr != nil {
			return nil, fmt.Errorf("find go to build the fake CLI: %w", goToolErr)
		}
		dir, err := os.MkdirTemp("", "selfinstalltest-*")
		if err != nil {
			return nil, err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		out := filepath.Join(dir, ExeName())
		cmd := exec.Command(goTool, "build", "-ldflags=-s -w", "-o", out, fakecliCommand)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("build the fake CLI: %w\n%s", err, output)
		}
		return os.ReadFile(out)
	})
)

type (
	// Release is one version for one platform. Empty GOOS and GOARCH mean
	// the running platform's.
	Release struct {
		Version string
		GOOS    string
		GOARCH  string
		// BadChecksum publishes a checksum that does not match the archive.
		BadChecksum bool
	}

	// Server serves a Release under URL/v<version>/.
	Server struct {
		URL string
		// Requests counts every request the server received.
		Requests *atomic.Int32
	}

	// NPMInstall is a fake global npm install: the package under
	// Prefix/lib/node_modules, and a fake npm first on PATH that answers
	// `npm prefix -g` with Prefix and, for `npm install -g <pkg>@<version>`,
	// replaces Exe with Binary(version) or fails.
	NPMInstall struct {
		Prefix string
		// Exe is the package's binary, Binary(current) until npm installs.
		Exe string
		// Calls is the file the fake npm records each call's arguments in,
		// one line per call.
		Calls string
	}
)

// Serve starts a release server for r, stopped when the test ends. The
// archive holds a fake binary built by Binary.
func Serve(t *testing.T, r Release) Server {
	t.Helper()
	goos, goarch := r.GOOS, r.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	name := fmt.Sprintf("tollbit_%s_%s_%s", r.Version, goos, goarch)
	var archive []byte
	if goos == "windows" {
		name += ".zip"
		archive = zipArchive(t, "tollbit.exe", Binary(t, r.Version))
	} else {
		name += ".tar.gz"
		archive = tarGzArchive(t, "tollbit", Binary(t, r.Version))
	}
	sum := sha256.Sum256(archive)
	if r.BadChecksum {
		sum = sha256.Sum256([]byte("something else"))
	}
	checksums := fmt.Sprintf("%s  %s\n%s  tollbit_%s_other_arch.tar.gz\n", hex.EncodeToString(sum[:]), name, hex.EncodeToString(sum[:]), r.Version)

	requests := new(atomic.Int32)
	mux := http.NewServeMux()
	prefix := "/v" + r.Version + "/"
	mux.HandleFunc(prefix+name, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc(prefix+fmt.Sprintf("tollbit_%s_checksums.txt", r.Version), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, req)
	}))
	t.Cleanup(srv.Close)
	return Server{URL: srv.URL, Requests: requests}
}

// ExeName is the CLI binary's file name on the running platform. Windows
// runs only files named .exe.
func ExeName() string {
	if runtime.GOOS == "windows" {
		return "tollbit.exe"
	}
	return "tollbit"
}

// Binary returns a stand-in for the CLI binary that answers `version` with
// version; see fakecli.Main for the rest of what it does.
func Binary(t *testing.T, version string) []byte {
	t.Helper()
	return fakecli.Append(program(t), fakecli.Config{Version: version})
}

// program is the build of the fakecli command, made once per test binary.
func program(t *testing.T) []byte {
	t.Helper()
	program, err := buildProgram()
	if err != nil {
		t.Fatal(err)
	}
	return program
}

// AssertBinary fails the test unless path holds Binary(version).
func AssertBinary(t *testing.T, path, version string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, Binary(t, version)) {
		_, config, err := fakecli.Split(got)
		t.Fatalf("expected %s to be the %s binary, got %+v (%v)", path, version, config, err)
	}
}

// WriteBinary writes Binary(version) to path as an executable.
func WriteBinary(t *testing.T, path, version string) {
	t.Helper()
	if err := os.WriteFile(path, Binary(t, version), 0o755); err != nil {
		t.Fatal(err)
	}
}

func tarGzArchive(t *testing.T, binary string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name    string
		content []byte
	}{
		{name: "LICENSE", content: []byte("license\n")},
		{name: binary, content: content},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o755, Size: int64(len(f.content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, binary string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(binary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// FakeNPM sets up an NPMInstall for current whose npm install fails when
// fail is set. PATH holds only the fake npm, so the test sees no other npm.
func FakeNPM(t *testing.T, current string, fail bool) NPMInstall {
	t.Helper()
	prefix, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(prefix, "lib", "node_modules", "@tollbit", "tollbit-cli")
	for _, dir := range []string{pkgDir, filepath.Join(prefix, "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	install := NPMInstall{
		Prefix: prefix,
		Exe:    filepath.Join(pkgDir, ExeName()),
		Calls:  filepath.Join(t.TempDir(), "npm-calls"),
	}
	WriteBinary(t, install.Exe, current)

	npm := fakecli.Append(program(t), fakecli.Config{NPM: &fakecli.NPM{
		Prefix: prefix,
		Exe:    install.Exe,
		Calls:  install.Calls,
		Fail:   fail,
	}})
	npmName := "npm"
	if runtime.GOOS == "windows" {
		npmName = "npm.exe"
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, npmName), npm, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	return install
}

// NPMCalls returns the arguments of each npm call so far.
func (n NPMInstall) NPMCalls(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(n.Calls)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}
