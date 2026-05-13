package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/version"
)

const usageExitCode = 2

type RootCommand struct {
	Use        string
	Version    string
	Out        io.Writer
	Err        io.Writer
	Doctor     *DoctorCommand
	Extract    *ExtractCommand
	Inspect    *InspectCommand
	Decode     *DecodeCommand
	Packetize  *PacketizeCommand
	Diagnose   *DiagnoseCommand
	Record     *RecordCommand
	Completion *CompletionCommand
	VersionCmd *VersionCommand
	Update     *UpdateCommand
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use:        "klvtool",
		Version:    version.String(),
		Out:        os.Stdout,
		Err:        os.Stderr,
		Doctor:     NewDoctorCommand(),
		Extract:    NewExtractCommand(),
		Inspect:    NewInspectCommand(),
		Decode:     NewDecodeCommand(),
		Packetize:  NewPacketizeCommand(),
		Diagnose:   NewDiagnoseCommand(),
		Record:     NewRecordCommand(),
		Completion: NewCompletionCommand(),
		VersionCmd: NewVersionCommand(),
		Update:     NewUpdateCommand(),
	}
}

func (c *RootCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 0 {
		c.writeUsage(c.Err)
		return usageExitCode
	}
	if len(args) == 1 && (isHelpArg(args[0]) || args[0] == "help") {
		c.writeUsage(c.Out)
		return 0
	}
	if len(args) > 1 && args[0] == "help" {
		c.writeUnsupportedArgs(args[1:])
		return usageExitCode
	}
	if len(args) > 0 && args[0] == "version" {
		return c.versionCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "update" {
		return c.updateCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "doctor" {
		return c.doctorCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "extract" {
		return c.extractCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "inspect" {
		return c.inspectCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "decode" {
		return c.decodeCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "packetize" {
		return c.packetizeCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "diagnose" {
		return c.diagnoseCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "record" {
		return c.recordCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "completion" {
		return c.completionCommand().Execute(args[1:])
	}
	c.writeUnsupportedArgs(args)
	return usageExitCode
}

func Main() int {
	return NewRootCommand().Execute(os.Args[1:])
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func (c *RootCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	commanddef.RenderHelp(c.rootDef(), nil, w)
	if c != nil && c.Version != "" {
		_, _ = fmt.Fprintf(w, "\nVersion: %s\n", c.Version)
	}
}

// Definition returns the root CommandDef. The man-page generator calls this
// then walks each subcommand's Definition() to emit per-command pages.
func (c *RootCommand) Definition() commanddef.CommandDef { return c.rootDef() }

// SubcommandDefs returns each subcommand's CommandDef in stable order.
// Used by the man-page generator.
func (c *RootCommand) SubcommandDefs() []commanddef.CommandDef {
	return []commanddef.CommandDef{
		c.versionCommand().Definition(),
		c.updateCommand().Definition(),
		c.doctorCommand().Definition(),
		c.extractCommand().Definition(),
		c.inspectCommand().Definition(),
		c.decodeCommand().Definition(),
		c.packetizeCommand().Definition(),
		c.diagnoseCommand().Definition(),
		c.recordCommand().Definition(),
		c.completionCommand().Definition(),
	}
}

// rootDef assembles the top-level CommandDef. The COMMANDS list is built from
// each subcommand's Synopsis at render time so the root page never disagrees
// with the per-command page about what a command does.
func (c *RootCommand) rootDef() commanddef.CommandDef {
	subs := []commanddef.SubcommandRef{}
	for _, def := range c.SubcommandDefs() {
		subs = append(subs, commanddef.SubcommandRef{Name: def.Subcommand, Synopsis: def.Synopsis})
	}
	def := rootDef
	def.Subcommands = subs
	return def
}

// rootDef holds the static documentation for `klvtool` without a subcommand.
// COMMANDS is populated at render time from the live subcommand Defs.
var rootDef = commanddef.CommandDef{
	Name:        "klvtool",
	UsageLine:   "klvtool <command> [flags]",
	Synopsis:    "CLI for inspecting MPEG-TS streams, extracting KLV payloads, packetizing raw checkpoints, and decoding MISB metadata.",
	Description: "klvtool is a Go CLI for working with MPEG-TS video assets and the KLV metadata streams carried inside them. The primary audience is intelligence analysts triaging full-motion video from sensor platforms (UAS, manned ISR) and engineers integrating KLV pipelines.\n\nThe canonical entry point for triaging an unknown file is `klvtool diagnose --input <file.ts>` — it runs the full health check, transport inspection, video bitstream analysis, and KLV decode in one pass and points to a remediation when something fails.",
	Workflows: []commanddef.Workflow{
		{
			Title: "Triage an unknown mission file",
			When:  "You just received a TS file and need to know whether it is usable, whether the metadata is intact, and what to do if something looks wrong.",
			Steps: []commanddef.WorkflowStep{
				{Command: "klvtool diagnose --input mission.ts", Explain: "runs health check, transport inspect, video analysis, and KLV decode; produces a single consolidated report with remediation guidance for any failed stage"},
				{Command: "klvtool decode --input mission.ts --pid <PID> --format text", Explain: "if diagnose surfaces decode errors on a specific PID, this shows the per-packet diagnostics in pretty text view"},
			},
		},
		{
			Title: "Geo-locate a frame at a known timestamp",
			When:  "You need sensor latitude/longitude (or frame-center geo) for a specific moment in the stream and intend to hand the result off to GIS or a downstream pipeline.",
			Steps: []commanddef.WorkflowStep{
				{Command: "klvtool inspect --input mission.ts", Explain: "find the metadata PID — the row labeled \"Likely metadata\" in pretty view"},
				{Command: "klvtool decode --input mission.ts --pid <PID> --format ndjson --out frames.ndjson", Explain: "decode only that PID into stable NDJSON; one record per KLV packet, with tag 2 carrying the wall-clock timestamp"},
				{Command: "jq 'select(.items[] | .tag == 2 and .value >= \"2024-01-15T14:23:45.000000Z\")' frames.ndjson", Explain: "filter to your timestamp; the surrounding record carries tags 13/14 (sensor lat/lon) and 23/24 (frame-center lat/lon)"},
			},
		},
		{
			Title: "Forensic recovery from corrupt transport",
			When:  "Diagnose reports KLV decode errors or framing problems and re-pulling the file is not an option.",
			Steps: []commanddef.WorkflowStep{
				{Command: "klvtool extract --input mission.ts --out ./mission-extract", Explain: "capture raw KLV payloads and a manifest with SHA-256 hashes and packet-level transport metadata for chain of custody"},
				{Command: "klvtool packetize --input ./mission-extract --out ./mission-packets --mode best-effort", Explain: "scan past malformed packets; per-record diagnostics include byte offsets where the wire format diverged"},
				{Command: "klvtool decode --input mission.ts --pid <PID> --raw --strict", Explain: "with --raw, NDJSON includes base64 of the offending bytes — useful when escalating to platform / sensor engineering"},
			},
		},
	},
	Glossary: []commanddef.GlossaryEntry{
		{Term: "Raw checkpoint", Definition: "the per-record artifact written by `klvtool extract`: the full PES payload bytes plus transport metadata. Input format for `klvtool packetize`."},
		{Term: "Packet checkpoint", Definition: "the parsed-KLV-packet artifact written by `klvtool packetize`: per-packet key/length/value byte ranges plus parser diagnostics. One JSON file per source raw checkpoint."},
		{Term: "Strict vs best-effort packetize", Definition: "strict aborts the record on the first malformed KLV packet; best-effort scans forward, recovers what it can, and surfaces diagnostics with byte offsets. Use best-effort for forensic recovery on suspect input."},
		{Term: "Structural validation", Definition: "klvtool's per-tag checks (length, range, encoding) against the registered MISB Local Set spec. Failures appear as error-severity diagnostics on the relevant packet."},
		{Term: "View modes (auto/pretty/raw)", Definition: "auto picks pretty when stdout is a TTY and the format is human-oriented, raw otherwise. Pretty is for humans (color, multi-line); raw is for piping (single-line, no escapes)."},
		{Term: "Likely metadata stream", Definition: "a PID whose stream type is one of the data/private types commonly used to carry KLV. `klvtool inspect` highlights these in pretty view; `klvtool diagnose` decodes them automatically."},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "success"},
		{Code: 1, Meaning: "command-specific failure (see the per-command page)"},
		{Code: 2, Meaning: "invalid usage"},
	},
	EnvVars: []commanddef.EnvVar{
		{Name: "NO_COLOR", Description: "disable ANSI color in pretty output across every subcommand"},
	},
	RequiredTools: []string{"ffmpeg", "ffprobe"},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool-doctor", Section: 1},
		{Name: "klvtool-diagnose", Section: 1},
		{Name: "klvtool-inspect", Section: 1},
		{Name: "klvtool-decode", Section: 1},
		{Name: "klvtool-extract", Section: 1},
		{Name: "klvtool-packetize", Section: 1},
		{Name: "klvtool-record", Section: 1},
		{Name: "klvtool-version", Section: 1},
		{Name: "klvtool-update", Section: 1},
		{Name: "klvtool-completion", Section: 1},
	},
	ExternalRefs: []commanddef.ExternalRef{
		{Title: "klvtool source and releases", URL: "https://github.com/jacorbello/klvtool"},
		{Title: "MISB ST 0601.19 — UAS Datalink Local Set", URL: "https://nsgreg.nga.mil/doc/view?i=5337"},
		{Title: "MISB ST 1402 — MPEG-2 Transport of Compressed Video Metadata", URL: "https://nsgreg.nga.mil/doc/view?i=5135"},
		{Title: "ITU-T H.222.0 / ISO/IEC 13818-1 — MPEG-TS systems layer", URL: "https://www.itu.int/rec/T-REC-H.222.0"},
	},
}

func (c *RootCommand) writeUnsupportedArgs(args []string) {
	if c.Err == nil {
		return
	}
	c.writeUsage(c.Err)
	_, _ = fmt.Fprintf(c.Err, "error: unsupported arguments: %v\n", args)
}

func (c *RootCommand) doctorCommand() *DoctorCommand {
	if c == nil {
		return NewDoctorCommand()
	}
	doctor := c.Doctor
	if doctor == nil {
		doctor = NewDoctorCommand()
		c.Doctor = doctor
	}
	doctor.Out = c.Out
	doctor.Err = c.Err
	doctor.Version = c.Version
	return doctor
}

func (c *RootCommand) extractCommand() *ExtractCommand {
	if c == nil {
		return NewExtractCommand()
	}
	extractCmd := c.Extract
	if extractCmd == nil {
		extractCmd = NewExtractCommand()
		c.Extract = extractCmd
	}
	extractCmd.Out = c.Out
	extractCmd.Err = c.Err
	return extractCmd
}

func (c *RootCommand) inspectCommand() *InspectCommand {
	if c == nil {
		return NewInspectCommand()
	}
	inspectCmd := c.Inspect
	if inspectCmd == nil {
		inspectCmd = NewInspectCommand()
		c.Inspect = inspectCmd
	}
	inspectCmd.Out = c.Out
	inspectCmd.Err = c.Err
	return inspectCmd
}

func (c *RootCommand) decodeCommand() *DecodeCommand {
	if c == nil {
		return NewDecodeCommand()
	}
	decodeCmd := c.Decode
	if decodeCmd == nil {
		decodeCmd = NewDecodeCommand()
		c.Decode = decodeCmd
	}
	decodeCmd.Out = c.Out
	decodeCmd.Err = c.Err
	return decodeCmd
}

func (c *RootCommand) packetizeCommand() *PacketizeCommand {
	if c == nil {
		return NewPacketizeCommand()
	}
	packetizeCmd := c.Packetize
	if packetizeCmd == nil {
		packetizeCmd = NewPacketizeCommand()
		c.Packetize = packetizeCmd
	}
	packetizeCmd.Out = c.Out
	packetizeCmd.Err = c.Err
	return packetizeCmd
}

func (c *RootCommand) diagnoseCommand() *DiagnoseCommand {
	if c == nil {
		return NewDiagnoseCommand()
	}
	d := c.Diagnose
	if d == nil {
		d = NewDiagnoseCommand()
		c.Diagnose = d
	}
	d.Out = c.Out
	d.Err = c.Err
	return d
}

func (c *RootCommand) recordCommand() *RecordCommand {
	if c == nil {
		return NewRecordCommand()
	}
	rec := c.Record
	if rec == nil {
		rec = NewRecordCommand()
		c.Record = rec
	}
	rec.Out = c.Out
	rec.Err = c.Err
	return rec
}

func (c *RootCommand) completionCommand() *CompletionCommand {
	if c == nil {
		return NewCompletionCommand()
	}
	comp := c.Completion
	if comp == nil {
		comp = NewCompletionCommand()
		c.Completion = comp
	}
	comp.Out = c.Out
	comp.Err = c.Err
	return comp
}

func (c *RootCommand) versionCommand() *VersionCommand {
	if c == nil {
		return NewVersionCommand()
	}
	v := c.VersionCmd
	if v == nil {
		v = NewVersionCommand()
		c.VersionCmd = v
	}
	v.Out = c.Out
	v.Err = c.Err
	v.Version = c.Version
	return v
}

func (c *RootCommand) updateCommand() *UpdateCommand {
	if c == nil {
		return NewUpdateCommand()
	}
	u := c.Update
	if u == nil {
		u = NewUpdateCommand()
		c.Update = u
	}
	u.Out = c.Out
	u.Err = c.Err
	u.Version = c.Version
	return u
}
