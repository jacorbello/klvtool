package cli

import (
	"flag"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
)

// SubcommandFlagSet pairs a subcommand's CommandDef with a freshly-built
// FlagSet. The FlagSet uses the same registration helpers (decodeFlagSet,
// extractFlagSet, ...) that Execute uses at runtime, so the man-page
// generator and the runtime parser cannot disagree about flag names,
// defaults, or descriptions.
type SubcommandFlagSet struct {
	Def commanddef.CommandDef
	FS  *flag.FlagSet
}

// SubcommandFlagSets returns one entry per klvtool subcommand, in the same
// order shown by `klvtool --help`. Used by cmd/gen-manpages.
//
// Commands without flags (doctor, completion) return a nil FlagSet — the
// renderer skips the OPTIONS section in that case.
func SubcommandFlagSets() []SubcommandFlagSet {
	return []SubcommandFlagSet{
		{Def: versionDef, FS: versionFlagSet(&versionFlags{})},
		{Def: updateDef, FS: updateFlagSet(&updateFlags{})},
		{Def: doctorDef, FS: nil},
		{Def: extractDef, FS: extractFlagSet(&extractFlags{})},
		{Def: inspectDef, FS: inspectFlagSet(&inspectFlags{})},
		{Def: decodeDef, FS: decodeFlagSet(&decodeFlags{})},
		{Def: packetizeDef, FS: packetizeFlagSet(&packetizeFlags{})},
		{Def: diagnoseDef, FS: diagnoseFlagSet(&diagnoseFlags{})},
		{Def: completionDef, FS: nil},
	}
}
