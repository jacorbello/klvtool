package commanddef

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ManOpts carries the runtime values stamped into the .TH header. They are
// inputs to the generator (CLI flags, config), not properties of the command.
type ManOpts struct {
	Version string
	Date    string
}

// RenderMan writes a UNIX man page for one command in roff format. Sections
// are emitted in conventional order; empty sections are skipped. The output
// is suitable for `man -l` / `mandoc -T lint`.
func RenderMan(def CommandDef, fs *flag.FlagSet, opts ManOpts, w io.Writer) {
	if w == nil {
		return
	}

	titleName := strings.ToUpper(def.Name)
	versionLabel := opts.Version
	if versionLabel == "" {
		versionLabel = "dev"
	}
	date := opts.Date
	if date == "" {
		date = "1970-01-01"
	}
	fmt.Fprintf(w, ".TH %s 1 %q %q %q\n", titleName, date, "klvtool "+versionLabel, "User Commands")

	writeSection(w, "NAME")
	if def.Synopsis != "" {
		fmt.Fprintf(w, "%s \\- %s\n", roffEscape(def.Name), roffEscape(def.Synopsis))
	} else {
		fmt.Fprintf(w, "%s\n", roffEscape(def.Name))
	}

	if def.UsageLine != "" {
		writeSection(w, "SYNOPSIS")
		fmt.Fprintln(w, roffEscape(def.UsageLine))
	}

	if def.Description != "" {
		writeSection(w, "DESCRIPTION")
		writeParagraphs(w, def.Description)
	}

	if len(def.Subcommands) > 0 {
		writeSection(w, "COMMANDS")
		for _, sc := range def.Subcommands {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fBklvtool\\-%s\\fR(1)\n", roffEscape(sc.Name))
			fmt.Fprintln(w, roffEscape(sc.Synopsis))
		}
	}

	if fs != nil && hasFlags(fs) {
		writeSection(w, "OPTIONS")
		writeOptions(w, fs)
	}

	if len(def.Workflows) > 0 {
		writeSection(w, "WORKFLOWS")
		for _, wf := range def.Workflows {
			fmt.Fprintf(w, ".SS %s\n", roffEscape(wf.Title))
			if wf.When != "" {
				writeParagraphs(w, wf.When)
			}
			for i, st := range wf.Steps {
				if i > 0 || wf.When != "" {
					fmt.Fprintln(w, ".PP")
				}
				fmt.Fprintln(w, ".RS 4")
				fmt.Fprintln(w, ".nf")
				fmt.Fprintf(w, "$ %s\n", roffLiteral(st.Command))
				fmt.Fprintln(w, ".fi")
				fmt.Fprintln(w, ".RE")
				if st.Explain != "" {
					fmt.Fprintln(w, roffEscape(st.Explain))
				}
			}
		}
	}

	if def.OutputFormat != nil {
		writeSection(w, "OUTPUT FORMAT")
		of := def.OutputFormat
		if of.Format != "" {
			writeParagraphs(w, of.Format)
		}
		if len(of.Fields) > 0 {
			fmt.Fprintln(w, ".SS Fields")
			for _, f := range of.Fields {
				fmt.Fprintln(w, ".TP")
				header := fmt.Sprintf("\\fB%s\\fR", roffEscape(f.Name))
				if f.Type != "" {
					header += fmt.Sprintf(" (%s", roffEscape(f.Type))
					if f.Units != "" {
						header += "; " + roffEscape(f.Units)
					}
					header += ")"
				}
				fmt.Fprintln(w, header)
				if f.Notes != "" {
					fmt.Fprintln(w, roffEscape(f.Notes))
				}
			}
		}
		if of.TimeSemantics != "" {
			fmt.Fprintln(w, ".SS Time semantics")
			writeParagraphs(w, of.TimeSemantics)
		}
		if of.Stability != "" {
			fmt.Fprintln(w, ".SS Schema stability")
			writeParagraphs(w, of.Stability)
		}
	}

	if len(def.MISBTagSummary) > 0 {
		fmt.Fprintln(w, ".SS Common MISB ST 0601 tags")
		for _, t := range def.MISBTagSummary {
			fmt.Fprintln(w, ".TP")
			header := fmt.Sprintf("\\fBTag %d \\(em %s\\fR", t.Number, roffEscape(t.Name))
			if t.Units != "" {
				header += fmt.Sprintf(" (%s)", roffEscape(t.Units))
			}
			fmt.Fprintln(w, header)
			if t.Notes != "" {
				fmt.Fprintln(w, roffEscape(t.Notes))
			}
		}
	}

	if len(def.Examples) > 0 {
		writeSection(w, "EXAMPLES")
		for i, ex := range def.Examples {
			if i > 0 {
				fmt.Fprintln(w, ".PP")
			}
			if ex.Comment != "" {
				fmt.Fprintln(w, roffEscape(ex.Comment))
			}
			fmt.Fprintln(w, ".RS 4")
			fmt.Fprintln(w, ".nf")
			fmt.Fprintf(w, "$ %s\n", roffLiteral(ex.Command))
			fmt.Fprintln(w, ".fi")
			fmt.Fprintln(w, ".RE")
		}
	}

	if len(def.Troubleshooting) > 0 {
		writeSection(w, "TROUBLESHOOTING")
		for _, t := range def.Troubleshooting {
			fmt.Fprintf(w, ".SS %s\n", roffEscape(t.Symptom))
			if t.LikelyCause != "" {
				fmt.Fprintln(w, "\\fBLikely cause:\\fR")
				fmt.Fprintln(w, roffEscape(t.LikelyCause))
				fmt.Fprintln(w, ".PP")
			}
			if t.Action != "" {
				fmt.Fprintln(w, "\\fBAction:\\fR")
				fmt.Fprintln(w, roffEscape(t.Action))
			}
		}
	}

	if len(def.Glossary) > 0 {
		writeSection(w, "GLOSSARY")
		for _, g := range def.Glossary {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fB%s\\fR\n", roffEscape(g.Term))
			fmt.Fprintln(w, roffEscape(g.Definition))
		}
	}

	if len(def.ExitCodes) > 0 {
		writeSection(w, "EXIT STATUS")
		for _, ec := range def.ExitCodes {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fB%d\\fR\n", ec.Code)
			fmt.Fprintln(w, roffEscape(ec.Meaning))
		}
	}

	if len(def.EnvVars) > 0 {
		writeSection(w, "ENVIRONMENT")
		for _, ev := range def.EnvVars {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fB%s\\fR\n", roffEscape(ev.Name))
			fmt.Fprintln(w, roffEscape(ev.Description))
		}
	}

	if len(def.Files) > 0 {
		writeSection(w, "FILES")
		for _, f := range def.Files {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fB%s\\fR\n", roffEscape(f.Path))
			fmt.Fprintln(w, roffEscape(f.Description))
		}
	}

	if len(def.RequiredTools) > 0 {
		writeSection(w, "REQUIRED TOOLS")
		fmt.Fprintln(w, roffEscape(strings.Join(def.RequiredTools, ", ")))
	}

	if len(def.SeeAlso) > 0 {
		writeSection(w, "SEE ALSO")
		parts := make([]string, 0, len(def.SeeAlso))
		for _, ref := range def.SeeAlso {
			parts = append(parts, fmt.Sprintf("\\fB%s\\fR(%d)", roffEscape(ref.Name), ref.Section))
		}
		fmt.Fprintln(w, strings.Join(parts, ",\n"))
	}

	if len(def.ExternalRefs) > 0 {
		writeSection(w, "EXTERNAL REFERENCES")
		for _, ref := range def.ExternalRefs {
			fmt.Fprintln(w, ".TP")
			fmt.Fprintf(w, "\\fB%s\\fR\n", roffEscape(ref.Title))
			fmt.Fprintln(w, ref.URL)
		}
	}
}

func writeSection(w io.Writer, name string) {
	fmt.Fprintf(w, ".SH %q\n", name)
}

// writeParagraphs splits on blank lines and emits each paragraph separated by
// .PP. Single line breaks within a paragraph are preserved as soft breaks
// (.br) so analyst-written prose with intentional line endings doesn't get
// mangled.
func writeParagraphs(w io.Writer, text string) {
	paras := strings.Split(text, "\n\n")
	for i, p := range paras {
		if i > 0 {
			fmt.Fprintln(w, ".PP")
		}
		lines := strings.Split(p, "\n")
		for j, line := range lines {
			if j > 0 {
				fmt.Fprintln(w, ".br")
			}
			fmt.Fprintln(w, roffEscape(line))
		}
	}
}

func writeOptions(w io.Writer, fs *flag.FlagSet) {
	type row struct{ name, usage, def string }
	var rows []row
	fs.VisitAll(func(f *flag.Flag) {
		rows = append(rows, row{name: f.Name, usage: f.Usage, def: defaultIfMeaningful(f.DefValue)})
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	for _, r := range rows {
		fmt.Fprintln(w, ".TP")
		header := fmt.Sprintf("\\fB\\-\\-%s\\fR", roffEscape(r.name))
		if r.def != "" {
			header += fmt.Sprintf(" \\fI(default %s)\\fR", roffEscape(r.def))
		}
		fmt.Fprintln(w, header)
		fmt.Fprintln(w, roffEscape(r.usage))
	}
}

// roffEscape escapes user-supplied text for inclusion in a roff document.
// The order matters: backslashes first to avoid double-escaping our own
// emitted control sequences. Hyphens become \- so they render as ASCII
// hyphens rather than soft-hyphens.
func roffEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "-", "\\-")
	return s
}

// roffLiteral is for inside .nf/.fi blocks (code samples). Backslashes still
// need to be doubled, but hyphens are intentionally left alone — a shell
// invocation with `--input` is more readable than `\-\-input` when rendered
// by `man`. (Modern groff renders bare hyphens correctly inside .nf blocks.)
func roffLiteral(s string) string {
	return strings.ReplaceAll(s, "\\", "\\\\")
}
