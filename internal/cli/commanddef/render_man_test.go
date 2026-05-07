package commanddef

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

func TestRenderMan_SubcommandWithEverything(t *testing.T) {
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
		Synopsis:    "Decode MISB ST 0601 KLV records.",
		UsageLine:   "klvtool decode --input <file.ts> [--pid N]",
		Description: "Decode MISB ST 0601 KLV.\n\nSecond paragraph for the body.",
		Examples: []Example{
			{Command: "klvtool decode --input mission.ts --pid 257", Comment: "decode one PID"},
			{Command: "klvtool decode --input mission.ts", Comment: "decode every PID"},
		},
		OutputFormat: &OutputDoc{
			Format: "NDJSON, one record per packet.",
			Fields: []FieldDef{
				{Name: "schema", Type: "string", Notes: "spec URN"},
				{Name: "items[].value", Type: "polymorphic", Units: "varies", Notes: "per-tag type"},
			},
			TimeSemantics: "Tag 2 is RFC 3339 microsecond UTC.",
			Stability:     "Schema is stable within 1.x.",
		},
		MISBTagSummary: []MISBTag{
			{Number: 2, Name: "Precision Time Stamp", Notes: "wall-clock at sample time (UTC)"},
			{Number: 13, Name: "Sensor Latitude", Units: "degrees", Notes: "WGS-84"},
		},
		ExitCodes: []ExitCode{
			{Code: 0, Meaning: "success"},
			{Code: 1, Meaning: "decode failure"},
			{Code: 2, Meaning: "invalid usage"},
		},
		EnvVars: []EnvVar{
			{Name: "NO_COLOR", Description: "disable ANSI color"},
		},
		Files: []FileRef{
			{Path: "<out>", Description: "output destination"},
		},
		RequiredTools: []string{"ffmpeg", "ffprobe"},
		SeeAlso: []SeeAlsoRef{
			{Name: "klvtool", Section: 1},
			{Name: "klvtool-inspect", Section: 1},
		},
		ExternalRefs: []ExternalRef{
			{Title: "MISB ST 0601.19", URL: "https://nsgreg.nga.mil/doc/view?i=5337"},
		},
	}

	got := renderManString(def, fs, ManOpts{Version: "1.2.0", Date: "2026-05-06"})
	checkGolden(t, "decode_full.golden", got)
}

func TestRenderMan_RootWithWorkflowsAndGlossary(t *testing.T) {
	def := CommandDef{
		Name:        "klvtool",
		Synopsis:    "CLI for MPEG-TS and KLV.",
		UsageLine:   "klvtool <command> [flags]",
		Description: "klvtool overview paragraph.",
		Subcommands: []SubcommandRef{
			{Name: "decode", Synopsis: "Decode KLV."},
			{Name: "inspect", Synopsis: "Inspect MPEG-TS."},
		},
		Workflows: []Workflow{
			{
				Title: "Triage a file",
				When:  "You just received a TS file.",
				Steps: []WorkflowStep{
					{Command: "klvtool diagnose --input mission.ts", Explain: "runs every stage"},
				},
			},
		},
		Glossary: []GlossaryEntry{
			{Term: "Raw checkpoint", Definition: "the per-record artifact"},
		},
		ExitCodes: []ExitCode{
			{Code: 0, Meaning: "success"},
		},
		SeeAlso: []SeeAlsoRef{
			{Name: "klvtool-decode", Section: 1},
		},
	}

	got := renderManString(def, nil, ManOpts{Version: "1.2.0", Date: "2026-05-06"})
	checkGolden(t, "root_full.golden", got)
}

func TestRenderMan_DiagnoseTroubleshooting(t *testing.T) {
	def := CommandDef{
		Name:       "klvtool-diagnose",
		Subcommand: "diagnose",
		Synopsis:   "Run the full diagnostic pipeline.",
		UsageLine:  "klvtool diagnose --input <file.ts>",
		Troubleshooting: []TroubleshootingEntry{
			{
				Symptom:     "Stopped at: health check",
				LikelyCause: "ffmpeg not on PATH",
				Action:      "Install ffmpeg and retry",
			},
		},
	}
	got := renderManString(def, nil, ManOpts{Version: "1.2.0", Date: "2026-05-06"})

	for _, want := range []string{
		".SH \"TROUBLESHOOTING\"",
		".SS Stopped at: health check",
		"\\fBLikely cause:\\fR",
		"ffmpeg not on PATH",
		"\\fBAction:\\fR",
		"Install ffmpeg and retry",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderMan_DefaultsAndDate(t *testing.T) {
	def := CommandDef{Name: "klvtool", Synopsis: "test"}
	got := renderManString(def, nil, ManOpts{})
	if !strings.Contains(got, "klvtool dev") {
		t.Errorf("expected fallback version 'dev' in TH header:\n%s", got)
	}
	if !strings.Contains(got, "1970-01-01") {
		t.Errorf("expected fallback date '1970-01-01' in TH header:\n%s", got)
	}
}

func TestRoffEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"foo-bar", "foo\\-bar"},
		{"a\\b", "a\\\\b"},
		{"--input", "\\-\\-input"},
		// Backslashes must be escaped before hyphens, otherwise our own
		// emitted \- pairs would get re-mangled.
		{"a\\-b", "a\\\\\\-b"},
	}
	for _, c := range cases {
		if got := roffEscape(c.in); got != c.want {
			t.Errorf("roffEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func renderManString(def CommandDef, fs *flag.FlagSet, opts ManOpts) string {
	var buf bytes.Buffer
	RenderMan(def, fs, opts, &buf)
	return buf.String()
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (re-run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("output does not match golden %s\n--- want ---\n%s\n--- got ---\n%s", path, string(want), got)
	}
}
