package commanddef

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRenderHelp_FullSubcommand(t *testing.T) {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var input, format string
	var strict bool
	var pid int
	fs.StringVar(&input, "input", "", "MPEG-TS input path")
	fs.StringVar(&format, "format", "ndjson", "output format: ndjson, text, csv")
	fs.BoolVar(&strict, "strict", false, "exit non-zero if any error diagnostic fires")
	fs.IntVar(&pid, "pid", 0, "limit decoding to one PID (0 = all)")

	def := CommandDef{
		Name:        "klvtool-decode",
		Subcommand:  "decode",
		UsageLine:   "klvtool decode --input <file.ts> [--pid N]",
		Description: "Decode MISB ST 0601 KLV records from an MPEG-TS file.",
		Examples: []Example{
			{Command: "klvtool decode --input mission.ts --pid 257", Comment: "decode one PID"},
		},
		ExitCodes: []ExitCode{
			{Code: 0, Meaning: "success"},
			{Code: 1, Meaning: "decode failed"},
			{Code: 2, Meaning: "invalid usage"},
		},
		RequiredTools: []string{"ffmpeg", "ffprobe"},
		SeeAlso: []SeeAlsoRef{
			{Name: "klvtool-inspect", Section: 1},
			{Name: "klvtool", Section: 1},
		},
	}

	var buf bytes.Buffer
	RenderHelp(def, fs, &buf)
	out := buf.String()

	for _, want := range []string{
		"Usage: klvtool decode --input <file.ts> [--pid N]",
		"Decode MISB ST 0601 KLV records",
		"Flags:",
		"--format",
		"(default ndjson)",
		"--strict",
		"--pid",
		"Examples:",
		"# decode one PID",
		"klvtool decode --input mission.ts --pid 257",
		"Required tools: ffmpeg, ffprobe",
		"Exit status:",
		"  0  success",
		"  2  invalid usage",
		"See also: klvtool-inspect(1), klvtool(1)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// Defaults that are not meaningful (empty / false / 0) should not show.
	for _, mustNotContain := range []string{
		"(default )", "(default false)", "(default 0)",
	} {
		if strings.Contains(out, mustNotContain) {
			t.Errorf("output should not contain %q:\n%s", mustNotContain, out)
		}
	}
}

func TestRenderHelp_RootWithSubcommands(t *testing.T) {
	def := CommandDef{
		Name:        "klvtool",
		UsageLine:   "klvtool [command] [--help|-h]",
		Description: "CLI for inspecting MPEG-TS streams and decoding MISB KLV.",
		Subcommands: []SubcommandRef{
			{Name: "diagnose", Synopsis: "Run the full diagnostic pipeline."},
			{Name: "decode", Synopsis: "Decode MISB ST 0601 KLV records."},
		},
	}

	var buf bytes.Buffer
	RenderHelp(def, nil, &buf)
	out := buf.String()

	for _, want := range []string{
		"Commands:",
		"diagnose  Run the full diagnostic pipeline.",
		"decode    Decode MISB ST 0601 KLV records.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// No flagset → no Flags section.
	if strings.Contains(out, "Flags:") {
		t.Errorf("root help should not have Flags: section:\n%s", out)
	}
}

func TestRenderHelp_NilWriter_NoPanic(t *testing.T) {
	RenderHelp(CommandDef{UsageLine: "klvtool foo"}, nil, nil)
}
