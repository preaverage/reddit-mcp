package browser

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	releasesURL = "https://api.github.com/repos/daijro/camoufox/releases/latest"

	installDir  = "camoufox"
	binaryUnix  = "camoufox-bin"
	binaryWin   = "camoufox.exe"
	archiveName = "camoufox.zip"

	binaryPerm = 0o755
)

var osTokens = map[string]string{
	"linux":   "lin",
	"windows": "win",
	"darwin":  "mac",
}

var archTokens = map[string]string{
	"amd64": "x86_64",
	"arm64": "arm64",
}

type release struct {
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func InstallDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, cacheDir, installDir)

	return dir, os.MkdirAll(dir, dirPerm)
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return binaryWin
	}

	return binaryUnix
}

func Install() (string, error) {
	dir, err := InstallDir()
	if err != nil {
		return "", err
	}

	binary := filepath.Join(dir, binaryName())
	if _, err := os.Stat(binary); err == nil {
		return binary, nil
	}

	url, err := assetURL()
	if err != nil {
		return "", err
	}

	archive := filepath.Join(dir, archiveName)
	defer os.Remove(archive)

	if err := download(url, archive); err != nil {
		return "", fmt.Errorf("download camoufox: %w", err)
	}

	if err := extract(archive, dir); err != nil {
		return "", fmt.Errorf("extract camoufox: %w", err)
	}

	if err := os.Chmod(binary, binaryPerm); err != nil {
		return "", fmt.Errorf("camoufox binary missing after extract: %w", err)
	}

	return binary, nil
}

func assetURL() (string, error) {
	osToken, ok := osTokens[runtime.GOOS]
	if !ok {
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	archToken, ok := archTokens[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	resp, err := http.Get(releasesURL)
	if err != nil {
		return "", fmt.Errorf("query camoufox releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("query camoufox releases: %s", resp.Status)
	}

	var latest release
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return "", fmt.Errorf("decode camoufox releases: %w", err)
	}

	for _, asset := range latest.Assets {
		if strings.Contains(asset.Name, osToken) && strings.Contains(asset.Name, archToken) {
			return asset.URL, nil
		}
	}

	return "", fmt.Errorf("no camoufox build for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func download(url, dst string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}

func extract(archive, dst string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path, err := safeJoin(dst, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, dirPerm); err != nil {
				return err
			}

			continue
		}

		if err := writeZipEntry(f, path); err != nil {
			return err
		}
	}

	return nil
}

func safeJoin(dst, name string) (string, error) {
	path := filepath.Join(dst, name)

	if !strings.HasPrefix(path, filepath.Clean(dst)+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes install directory: %s", name)
	}

	return path, nil
}

func writeZipEntry(f *zip.File, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}

	in, err := f.Open()
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
