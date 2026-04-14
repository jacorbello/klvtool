package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LatestRelease is the subset of GitHub's release JSON used for updates.
type LatestRelease struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []ReleaseAsset `json:"assets"`
}

// FetchLatestRelease GETs releaseURL (typically .../releases/latest) and decodes JSON.
func FetchLatestRelease(ctx context.Context, client *http.Client, releaseURL, userAgent string) (*LatestRelease, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release fetch: %s", resp.Status)
	}
	var rel LatestRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// DownloadBytes GETs url and returns the full body (bounded).
func DownloadBytes(ctx context.Context, client *http.Client, url, userAgent string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: %s", resp.Status)
	}
	const maxDownload = 256 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxDownload {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return b, nil
}
