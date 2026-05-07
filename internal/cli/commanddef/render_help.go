package commanddef

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// rendW absorbs write errors. Renderers don't return errors, and there is
// nothing meaningful to do if the underlying writer fails mid-stream — the
// caller's io.Writer.Close (or its absence) is the failure surface.
type rendW struct{ w io.Writer }

func (r *rendW) printf(format string, args ...any) {
	if r == nil || r.w == nil {
		return
	}
	_, _ = fmt.Fprintf(r.w, format, args...)
}

func (r *rendW) println(args ...any) {
	if r == nil || r.w == nil {
		return
	}
	_, _ = fmt.Fprintln(r.w, args...)
}

// RenderHelp writes a terse --help block for one command. It is intentionally
// less verbose than the man page: the full prose, workflows, glossary, and
// troubleshooting tables live there. Here we want what an operator typing
// `klvtool decode --help` actually needs in the moment: usage, what it does,
// flags, an example, and where to look for more.
//
// Either def or fs may be zero — empty sections are skipped.
func RenderHelp(def CommandDef, fs *flag.FlagSet, w io.Writer) {
	if w == nil {
		return
	}
	rw := &rendW{w: w}

	if def.UsageLine != "" {
		rw.printf("Usage: %s\n", def.UsageLine)
	}

	if def.Description != "" {
		rw.println()
		writeWrapped(rw, def.Description, "")
	}

	if fs != nil && hasFlags(fs) {
		rw.println()
		rw.println("Flags:")
		writeFlags(rw, fs)
	}

	if len(def.Subcommands) > 0 {
		rw.println()
		rw.println("Commands:")
		width := subcommandColumnWidth(def.Subcommands)
		for _, sc := range def.Subcommands {
			rw.printf("  %-*s  %s\n", width, sc.Name, sc.Synopsis)
		}
	}

	if len(def.Workflows) > 0 {
		rw.println()
		rw.println("Common workflows:")
		for _, wf := range def.Workflows {
			rw.printf("  %s\n", wf.Title)
			steps := commandChain(wf.Steps)
			if steps != "" {
				rw.printf("    %s\n", steps)
			}
		}
		rw.println()
		rw.println("  See klvtool(1) for the full step-by-step walkthrough of each workflow.")
	}

	if len(def.Examples) > 0 {
		rw.println()
		rw.println("Examples:")
		for i, ex := range def.Examples {
			if i > 0 {
				rw.println()
			}
			if ex.Comment != "" {
				rw.printf("  # %s\n", ex.Comment)
			}
			rw.printf("  %s\n", ex.Command)
		}
	}

	if len(def.RequiredTools) > 0 {
		rw.println()
		rw.printf("Required tools: %s\n", strings.Join(def.RequiredTools, ", "))
	}

	if len(def.ExitCodes) > 0 {
		rw.println()
		rw.println("Exit status:")
		for _, ec := range def.ExitCodes {
			rw.printf("  %d  %s\n", ec.Code, ec.Meaning)
		}
	}

	if len(def.SeeAlso) > 0 {
		rw.println()
		rw.printf("See also: %s\n", joinSeeAlso(def.SeeAlso))
	}
}

func hasFlags(fs *flag.FlagSet) bool {
	any := false
	fs.VisitAll(func(*flag.Flag) { any = true })
	return any
}

// writeFlags emits one line per flag, alphabetically. Default values are
// elided when they are empty/false/zero — an operator scanning help should
// see meaningful defaults, not visual noise.
func writeFlags(rw *rendW, fs *flag.FlagSet) {
	type row struct{ name, usage, def string }
	var rows []row
	fs.VisitAll(func(f *flag.Flag) {
		rows = append(rows, row{name: f.Name, usage: f.Usage, def: defaultIfMeaningful(f.DefValue)})
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	width := 0
	for _, r := range rows {
		if l := len(r.name); l > width {
			width = l
		}
	}
	for _, r := range rows {
		line := fmt.Sprintf("  --%-*s  %s", width, r.name, r.usage)
		if r.def != "" {
			line += fmt.Sprintf(" (default %s)", r.def)
		}
		rw.println(line)
	}
}

func defaultIfMeaningful(v string) string {
	switch v {
	case "", "0", "false":
		return ""
	}
	return v
}

func subcommandColumnWidth(subs []SubcommandRef) int {
	w := 0
	for _, s := range subs {
		if l := len(s.Name); l > w {
			w = l
		}
	}
	return w
}

func joinSeeAlso(refs []SeeAlsoRef) string {
	parts := make([]string, 0, len(refs))
	for _, r := range refs {
		parts = append(parts, fmt.Sprintf("%s(%d)", r.Name, r.Section))
	}
	return strings.Join(parts, ", ")
}

// commandChain extracts the leading subcommand name from each step's Command
// and joins them with " -> " so an analyst skimming --help can see the
// command flow at a glance ("inspect -> decode") without the full prose.
func commandChain(steps []WorkflowStep) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		fields := strings.Fields(s.Command)
		// Step commands start with "klvtool <subcommand>". Skip non-klvtool
		// invocations (e.g., a pipeline step like `jq ...`) so the chain
		// summary stays focused on klvtool subcommands.
		if len(fields) >= 2 && fields[0] == "klvtool" {
			parts = append(parts, fields[1])
		}
	}
	return strings.Join(parts, " -> ")
}

// writeWrapped emits text without rewrapping. Paragraph breaks (blank lines)
// are preserved. Indentation prefix is added to every non-blank line.
func writeWrapped(rw *rendW, text, indent string) {
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			rw.println()
			continue
		}
		rw.println(indent + line)
	}
}
