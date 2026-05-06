// Command gen-manpages emits roff man pages for klvtool by walking the live
// command structures. Both --help and the generated pages render from the
// same CommandDef + flag.FlagSet, so they cannot drift.
//
// Usage:
//
//	go run ./cmd/gen-manpages -out man/man1
//	go run ./cmd/gen-manpages -out man/man1 -date 1970-01-01
//
// The Makefile wires this into `make man` (real date) and `make man-check`
// (fixed epoch date for deterministic CI diffing).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jacorbello/klvtool/internal/cli"
	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/version"
)

func main() {
	var outDir, date, ver string
	flag.StringVar(&outDir, "out", "man/man1", "output directory for generated .1 files")
	flag.StringVar(&date, "date", "", "date stamped into the .TH header (YYYY-MM-DD); defaults to today")
	flag.StringVar(&ver, "version", "", "version stamped into the .TH header; defaults to internal/version")
	flag.Parse()

	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	if ver == "" {
		ver = version.String()
	}
	opts := commanddef.ManOpts{Version: ver, Date: date}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail("create out dir: %v", err)
	}

	root := cli.NewRootCommand()

	// Root page: klvtool.1 (no FlagSet — root has no flags of its own).
	if err := writePage(filepath.Join(outDir, "klvtool.1"), root.Definition(), nil, opts); err != nil {
		fail("write klvtool.1: %v", err)
	}

	// One page per subcommand. The (Def, FlagSet) pairs come from the cli
	// package so each FlagSet is the exact one Execute would build at
	// runtime — generated OPTIONS cannot disagree with --help.
	for _, sc := range cli.SubcommandFlagSets() {
		path := filepath.Join(outDir, fmt.Sprintf("klvtool-%s.1", sc.Def.Subcommand))
		if err := writePage(path, sc.Def, sc.FS, opts); err != nil {
			fail("write %s: %v", path, err)
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-manpages: "+format+"\n", args...)
	os.Exit(1)
}

func writePage(path string, def commanddef.CommandDef, fs *flag.FlagSet, opts commanddef.ManOpts) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	commanddef.RenderMan(def, fs, opts, f)
	return f.Close()
}
