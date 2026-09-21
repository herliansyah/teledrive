package update

import (
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"v1.5.0", "v1.4.0", 1},
		{"1.4.0", "1.5.0", -1},
		{"v1.4.0", "1.4.0", 0},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.4.1", "v1.4.0", 1},
		{"v1.4.0", "v1.4.1", -1},
	}

	for _, tc := range tests {
		res := CompareVersions(tc.v1, tc.v2)
		if res != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tc.v1, tc.v2, res, tc.expected)
		}
	}
}

func TestFindAssetForCurrentPlatform(t *testing.T) {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	expectedName := "teledrive-v1.5.0-" + runtime.GOOS + "-" + runtime.GOARCH + ext

	release := &ReleaseInfo{
		TagName: "v1.5.0",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{
				Name:               "teledrive-v1.5.0-otheros-otherarch.tar.gz",
				BrowserDownloadURL: "https://example.com/other",
			},
			{
				Name:               expectedName,
				BrowserDownloadURL: "https://example.com/match",
			},
		},
	}

	name, url, err := FindAssetForCurrentPlatform(release)
	if err != nil {
		t.Fatalf("FindAssetForCurrentPlatform failed: %v", err)
	}
	if name != expectedName {
		t.Errorf("expected asset name %s, got %s", expectedName, name)
	}
	if url != "https://example.com/match" {
		t.Errorf("expected url https://example.com/match, got %s", url)
	}
}
