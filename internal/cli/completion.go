package cli

import (
	"fmt"
	"io"
	"os"
)

// CompletionCommand generates shell completion scripts.
type CompletionCommand struct {
	Out io.Writer
	Err io.Writer
}

// NewCompletionCommand returns a CompletionCommand with default writers.
func NewCompletionCommand() *CompletionCommand {
	return &CompletionCommand{
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

func (c *CompletionCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}
	if len(args) == 0 {
		c.writeUsage(c.Err)
		c.writeError(c.Err, "shell argument required: bash, zsh, or fish")
		return usageExitCode
	}
	if len(args) > 1 {
		c.writeUsage(c.Err)
		c.writeError(c.Err, fmt.Sprintf("unsupported arguments: %v", args[1:]))
		return usageExitCode
	}

	switch args[0] {
	case "bash":
		writeBashCompletion(c.Out)
	case "zsh":
		writeZshCompletion(c.Out)
	case "fish":
		writeFishCompletion(c.Out)
	default:
		c.writeUsage(c.Err)
		c.writeError(c.Err, fmt.Sprintf("unsupported shell %q (want bash, zsh, or fish)", args[0]))
		return usageExitCode
	}
	return 0
}

func (c *CompletionCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool completion <bash|zsh|fish>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Generate shell completion scripts.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Examples:")
	_, _ = fmt.Fprintln(w, `  eval "$(klvtool completion bash)"`)
	_, _ = fmt.Fprintln(w, `  eval "$(klvtool completion zsh)"`)
	_, _ = fmt.Fprintln(w, "  klvtool completion fish | source")
}

func (c *CompletionCommand) writeError(w io.Writer, msg string) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "error: %s\n", msg)
}

func writeBashCompletion(w io.Writer) {
	_, _ = fmt.Fprint(w, `_klvtool() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="version update doctor inspect extract decode packetize diagnose completion"

    if [ "$COMP_CWORD" -eq 1 ]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return
    fi

    case "${COMP_WORDS[1]}" in
        version)
            COMPREPLY=($(compgen -W "--check" -- "$cur"))
            ;;
        update)
            COMPREPLY=($(compgen -W "--dry-run --prefer-binary" -- "$cur"))
            ;;
        doctor)
            COMPREPLY=()
            ;;
        inspect)
            case "$prev" in
                --input) COMPREPLY=($(compgen -f -- "$cur")) ;;
                --view) COMPREPLY=($(compgen -W "auto pretty raw" -- "$cur")) ;;
                *) COMPREPLY=($(compgen -W "--input --view" -- "$cur")) ;;
            esac
            ;;
        extract)
            case "$prev" in
                --input) COMPREPLY=($(compgen -f -- "$cur")) ;;
                --out) COMPREPLY=($(compgen -d -- "$cur")) ;;
                --view) COMPREPLY=($(compgen -W "auto pretty raw" -- "$cur")) ;;
                *) COMPREPLY=($(compgen -W "--input --out --view" -- "$cur")) ;;
            esac
            ;;
        decode)
            case "$prev" in
                --input|--out) COMPREPLY=($(compgen -f -- "$cur")) ;;
                --format) COMPREPLY=($(compgen -W "ndjson text csv" -- "$cur")) ;;
                --view) COMPREPLY=($(compgen -W "auto pretty raw" -- "$cur")) ;;
                --pid|--schema) COMPREPLY=() ;;
                *) COMPREPLY=($(compgen -W "--input --format --view --raw --strict --step --pid --out --schema" -- "$cur")) ;;
            esac
            ;;
        packetize)
            case "$prev" in
                --input|--out) COMPREPLY=($(compgen -d -- "$cur")) ;;
                --mode) COMPREPLY=($(compgen -W "strict best-effort" -- "$cur")) ;;
                --view) COMPREPLY=($(compgen -W "auto pretty raw" -- "$cur")) ;;
                *) COMPREPLY=($(compgen -W "--input --out --mode --view" -- "$cur")) ;;
            esac
            ;;
        diagnose)
            case "$prev" in
                --input) COMPREPLY=($(compgen -f -- "$cur")) ;;
                --view) COMPREPLY=($(compgen -W "auto pretty raw" -- "$cur")) ;;
                *) COMPREPLY=($(compgen -W "--input --view" -- "$cur")) ;;
            esac
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            ;;
    esac
}
complete -F _klvtool klvtool
`)
}

func writeZshCompletion(w io.Writer) {
	_, _ = fmt.Fprint(w, `#compdef klvtool

_klvtool() {
    local -a commands
    commands=(
        'version:Print version information'
        'update:Update to the latest release'
        'doctor:Check backend availability'
        'inspect:Inspect MPEG-TS stream inventory'
        'extract:Extract payloads and write manifest'
        'decode:Decode MISB ST 0601 KLV records'
        'packetize:Replay raw checkpoints and write packet output'
        'diagnose:Run the full diagnostic pipeline'
        'completion:Generate shell completion scripts'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
        return
    fi

    case "${words[2]}" in
        version)
            _arguments '--check[Check for updates]'
            ;;
        update)
            _arguments \
                '--dry-run[Print actions without installing]' \
                '--prefer-binary[Download release archive]'
            ;;
        inspect)
            _arguments \
                '--input[MPEG-TS input file]:file:_files' \
                '--view[Presentation mode]:mode:(auto pretty raw)'
            ;;
        extract)
            _arguments \
                '--input[MPEG-TS input file]:file:_files' \
                '--out[Output directory]:dir:_directories' \
                '--view[Presentation mode]:mode:(auto pretty raw)'
            ;;
        decode)
            _arguments \
                '--input[MPEG-TS input file]:file:_files' \
                '--format[Output format]:format:(ndjson text csv)' \
                '--view[Presentation mode]:mode:(auto pretty raw)' \
                '--raw[Include raw bytes]' \
                '--strict[Exit 1 on error diagnostics]' \
                '--step[Interactive stepping]' \
                '--pid[Filter to PID]:pid:' \
                '--out[Output file]:file:_files' \
                '--schema[Override spec URN]:urn:'
            ;;
        packetize)
            _arguments \
                '--input[Checkpoint input directory]:dir:_directories' \
                '--out[Output directory]:dir:_directories' \
                '--mode[Parser mode]:mode:(strict best-effort)' \
                '--view[Presentation mode]:mode:(auto pretty raw)'
            ;;
        diagnose)
            _arguments \
                '--input[MPEG-TS input file]:file:_files' \
                '--view[Presentation mode]:mode:(auto pretty raw)'
            ;;
        completion)
            _arguments ':shell:(bash zsh fish)'
            ;;
    esac
}

compdef _klvtool klvtool
`)
}

func writeFishCompletion(w io.Writer) {
	_, _ = fmt.Fprint(w, `# klvtool fish completions

set -l commands version update doctor inspect extract decode packetize diagnose completion

complete -c klvtool -f
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a version -d "Print version information"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a update -d "Update to the latest release"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a doctor -d "Check backend availability"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a inspect -d "Inspect MPEG-TS stream inventory"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a extract -d "Extract payloads and write manifest"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a decode -d "Decode MISB ST 0601 KLV records"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a packetize -d "Replay raw checkpoints"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a diagnose -d "Run the full diagnostic pipeline"
complete -c klvtool -n "not __fish_seen_subcommand_from $commands" -a completion -d "Generate shell completions"

# version
complete -c klvtool -n "__fish_seen_subcommand_from version" -l check -d "Check for updates"

# update
complete -c klvtool -n "__fish_seen_subcommand_from update" -l dry-run -d "Print actions without installing"
complete -c klvtool -n "__fish_seen_subcommand_from update" -l prefer-binary -d "Download release archive"

# inspect
complete -c klvtool -n "__fish_seen_subcommand_from inspect" -l input -rF -d "MPEG-TS input file"
complete -c klvtool -n "__fish_seen_subcommand_from inspect" -l view -ra "auto pretty raw" -d "Presentation mode"

# extract
complete -c klvtool -n "__fish_seen_subcommand_from extract" -l input -rF -d "MPEG-TS input file"
complete -c klvtool -n "__fish_seen_subcommand_from extract" -l out -rF -d "Output directory"
complete -c klvtool -n "__fish_seen_subcommand_from extract" -l view -ra "auto pretty raw" -d "Presentation mode"

# decode
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l input -rF -d "MPEG-TS input file"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l format -ra "ndjson text csv" -d "Output format"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l view -ra "auto pretty raw" -d "Presentation mode"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l raw -d "Include raw bytes"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l strict -d "Exit 1 on error diagnostics"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l step -d "Interactive stepping"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l pid -r -d "Filter to PID"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l out -rF -d "Output file"
complete -c klvtool -n "__fish_seen_subcommand_from decode" -l schema -r -d "Override spec URN"

# packetize
complete -c klvtool -n "__fish_seen_subcommand_from packetize" -l input -rF -d "Checkpoint input directory"
complete -c klvtool -n "__fish_seen_subcommand_from packetize" -l out -rF -d "Output directory"
complete -c klvtool -n "__fish_seen_subcommand_from packetize" -l mode -ra "strict best-effort" -d "Parser mode"
complete -c klvtool -n "__fish_seen_subcommand_from packetize" -l view -ra "auto pretty raw" -d "Presentation mode"

# diagnose
complete -c klvtool -n "__fish_seen_subcommand_from diagnose" -l input -rF -d "MPEG-TS input file"
complete -c klvtool -n "__fish_seen_subcommand_from diagnose" -l view -ra "auto pretty raw" -d "Presentation mode"

# completion
complete -c klvtool -n "__fish_seen_subcommand_from completion" -a "bash zsh fish" -d "Shell type"
`)
}
