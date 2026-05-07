package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/version"
)

type versionFlags struct {
	check bool
}

func versionFlagSet(v *versionFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&v.check, "check", false, "check GitHub for a newer release")
	return fs
}

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
		Out:        os.Stdout,
		Err:        os.Stderr,
		Version:    version.String(),
		ReleaseURL: defaultReleaseURL,
	}
}

func (c *VersionCommand) Execute(args []string) int {
	var v versionFlags
	fs := versionFlagSet(&v)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			c.writeUsage(c.Out)
			return 0
		}
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}

	if len(fs.Args()) > 0 {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("unsupported arguments: %v", fs.Args())))
		return usageExitCode
	}

	if v.check {
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
	req.Header.Set("User-Agent", "klvtool/"+c.Version)

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

func (c *VersionCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
}

func (c *VersionCommand) writeUsage(w io.Writer) {
	commanddef.RenderHelp(versionDef, versionFlagSet(&versionFlags{}), w)
}

// Definition returns the CommandDef driving --help and man-page generation.
func (c *VersionCommand) Definition() commanddef.CommandDef { return versionDef }

var versionDef = commanddef.CommandDef{
	Name:        "klvtool-version",
	Subcommand:  "version",
	Synopsis:    "Print the klvtool version, optionally checking GitHub for a newer release.",
	UsageLine:   "klvtool version [--check]",
	Description: "Print the embedded klvtool version. With --check, query the GitHub API for the latest release tag and report whether the local build is current.",
	Examples: []commanddef.Example{
		{Comment: "Print the version", Command: "klvtool version"},
		{Comment: "Check for a newer release", Command: "klvtool version --check"},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "success (always — update-check failures are reported but do not change exit code)"},
		{Code: 2, Meaning: "invalid usage"},
	},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool", Section: 1},
		{Name: "klvtool-update", Section: 1},
	},
}
