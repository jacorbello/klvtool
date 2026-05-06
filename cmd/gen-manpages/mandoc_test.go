package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedPagesPassMandocLint runs `mandoc -T lint` against the
// committed man/man1/*.1 pages and fails on anything that's not a STYLE
// warning. Skipped when mandoc isn't installed (older distros, Windows
// runners). The Linux CI image has mandoc pre-installed via the
// `mandoc` package; macOS ships it in the base system.
//
// This catches roff-correctness regressions in render_man.go (empty
// .UR blocks, unguarded leading-dot lines, malformed macros) before
// they ship — the unit tests on synthetic CommandDefs only lock in
// byte-equality against goldens, which won't fire if a future change
// produces *different* but still-broken output.
func TestCommittedPagesPassMandocLint(t *testing.T) {
	mandoc, err := exec.LookPath("mandoc")
	if err != nil {
		t.Skip("mandoc not installed; skipping committed-page lint")
	}

	// The test runs from cmd/gen-manpages — repo root is two levels up.
	pages, err := filepath.Glob("../../man/man1/klvtool*.1")
	if err != nil {
		t.Fatalf("glob committed pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatalf("no committed pages found; expected man/man1/klvtool*.1")
	}

	args := append([]string{"-T", "lint"}, pages...)
	out, err := exec.Command(mandoc, args...).CombinedOutput()
	// mandoc exits non-zero only on parse errors. STYLE warnings still
	// produce output but exit 0. We accept STYLE; reject anything else.
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		// Lines look like: "mandoc: <file>:<line>:<col>: <SEVERITY>: <msg>"
		// SEVERITY ∈ {STYLE, WARNING, ERROR, UNSUPP, BADARG, SYSERR}.
		// Only STYLE is acceptable noise; everything else fails the test.
		if strings.Contains(line, ": STYLE: ") {
			continue
		}
		t.Errorf("mandoc lint issue: %s", line)
	}
	// If exec failed (parse error of severity SYSERR/etc.), surface the
	// exit error too. We've already logged the diagnostic lines above.
	if err != nil && len(bytes.TrimSpace(out)) == 0 {
		t.Errorf("mandoc exited with %v but emitted no diagnostic", err)
	}
}
