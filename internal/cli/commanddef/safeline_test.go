package commanddef

import "testing"

func TestSafeLine(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "hello world", "hello world"},
		{"hyphen escaped", "foo-bar", "foo\\-bar"},
		{"leading dot guarded", ".bin files", `\&.bin files`},
		{"leading apostrophe guarded", "'twas the night", `\&'twas the night`},
		{"backslash inside passes through", "a\\b", "a\\\\b"},
		// roffEscape doubles backslashes; safeLine then sees the escaped
		// string. A line starting with a single literal backslash becomes
		// "\\..." which does NOT start with `.` or `'`, so no \& guard.
		{"leading backslash", "\\foo", "\\\\foo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeLine(c.in); got != c.want {
				t.Errorf("safeLine(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestRenderMan_LeadingDotInDescriptionIsGuarded(t *testing.T) {
	def := CommandDef{
		Name:        "klvtool-test",
		Synopsis:    "test",
		UsageLine:   "klvtool test",
		Description: "First line is fine.\n.bin files would otherwise be a macro call.",
	}
	got := renderManString(def, nil, ManOpts{Version: "1.2.0", Date: "2026-05-06"})

	// The escaped-and-guarded line must appear; the bare ".bin" must not
	// appear at line-start (which would invoke a roff macro).
	want := `\&.bin files would otherwise be a macro call.`
	if !contains(got, want) {
		t.Errorf("output missing guarded leading-dot line %q:\n%s", want, got)
	}
	// Cheaper sanity: no line in output should be a bare ".bin" prefix.
	for _, line := range splitLines(got) {
		if line == ".bin files would otherwise be a macro call." {
			t.Errorf("bare leading-dot line escaped to mandoc: %q", line)
		}
	}
}

// Tiny helpers so this file doesn't pull strings into a test that's
// otherwise about formatting.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
