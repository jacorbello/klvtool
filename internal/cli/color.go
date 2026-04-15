package cli

import "fmt"

type colorizer struct {
	enabled bool
}

func newColorizer(enabled bool) colorizer {
	return colorizer{enabled: enabled}
}

func (c colorizer) green(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func (c colorizer) red(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func (c colorizer) yellow(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func (c colorizer) cyan(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[36m%s\033[0m", s)
}

func (c colorizer) bold(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[1m%s\033[0m", s)
}

func (c colorizer) dim(s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\033[2m%s\033[0m", s)
}
