package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/updater"
	"github.com/jacorbello/klvtool/internal/version"
)

const goInstallModule = "github.com/jacorbello/klvtool/cmd/klvtool"

// UpdateCommand updates klvtool using go install when possible, otherwise a release archive.
type UpdateCommand struct {
	Out        io.Writer
	Err        io.Writer
	Version    string
	ReleaseURL string
	HTTPClient *http.Client
	GoModule   string

	LookPath   func(string) (string, error)
	Executable func() (string, error)
	GOOS       string
	GOARCH     string

	RunGo func(ctx context.Context, goBin string, args []string) (stdout, stderr []byte, err error)
}

func NewUpdateCommand() *UpdateCommand {
	return &UpdateCommand{
		Out:        os.Stdout,
		Err:        os.Stderr,
		Version:    version.String(),
		ReleaseURL: defaultReleaseURL,
		HTTPClient: nil,
		GoModule:   goInstallModule,
		LookPath:   exec.LookPath,
		Executable: os.Executable,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		RunGo:      defaultRunGo,
	}
}

func defaultRunGo(ctx context.Context, goBin string, args []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, goBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (c *UpdateCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dryRun := fs.Bool("dry-run", false, "print actions without installing")
	preferBinary := fs.Bool("prefer-binary", false, "download the release archive even if go is available")

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

	if c.Version == "dev" || c.Version == "" {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s (update skipped — dev build)\n", c.Version)
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ua := "klvtool/" + c.Version
	rel, err := updater.FetchLatestRelease(ctx, c.httpClient(), c.ReleaseURL, ua)
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	if rel.TagName == c.Version {
		_, _ = fmt.Fprintf(c.Out, "klvtool %s (up to date)\n", c.Version)
		return 0
	}

	if *dryRun {
		c.printDryRun(c.Out, rel, *preferBinary)
		return 0
	}

	useGo := !*preferBinary
	if useGo {
		if goBin, err := c.lookPath()("go"); err == nil {
			return c.runGoInstall(ctx, goBin, rel.TagName)
		}
	}

	return c.installFromRelease(ctx, rel, ua)
}

func (c *UpdateCommand) printDryRun(out io.Writer, rel *updater.LatestRelease, preferBinary bool) {
	if out == nil {
		return
	}
	_, _ = fmt.Fprintf(out, "would update %s -> %s\n", c.Version, rel.TagName)
	if !preferBinary {
		if _, err := c.lookPath()("go"); err == nil {
			_, _ = fmt.Fprintf(out, "strategy: go install %s@%s\n", c.goModule(), rel.TagName)
			return
		}
	}
	arch := updater.ArchiveFileName(c.goos(), c.goarch())
	if url, err := updater.PickAssetURL(rel.Assets, c.goos(), c.goarch()); err == nil {
		_, _ = fmt.Fprintf(out, "strategy: download %s\n", arch)
		_, _ = fmt.Fprintf(out, "  %s\n", url)
	}
	if c.goos() == "windows" {
		_, _ = fmt.Fprintln(out, "note: on Windows the new binary is written beside the current executable as *.exe.new; replace manually if needed.")
	}
}

func (c *UpdateCommand) runGoInstall(ctx context.Context, goBin, tag string) int {
	args := []string{"install", c.goModule() + "@" + tag}
	stdout, stderr, err := c.runGo()(ctx, goBin, args)
	if len(stdout) > 0 {
		_, _ = c.Out.Write(stdout)
	}
	if len(stderr) > 0 {
		_, _ = c.Err.Write(stderr)
	}
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "go install failed: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(c.Out, "updated via go install to %s\n", tag)
	return 0
}

func (c *UpdateCommand) installFromRelease(ctx context.Context, rel *updater.LatestRelease, userAgent string) int {
	ua := userAgent
	archName := updater.ArchiveFileName(c.goos(), c.goarch())
	archiveURL, err := updater.PickAssetURL(rel.Assets, c.goos(), c.goarch())
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	sumsURL, err := updater.ChecksumsAssetURL(rel.Assets)
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	sumsData, err := updater.DownloadBytes(ctx, c.httpClient(), sumsURL, ua)
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	archiveData, err := updater.DownloadBytes(ctx, c.httpClient(), archiveURL, ua)
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	if err := updater.VerifyChecksumInFile(string(sumsData), archName, archiveData); err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	binData, err := updater.ExtractPublishedBinary(archiveData, c.goos())
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	exePath, err := c.executable()()
	if err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	if c.goos() == "windows" {
		if err := updater.WritePendingUpdate(exePath, binData); err != nil {
			c.writeError(c.Err, err)
			return 1
		}
		_, _ = fmt.Fprintf(c.Out, "downloaded %s to %s.new\n", rel.TagName, exePath)
		_, _ = fmt.Fprintf(c.Out, "close any running klvtool instances, then replace %s with the new file.\n", exePath)
		return 0
	}
	if err := updater.AtomicReplaceExecutable(exePath, binData); err != nil {
		c.writeError(c.Err, err)
		return 1
	}
	_, _ = fmt.Fprintf(c.Out, "updated in place to %s\n", rel.TagName)
	return 0
}

func (c *UpdateCommand) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *UpdateCommand) lookPath() func(string) (string, error) {
	if c.LookPath != nil {
		return c.LookPath
	}
	return exec.LookPath
}

func (c *UpdateCommand) executable() func() (string, error) {
	if c.Executable != nil {
		return c.Executable
	}
	return os.Executable
}

func (c *UpdateCommand) goos() string {
	if c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *UpdateCommand) goarch() string {
	if c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

func (c *UpdateCommand) goModule() string {
	if c.GoModule != "" {
		return c.GoModule
	}
	return goInstallModule
}

func (c *UpdateCommand) runGo() func(ctx context.Context, goBin string, args []string) ([]byte, []byte, error) {
	if c.RunGo != nil {
		return c.RunGo
	}
	return defaultRunGo
}

func (c *UpdateCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
}

func (c *UpdateCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool update [--dry-run] [--prefer-binary]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Update klvtool to the latest GitHub release.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Flags:")
	_, _ = fmt.Fprintln(w, "  --dry-run        Print the chosen strategy without installing")
	_, _ = fmt.Fprintln(w, "  --prefer-binary  Download the release archive even if go is on PATH")
}
