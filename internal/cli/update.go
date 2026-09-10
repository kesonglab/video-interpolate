package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kesonglab/video-interpolate/internal/version"
	"github.com/spf13/cobra"
)

const (
	updateRepo      = "kesonglab/video-interpolate"
	apiBaseURL      = "https://api.github.com"
	downloadBaseURL = "https://github.com"
)

var tagRe = regexp.MustCompile(`"tag_name"\s*:\s*"v?([^"]+)"`)

func newUpdateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update vif to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd.OutOrStdout(), cmd.ErrOrStderr(), check)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "check for updates without installing")
	return cmd
}

func runUpdate(stdout, stderr io.Writer, checkOnly bool) error {
	return runUpdateFrom(stdout, stderr, apiBaseURL, downloadBaseURL, checkOnly)
}

func runUpdateFrom(stdout, stderr io.Writer, apiBase, downloadBase string, checkOnly bool) error {
	latest, err := fetchLatestVersion(apiBase)
	if err != nil {
		return err
	}

	current := strings.TrimPrefix(version.Version, "v")
	fmt.Fprintf(stdout, "current: %s\nlatest: %s\n", current, latest)

	if current == latest {
		fmt.Fprintf(stdout, "✓ already at latest version (%s)\n", current)
		return nil
	}
	if checkOnly {
		fmt.Fprintf(stdout, "update available: %s → %s (run 'vif update' to install)\n", current, latest)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exe, _ = filepath.EvalSymlinks(exe)

	url := fmt.Sprintf("%s/%s/releases/download/v%s/vif_%s_%s_%s.tar.gz",
		downloadBase, updateRepo, latest, latest, runtime.GOOS, runtime.GOARCH)

	fmt.Fprintf(stdout, "↓ downloading %s...\n", url)
	tmpDir, err := os.MkdirTemp("", "vif-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "vif.tar.gz")
	if err := downloadFile(tarPath, url); err != nil {
		return err
	}
	if err := extractTarGz(tarPath, tmpDir); err != nil {
		return err
	}
	newBin := filepath.Join(tmpDir, "vif")

	fmt.Fprintf(stdout, "→ installing to %s\n", exe)
	if err := replaceBinary(exe, newBin); err != nil {
		return err
	}

	// downloads via curl don't carry quarantine, but clear it in case this
	// binary ever got one from a browser
	_ = removeQuarantine(exe)

	fmt.Fprintf(stdout, "✓ updated to %s\n", latest)
	fmt.Fprintln(stdout, "run 'vif version' to verify")
	return nil
}

func fetchLatestVersion(apiBase string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", apiBase, updateRepo)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch latest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	m := tagRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("no tag_name found in response")
	}
	return string(m[1]), nil
}

func downloadFile(dst, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// extractTarGz extracts the release tarball into dst.
func extractTarGz(tarPath, dst string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		target := filepath.Join(dst, filepath.Base(hdr.Name))
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
		os.Chmod(target, 0o755)
	}
	return nil
}

// replaceBinary swaps src in over dst. The running binary can't be overwritten
// in place, so the old one is renamed aside first (macOS keeps the inode),
// then the new one moves in. Rolls back on failure.
func replaceBinary(dst, src string) error {
	backup := dst + ".old"
	os.Remove(backup) // stale backup from a failed run
	if err := os.Rename(dst, backup); err != nil {
		return fmt.Errorf("backup old binary: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		os.Rename(backup, dst)
		return fmt.Errorf("install new binary: %w", err)
	}
	os.Remove(backup)
	return nil
}

// removeQuarantine drops com.apple.quarantine if present; best effort.
func removeQuarantine(path string) error {
	return exec.Command("xattr", "-d", "com.apple.quarantine", path).Run()
}
