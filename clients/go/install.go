package mocklydriver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const DefaultMocklyVersion = "v0.1.0"
const githubBase = "https://github.com/dever-labs/mockly/releases/download"

// GetBinaryPath returns the path to the Mockly binary, or empty string if not found.
// Checks: MOCKLY_BINARY_PATH env var, then binDir/mockly[.exe], then ./bin/mockly[.exe].
func GetBinaryPath(binDir string) string {
	if p := os.Getenv("MOCKLY_BINARY_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}

	name := "mockly"
	if runtime.GOOS == "windows" {
		name = "mockly.exe"
	}

	for _, dir := range []string{binDir, "./bin"} {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Install downloads the Mockly binary for the current platform.
// Returns the path to the installed binary.
// Respects MOCKLY_BINARY_PATH, MOCKLY_NO_INSTALL, MOCKLY_DOWNLOAD_BASE_URL, MOCKLY_VERSION.
// HTTPS_PROXY / HTTP_PROXY are handled automatically by net/http.
func Install(opts InstallOptions) (string, error) {
	// 1. Check MOCKLY_BINARY_PATH first.
	if p := os.Getenv("MOCKLY_BINARY_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("MOCKLY_BINARY_PATH is set to %q but the file does not exist", p)
	}

	// 2. Resolve options from env.
	version := opts.Version
	if v := os.Getenv("MOCKLY_VERSION"); v != "" {
		version = v
	}
	if version == "" {
		version = DefaultMocklyVersion
	}

	baseURL := opts.BaseURL
	if u := os.Getenv("MOCKLY_DOWNLOAD_BASE_URL"); u != "" {
		baseURL = u
	}
	if baseURL == "" {
		baseURL = githubBase
	}
	baseURL = strings.TrimRight(baseURL, "/")

	binDir := opts.BinDir
	if binDir == "" {
		binDir = "./bin"
	}

	// 3. Check if already installed (skip if Force).
	if !opts.Force {
		if p := GetBinaryPath(binDir); p != "" {
			return p, nil
		}
	}

	// 4. Honour MOCKLY_NO_INSTALL.
	if os.Getenv("MOCKLY_NO_INSTALL") != "" {
		return "", fmt.Errorf(
			"MOCKLY_NO_INSTALL is set: refusing to download mockly binary; " +
				"set MOCKLY_BINARY_PATH to point at a pre-staged binary or unset MOCKLY_NO_INSTALL",
		)
	}

	// 5. Determine platform asset name.
	asset, err := getAssetName()
	if err != nil {
		return "", err
	}

	// 6. Build download URL.
	downloadURL := fmt.Sprintf("%s/%s/%s", baseURL, version, asset)

	// 7. Create destination directory.
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("creating bin directory %q: %w", binDir, err)
	}

	destName := "mockly"
	if runtime.GOOS == "windows" {
		destName = "mockly.exe"
	}
	dest := filepath.Join(binDir, destName)

	// 8. Download binary (Go's http.Client follows redirects automatically).
	resp, err := http.Get(downloadURL) //nolint:gosec // URL is constructed from trusted inputs
	if err != nil {
		return "", fmt.Errorf("downloading mockly from %s: %w", downloadURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading mockly: server returned HTTP %d for %s", resp.StatusCode, downloadURL)
	}

	// 9. Write to disk.
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("creating binary file %q: %w", dest, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)
		return "", fmt.Errorf("writing binary to %q: %w", dest, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing binary file %q: %w", dest, err)
	}

	// 10. Set executable bit on non-Windows.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dest, 0755); err != nil {
			return "", fmt.Errorf("setting executable bit on %q: %w", dest, err)
		}
	}

	return dest, nil
}

// getAssetName returns the platform-specific binary filename.
func getAssetName() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "mockly-linux-amd64", nil
		case "arm64":
			return "mockly-linux-arm64", nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "mockly-darwin-amd64", nil
		case "arm64":
			return "mockly-darwin-arm64", nil
		}
	case "windows":
		if goarch == "amd64" {
			return "mockly-windows-amd64.exe", nil
		}
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}
