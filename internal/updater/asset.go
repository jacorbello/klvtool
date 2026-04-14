package updater

import "fmt"

// ReleaseAsset is a subset of a GitHub release asset object.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// PickAssetURL returns the browser_download_url for the archive matching goos/goarch.
func PickAssetURL(assets []ReleaseAsset, goos, goarch string) (string, error) {
	want := ArchiveFileName(goos, goarch)
	for _, a := range assets {
		if a.Name == want {
			if a.BrowserDownloadURL == "" {
				return "", fmt.Errorf("asset %q has empty URL", want)
			}
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release has no asset %q", want)
}

// ChecksumsAssetURL returns the browser_download_url for checksums.txt.
func ChecksumsAssetURL(assets []ReleaseAsset) (string, error) {
	for _, a := range assets {
		if a.Name == "checksums.txt" {
			if a.BrowserDownloadURL == "" {
				return "", fmt.Errorf("asset checksums.txt has empty URL")
			}
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("release has no asset %q", "checksums.txt")
}
