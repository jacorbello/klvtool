package cli

import (
	"fmt"
	"io"
	"os"

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
	_, _ = fmt.Fprintf(w, "Usage: %s [command] [--help|-h]\n", c.Use)
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Version: %s\n", c.Version)
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "CLI for inspecting MPEG-TS streams, extracting KLV payloads, packetizing raw checkpoints, and decoding MISB metadata.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  version     Print version information.")
	_, _ = fmt.Fprintln(w, "  update      Update to the latest GitHub release.")
	_, _ = fmt.Fprintln(w, "  doctor      Check backend availability and environment health.")
	_, _ = fmt.Fprintln(w, "  extract     Extract payloads and write manifest output.")
	_, _ = fmt.Fprintln(w, "  inspect     Inspect MPEG-TS stream inventory and diagnostics.")
	_, _ = fmt.Fprintln(w, "  decode      Decode MISB ST 0601 KLV records to NDJSON, text, or CSV.")
	_, _ = fmt.Fprintln(w, "  packetize   Replay raw checkpoints and write packet output.")
	_, _ = fmt.Fprintln(w, "  diagnose    Run the full diagnostic pipeline on an input file.")
	_, _ = fmt.Fprintln(w, "  completion  Generate shell completion scripts.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Common workflows:")
	_, _ = fmt.Fprintln(w, "  inspect -> decode")
	_, _ = fmt.Fprintln(w, "    Find likely metadata PIDs, then decode only that stream.")
	_, _ = fmt.Fprintln(w, "  extract -> packetize")
	_, _ = fmt.Fprintln(w, "    Capture raw payload artifacts, then inspect KLV packet framing.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Required tools:")
	_, _ = fmt.Fprintln(w, "  ffmpeg:  ffmpeg, ffprobe")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Install:")
	_, _ = fmt.Fprintln(w, "  go install github.com/jacorbello/klvtool/cmd/klvtool@latest")
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
