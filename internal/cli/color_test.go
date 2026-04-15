package cli

import "testing"

func TestColorizerEnabledWrapsGreen(t *testing.T) {
	c := newColorizer(true)
	got := c.green("ok")
	want := "\033[32mok\033[0m"
	if got != want {
		t.Fatalf("green(%q) = %q, want %q", "ok", got, want)
	}
}

func TestColorizerEnabledWrapsRed(t *testing.T) {
	c := newColorizer(true)
	got := c.red("fail")
	want := "\033[31mfail\033[0m"
	if got != want {
		t.Fatalf("red(%q) = %q, want %q", "fail", got, want)
	}
}

func TestColorizerEnabledWrapsDim(t *testing.T) {
	c := newColorizer(true)
	got := c.dim("/usr/bin/ffmpeg")
	want := "\033[2m/usr/bin/ffmpeg\033[0m"
	if got != want {
		t.Fatalf("dim(%q) = %q, want %q", "/usr/bin/ffmpeg", got, want)
	}
}

func TestColorizerEnabledWrapsYellow(t *testing.T) {
	c := newColorizer(true)
	got := c.yellow("warn")
	want := "\033[33mwarn\033[0m"
	if got != want {
		t.Fatalf("yellow(%q) = %q, want %q", "warn", got, want)
	}
}

func TestColorizerEnabledWrapsCyan(t *testing.T) {
	c := newColorizer(true)
	got := c.cyan("hint")
	want := "\033[36mhint\033[0m"
	if got != want {
		t.Fatalf("cyan(%q) = %q, want %q", "hint", got, want)
	}
}

func TestColorizerDisabledPassesThrough(t *testing.T) {
	c := newColorizer(false)
	for _, tc := range []struct {
		name string
		fn   func(string) string
	}{
		{"green", c.green},
		{"red", c.red},
		{"dim", c.dim},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn("hello")
			if got != "hello" {
				t.Fatalf("%s(%q) = %q, want %q", tc.name, "hello", got, "hello")
			}
		})
	}
}
