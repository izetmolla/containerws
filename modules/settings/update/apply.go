package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func currentBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func downloadAndInstall(ctx context.Context, assetURL, assetName string) (installedPath string, err error) {
	if strings.TrimSpace(assetURL) == "" {
		return "", errors.New("missing asset download URL")
	}
	exePath, err := currentBinaryPath()
	if err != nil {
		return "", fmt.Errorf("resolve binary: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "containerws-update-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, filepath.Base(assetName))
	if err := downloadFile(ctx, assetURL, archivePath); err != nil {
		return "", err
	}

	binPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return "", err
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", err
	}

	bak := exePath + ".bak"
	_ = os.Remove(bak)
	if err := copyFile(exePath, bak); err != nil {
		// Non-fatal on some filesystems / first install layouts.
		_ = err
	}

	destTmp := exePath + ".new"
	_ = os.Remove(destTmp)
	if err := copyFile(binPath, destTmp); err != nil {
		return "", fmt.Errorf("stage new binary: %w", err)
	}
	if err := os.Chmod(destTmp, 0o755); err != nil {
		_ = os.Remove(destTmp)
		return "", err
	}
	if err := os.Rename(destTmp, exePath); err != nil {
		// Fallback: remove then rename (Windows-ish), or copy over.
		_ = os.Remove(exePath)
		if err2 := os.Rename(destTmp, exePath); err2 != nil {
			if err3 := copyFile(destTmp, exePath); err3 != nil {
				_ = os.Remove(destTmp)
				return "", fmt.Errorf("replace binary: %v / %v / %v", err, err2, err3)
			}
			_ = os.Remove(destTmp)
			_ = os.Chmod(exePath, 0o755)
		}
	}
	syncFilesystem(exePath)
	return exePath, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "containerws-update")
	req.Header.Set("Accept", "application/octet-stream")
	// Only attach a token for api.github.com asset endpoints; public
	// /releases/download URLs must not forward Authorization across redirects.
	if tok := githubToken(); tok != "" && strings.Contains(url, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	client := &http.Client{
		Timeout: 15 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			// Drop auth when leaving api.github.com (CDN / objects.githubusercontent.com).
			if req.URL.Host != "api.github.com" {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Sync()
}

func extractBinary(archivePath, destDir string) (string, error) {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGzBinary(archivePath, destDir)
	default:
		return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

func extractTarGzBinary(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var found string
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != "containerws" && base != "containerws.exe" && base != "cws" {
			continue
		}
		out := filepath.Join(destDir, base)
		w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(w, tr); err != nil {
			_ = w.Close()
			return "", err
		}
		_ = w.Close()
		found = out
		// Prefer containerws over cws if both appear.
		if base == "containerws" || base == "containerws.exe" {
			return found, nil
		}
	}
	if found == "" {
		return "", fmt.Errorf("archive does not contain containerws binary (os=%s arch=%s)", runtime.GOOS, runtime.GOARCH)
	}
	return found, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
