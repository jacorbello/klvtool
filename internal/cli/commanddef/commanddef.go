// Package commanddef defines the structured metadata that drives both --help
// rendering and man-page generation. The design rule: a single CommandDef per
// CLI command is the source of truth for everything the user sees in either
// channel. Flag descriptions stay in *flag.FlagSet so that the runtime parser
// and the documentation cannot disagree.
package commanddef

// CommandDef captures everything we need to know about a CLI command to render
// terse --help output and a full man page. Sections that don't apply to a
// command are left zero-valued; renderers skip empty sections.
type CommandDef struct {
	Name       string
	Subcommand string
	Synopsis   string
	UsageLine  string
	// Description is the multi-paragraph DESCRIPTION body. Paragraphs are
	// separated by blank lines.
	Description string

	Workflows       []Workflow
	Glossary        []GlossaryEntry
	Subcommands     []SubcommandRef
	OutputFormat    *OutputDoc
	Troubleshooting []TroubleshootingEntry
	ExternalRefs    []ExternalRef
	MISBTagSummary  []MISBTag

	Examples      []Example
	ExitCodes     []ExitCode
	EnvVars       []EnvVar
	Files         []FileRef
	SeeAlso       []SeeAlsoRef
	RequiredTools []string
}

// Workflow names a multi-step analyst scenario, rendered on the root man page.
type Workflow struct {
	Title string
	When  string
	Steps []WorkflowStep
}

type WorkflowStep struct {
	Command string
	Explain string
}

// GlossaryEntry defines a klvtool-specific term. Spec terms (KLV, MISB, PID)
// are intentionally omitted — the audience knows them.
type GlossaryEntry struct {
	Term       string
	Definition string
}

// SubcommandRef is a one-line listing for the COMMANDS section on the root
// page. Pulled from each subcommand's CommandDef at render time.
type SubcommandRef struct {
	Name     string
	Synopsis string
}

// OutputDoc describes the schema of a command's machine-readable output. Used
// by downstream pipeline owners (analysts ingesting NDJSON into pandas /
// Splunk / Elastic) to predict field shape.
type OutputDoc struct {
	Format        string
	Fields        []FieldDef
	TimeSemantics string
	Stability     string
}

type FieldDef struct {
	Name  string
	Type  string
	Units string
	Notes string
}

// TroubleshootingEntry maps an observed verdict pattern to a recommended
// remediation. Rendered on the diagnose page.
type TroubleshootingEntry struct {
	Symptom     string
	LikelyCause string
	Action      string
}

// ExternalRef is a non-man-page reference (a spec, a URL).
type ExternalRef struct {
	Title string
	URL   string
}

// MISBTag is one row in the common-tags table on klvtool-decode.1.
type MISBTag struct {
	Number int
	Name   string
	Units  string
	Notes  string
}

// Example is one runnable invocation with a one-line comment.
type Example struct {
	Command string
	Comment string
}

// ExitCode documents one return code.
type ExitCode struct {
	Code    int
	Meaning string
}

// EnvVar documents one environment variable that affects this command.
type EnvVar struct {
	Name        string
	Description string
}

// FileRef is a file path the command reads or writes (e.g., manifest.ndjson).
type FileRef struct {
	Path        string
	Description string
}

// SeeAlsoRef is a man-page cross-reference (klvtool-decode(1)).
type SeeAlsoRef struct {
	Name    string
	Section int
}
