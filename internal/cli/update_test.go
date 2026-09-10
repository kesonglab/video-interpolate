package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kesonglab/video-interpolate/internal/version"
)

// newReleaseServer fakes the GitHub latest-release API for a given tag.
func newReleaseServer(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tag_name": "v%s"}`, tag)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUpdate_checkVersion(t *testing.T) {
	srv := newReleaseServer(t, "9.9.9")
	var out, errOut bytes.Buffer
	err := runUpdateFrom(&out, &errOut, srv.URL, "https://github.com", true)
	if err != nil {
		t.Fatalf("runUpdateFrom: %v", err)
	}
	if !strings.Contains(out.String(), "update available") {
		t.Errorf("output missing update hint:\n%s", out.String())
	}
}

func TestUpdate_alreadyLatest(t *testing.T) {
	old := version.Version
	version.Version = "1.0.0"
	t.Cleanup(func() { version.Version = old })

	downloads := 0
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
	}))
	t.Cleanup(dlSrv.Close)

	srv := newReleaseServer(t, "1.0.0")
	var out, errOut bytes.Buffer
	err := runUpdateFrom(&out, &errOut, srv.URL, dlSrv.URL, false)
	if err != nil {
		t.Fatalf("runUpdateFrom: %v", err)
	}
	if !strings.Contains(out.String(), "already at latest") {
		t.Errorf("output missing already-latest message:\n%s", out.String())
	}
	if downloads != 0 {
		t.Errorf("download server hit %d times, want 0", downloads)
	}
}

func TestUpdate_replaceBinary(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "vif")
	src := filepath.Join(dir, "new")
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(dst, src); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new" {
		t.Errorf("dst = %q, want new", b)
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Errorf("backup %s.old not cleaned up", dst)
	}
}

func TestUpdate_rollback(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "vif")
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// missing src forces the install rename to fail
	if err := replaceBinary(dst, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("replaceBinary succeeded, want error")
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old" {
		t.Errorf("rollback failed: dst = %q, want old", b)
	}
}

func TestUpdate_downloadAndExtract(t *testing.T) {
	tarball := makeReleaseTarball(t, []byte("#!/bin/sh\necho vif"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	tarPath := filepath.Join(dir, "vif.tar.gz")
	if err := downloadFile(tarPath, srv.URL); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	if err := extractTarGz(tarPath, dir); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "vif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "#!/bin/sh\necho vif" {
		t.Errorf("extracted binary = %q", b)
	}
}

func makeReleaseTarball(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "vif", Mode: 0o755, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}
