package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"teledrive/internal/app"
)

const (
	RepoOwner = "herliansyah"
	RepoName  = "teledrive"
)

type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

type UpdateCheckResult struct {
	CurrentVersion  string       `json:"current_version"`
	LatestVersion   string       `json:"latest_version"`
	UpdateAvailable bool         `json:"update_available"`
	Release         *ReleaseInfo `json:"release,omitempty"`
}

// CompareVersions returns 1 if v1 > v2, -1 if v1 < v2, and 0 if equal.
func CompareVersions(v1, v2 string) int {
	clean := func(v string) []int {
		v = strings.TrimPrefix(strings.TrimSpace(v), "v")
		parts := strings.Split(v, ".")
		res := make([]int, 3)
		for i := 0; i < len(parts) && i < 3; i++ {
			// Extract numeric portion if pre-release suffix exists
			numStr := strings.Split(parts[i], "-")[0]
			n, _ := strconv.Atoi(numStr)
			res[i] = n
		}
		return res
	}

	nums1 := clean(v1)
	nums2 := clean(v2)

	for i := 0; i < 3; i++ {
		if nums1[i] > nums2[i] {
			return 1
		}
		if nums1[i] < nums2[i] {
			return -1
		}
	}
	return 0
}

// CheckForUpdate queries the GitHub API for the latest release.
func CheckForUpdate(client *http.Client) (*UpdateCheckResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", RepoOwner, RepoName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TeleDrive-Updater/"+app.Version)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query GitHub releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	currentVer := strings.TrimPrefix(app.Version, "v")

	updateAvailable := CompareVersions(latestVer, currentVer) > 0

	return &UpdateCheckResult{
		CurrentVersion:  app.Version,
		LatestVersion:   release.TagName,
		UpdateAvailable: updateAvailable,
		Release:         &release,
	}, nil
}

// FindAssetForCurrentPlatform finds the release asset matching runtime.GOOS and runtime.GOARCH.
func FindAssetForCurrentPlatform(release *ReleaseInfo) (string, string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	expectedExt := ".tar.gz"
	if goos == "windows" {
		expectedExt = ".zip"
	}

	// Pattern: teledrive-vX.Y.Z-linux-amd64.tar.gz or teledrive-linux-amd64.tar.gz
	platformMatch := fmt.Sprintf("%s-%s", goos, goarch)

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, platformMatch) && strings.HasSuffix(name, expectedExt) {
			return asset.Name, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("no compatible release asset found for %s/%s", goos, goarch)
}

// ApplyUpdate downloads the release archive, extracts the binary, replaces the active executable, and prepares restart.
func ApplyUpdate(downloadURL string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("could not resolve symlink: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	tempBin := filepath.Join(exeDir, fmt.Sprintf(".teledrive_update_%d", time.Now().UnixNano()))
	if runtime.GOOS == "windows" {
		tempBin += ".exe"
	}

	// Download archive to temp file
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	if strings.HasSuffix(downloadURL, ".zip") {
		// Save zip to temp file first
		zipFile, err := os.CreateTemp(exeDir, "teledrive_zip_*.zip")
		if err != nil {
			return err
		}
		defer os.Remove(zipFile.Name())

		if _, err := io.Copy(zipFile, resp.Body); err != nil {
			zipFile.Close()
			return err
		}
		zipFile.Close()

		if err := extractZipBinary(zipFile.Name(), tempBin); err != nil {
			return err
		}
	} else {
		// tar.gz stream extraction
		if err := extractTarGzBinary(resp.Body, tempBin); err != nil {
			return err
		}
	}
	defer os.Remove(tempBin)

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tempBin, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}

	// Replace active executable
	if runtime.GOOS == "windows" {
		oldExe := exePath + ".old"
		_ = os.Remove(oldExe)
		if err := os.Rename(exePath, oldExe); err != nil {
			return fmt.Errorf("failed to rotate current windows executable: %w", err)
		}
		if err := os.Rename(tempBin, exePath); err != nil {
			// Rollback
			_ = os.Rename(oldExe, exePath)
			return fmt.Errorf("failed to place new executable: %w", err)
		}
	} else {
		// On Unix, atomic rename replaces running executable seamlessly
		if err := os.Rename(tempBin, exePath); err != nil {
			return fmt.Errorf("failed to replace executable: %w", err)
		}
	}

	return nil
}

func extractTarGzBinary(r io.Reader, destPath string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	targetName := "teledrive"

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		baseName := filepath.Base(header.Name)
		if baseName == targetName && header.Typeflag != tar.TypeDir {
			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			return err
		}
	}

	return fmt.Errorf("binary %q not found inside release archive", targetName)
}

func extractZipBinary(zipPath, destPath string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	targetName := "teledrive.exe"
	for _, f := range zr.File {
		if filepath.Base(f.Name) == targetName || filepath.Base(f.Name) == "teledrive" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, rc)
			out.Close()
			return err
		}
	}

	return fmt.Errorf("binary %q not found inside zip archive", targetName)
}

// RestartServer re-executes the current binary with the original arguments.
func RestartServer() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start updated process: %w", err)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}
