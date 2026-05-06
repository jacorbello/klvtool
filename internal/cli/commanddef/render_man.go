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
	rw := &rendW{w: w}

	titleName := strings.ToUpper(def.Name)
	versionLabel := opts.Version
	if versionLabel == "" {
		versionLabel = "dev"
	}
	date := opts.Date
	if date == "" {
		date = "1970-01-01"
	}
	rw.printf(".TH %s 1 %q %q %q\n", titleName, date, "klvtool "+versionLabel, "User Commands")

	writeSection(rw, "NAME")
	if def.Synopsis != "" {
		rw.printf("%s \\- %s\n", roffEscape(def.Name), roffEscape(def.Synopsis))
	} else {
		rw.printf("%s\n", roffEscape(def.Name))
	}

	if def.UsageLine != "" {
		writeSection(rw, "SYNOPSIS")
		rw.println(roffEscape(def.UsageLine))
	}

	if def.Description != "" {
		writeSection(rw, "DESCRIPTION")
		writeParagraphs(rw, def.Description)
	}

	if len(def.Subcommands) > 0 {
		writeSection(rw, "COMMANDS")
		for _, sc := range def.Subcommands {
			rw.println(".TP")
			rw.printf("\\fBklvtool\\-%s\\fR(1)\n", roffEscape(sc.Name))
			rw.println(roffEscape(sc.Synopsis))
		}
	}

	if fs != nil && hasFlags(fs) {
		writeSection(rw, "OPTIONS")
		writeOptions(rw, fs)
	}

	if len(def.Workflows) > 0 {
		writeSection(rw, "WORKFLOWS")
		for _, wf := range def.Workflows {
			rw.printf(".SS %s\n", roffEscape(wf.Title))
			if wf.When != "" {
				writeParagraphs(rw, wf.When)
			}
			for i, st := range wf.Steps {
				if i > 0 || wf.When != "" {
					rw.println(".PP")
				}
				rw.println(".RS 4")
				rw.println(".nf")
				rw.printf("$ %s\n", roffLiteral(st.Command))
				rw.println(".fi")
				rw.println(".RE")
				if st.Explain != "" {
					rw.println(roffEscape(st.Explain))
				}
			}
		}
	}

	if def.OutputFormat != nil {
		writeSection(rw, "OUTPUT FORMAT")
		of := def.OutputFormat
		if of.Format != "" {
			writeParagraphs(rw, of.Format)
		}
		if len(of.Fields) > 0 {
			rw.println(".SS Fields")
			for _, f := range of.Fields {
				rw.println(".TP")
				header := fmt.Sprintf("\\fB%s\\fR", roffEscape(f.Name))
				if f.Type != "" {
					header += fmt.Sprintf(" (%s", roffEscape(f.Type))
					if f.Units != "" {
						header += "; " + roffEscape(f.Units)
					}
					header += ")"
				}
				rw.println(header)
				if f.Notes != "" {
					rw.println(roffEscape(f.Notes))
				}
			}
		}
		if of.TimeSemantics != "" {
			rw.println(".SS Time semantics")
			writeParagraphs(rw, of.TimeSemantics)
		}
		if of.Stability != "" {
			rw.println(".SS Schema stability")
			writeParagraphs(rw, of.Stability)
		}
	}

	if len(def.MISBTagSummary) > 0 {
		rw.println(".SS Common MISB ST 0601 tags")
		for _, t := range def.MISBTagSummary {
			rw.println(".TP")
			header := fmt.Sprintf("\\fBTag %d \\(em %s\\fR", t.Number, roffEscape(t.Name))
			if t.Units != "" {
				header += fmt.Sprintf(" (%s)", roffEscape(t.Units))
			}
			rw.println(header)
			if t.Notes != "" {
				rw.println(roffEscape(t.Notes))
			}
		}
	}

	if len(def.Examples) > 0 {
		writeSection(rw, "EXAMPLES")
		for i, ex := range def.Examples {
			if i > 0 {
				rw.println(".PP")
			}
			if ex.Comment != "" {
				rw.println(roffEscape(ex.Comment))
			}
			rw.println(".RS 4")
			rw.println(".nf")
			rw.printf("$ %s\n", roffLiteral(ex.Command))
			rw.println(".fi")
			rw.println(".RE")
		}
	}

	if len(def.Troubleshooting) > 0 {
		writeSection(rw, "TROUBLESHOOTING")
		for _, t := range def.Troubleshooting {
			rw.printf(".SS %s\n", roffEscape(t.Symptom))
			if t.LikelyCause != "" {
				rw.println("\\fBLikely cause:\\fR")
				rw.println(roffEscape(t.LikelyCause))
				rw.println(".PP")
			}
			if t.Action != "" {
				rw.println("\\fBAction:\\fR")
				rw.println(roffEscape(t.Action))
			}
		}
	}

	if len(def.Glossary) > 0 {
		writeSection(rw, "GLOSSARY")
		for _, g := range def.Glossary {
			rw.println(".TP")
			rw.printf("\\fB%s\\fR\n", roffEscape(g.Term))
			rw.println(roffEscape(g.Definition))
		}
	}

	if len(def.ExitCodes) > 0 {
		writeSection(rw, "EXIT STATUS")
		for _, ec := range def.ExitCodes {
			rw.println(".TP")
			rw.printf("\\fB%d\\fR\n", ec.Code)
			rw.println(roffEscape(ec.Meaning))
		}
	}

	if len(def.EnvVars) > 0 {
		writeSection(rw, "ENVIRONMENT")
		for _, ev := range def.EnvVars {
			rw.println(".TP")
			rw.printf("\\fB%s\\fR\n", roffEscape(ev.Name))
			rw.println(roffEscape(ev.Description))
		}
	}

	if len(def.Files) > 0 {
		writeSection(rw, "FILES")
		for _, f := range def.Files {
			rw.println(".TP")
			rw.printf("\\fB%s\\fR\n", roffEscape(f.Path))
			rw.println(roffEscape(f.Description))
		}
	}

	if len(def.RequiredTools) > 0 {
		writeSection(rw, "REQUIRED TOOLS")
		rw.println(roffEscape(strings.Join(def.RequiredTools, ", ")))
	}

	if len(def.SeeAlso) > 0 {
		writeSection(rw, "SEE ALSO")
		parts := make([]string, 0, len(def.SeeAlso))
		for _, ref := range def.SeeAlso {
			parts = append(parts, fmt.Sprintf("\\fB%s\\fR(%d)", roffEscape(ref.Name), ref.Section))
		}
		rw.println(strings.Join(parts, ",\n"))
	}

	if len(def.ExternalRefs) > 0 {
		writeSection(rw, "EXTERNAL REFERENCES")
		for _, ref := range def.ExternalRefs {
			rw.println(".TP")
			rw.printf("\\fB%s\\fR\n", roffEscape(ref.Title))
			rw.println(ref.URL)
		}
	}
}

func writeSection(rw *rendW, name string) {
	rw.printf(".SH %q\n", name)
}

// writeParagraphs splits on blank lines and emits each paragraph separated by
// .PP. Single line breaks within a paragraph are preserved as soft breaks
// (.br) so analyst-written prose with intentional line endings doesn't get
// mangled.
func writeParagraphs(rw *rendW, text string) {
	paras := strings.Split(text, "\n\n")
	for i, p := range paras {
		if i > 0 {
			rw.println(".PP")
		}
		lines := strings.Split(p, "\n")
		for j, line := range lines {
			if j > 0 {
				rw.println(".br")
			}
			rw.println(roffEscape(line))
		}
	}
}

func writeOptions(rw *rendW, fs *flag.FlagSet) {
	type row struct{ name, usage, def string }
	var rows []row
	fs.VisitAll(func(f *flag.Flag) {
		rows = append(rows, row{name: f.Name, usage: f.Usage, def: defaultIfMeaningful(f.DefValue)})
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	for _, r := range rows {
		rw.println(".TP")
		header := fmt.Sprintf("\\fB\\-\\-%s\\fR", roffEscape(r.name))
		if r.def != "" {
			header += fmt.Sprintf(" \\fI(default %s)\\fR", roffEscape(r.def))
		}
		rw.println(header)
		rw.println(roffEscape(r.usage))
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
