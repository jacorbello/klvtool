package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultReleaseURL = "https://api.github.com/repos/jacorbello/klvtool/releases/latest"

type VersionCommand struct {
	Out        io.Writer
	Err        io.Writer
	Version    string
	ReleaseURL string
	HTTPClient *http.Client
}

func NewVersionCommand() *VersionCommand {
	return &VersionCommand{
		ReleaseURL: defaultReleaseURL,
	}
}

func (c *VersionCommand) Execute(args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "check for updates")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			c.writeUsage(c.Out)
			return 0
		}
		c.writeUsage(c.Err)
		return usageExitCode
	}

	if *check {
		return c.executeCheck()
	}

	_, _ = fmt.Fprintf(c.Out, "klvtool %s\n", c.Version)
	return 0
}

func (c *VersionCommand) executeCheck() int {
	if c.Version == "dev" || c.Version == "" {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s (update check skipped — dev build)\n", c.Version)
		return 0
	}

	latest, url, err := c.fetchLatestRelease()
	if err != nil {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s (update check failed: %v)\n", c.Version, err)
		return 0
	}

	if latest == c.Version {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s (up to date)\n", c.Version)
	} else {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s — %s available at %s\n", c.Version, latest, url)
	}
	return 0
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func (c *VersionCommand) fetchLatestRelease() (tag string, url string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ReleaseURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}

	return release.TagName, release.HTMLURL, nil
}

func (c *VersionCommand) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *VersionCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool version [--check]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Print the klvtool version.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Flags:")
	_, _ = fmt.Fprintln(w, "  --check  Check for a newer release on GitHub")
}
