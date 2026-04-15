package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type viewMode string

const (
	viewAuto   viewMode = "auto"
	viewPretty viewMode = "pretty"
	viewRaw    viewMode = "raw"
)

type hintFooter struct {
	Title string
	Body  string
}

func parseViewMode(raw string) (viewMode, error) {
	switch viewMode(strings.TrimSpace(raw)) {
	case "", viewAuto:
		return viewAuto, nil
	case viewPretty:
		return viewPretty, nil
	case viewRaw:
		return viewRaw, nil
	default:
		return "", fmt.Errorf("invalid view %q (want auto|pretty|raw)", raw)
	}
}

func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func isTTYReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func supportsANSI() bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	term := strings.TrimSpace(os.Getenv("TERM"))
	return term != "" && term != "dumb"
}

func usePrettyView(mode viewMode, tty bool) bool {
	switch mode {
	case viewPretty:
		return true
	case viewRaw:
		return false
	default:
		return tty
	}
}

func writeHintFooters(w io.Writer, color colorizer, footers []hintFooter) {
	if w == nil || len(footers) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, color.cyan("Next steps:"))
	for _, footer := range footers {
		if strings.TrimSpace(footer.Title) != "" {
			_, _ = fmt.Fprintf(w, "  %s %s\n", color.bold("-"), footer.Title)
		}
		if strings.TrimSpace(footer.Body) != "" {
			_, _ = fmt.Fprintf(w, "    %s\n", footer.Body)
		}
	}
}

func warningLine(color colorizer, format string, args ...any) string {
	return fmt.Sprintf("%s %s", color.yellow("warning:"), fmt.Sprintf(format, args...))
}

func errorLine(color colorizer, err error) string {
	return fmt.Sprintf("%s %v", color.red("error:"), err)
}
