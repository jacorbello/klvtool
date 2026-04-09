package envcheck

import (
	"os"
	"strings"
)

// Guidance summarizes install steps for the detected platform.
type Guidance struct {
	Platform string
	Summary  string
	Steps    []string
}

type osReleaseReader func() (string, error)

const unsupportedGuidanceStep = "Install ffmpeg and gstreamer using the platform's native package manager or manual binaries."

// InstallGuidance returns platform-aware install advice for the media tools.
func InstallGuidance(goos string, env map[string]string) Guidance {
	return installGuidance(goos, env, defaultOSReleaseReader)
}

func installGuidance(goos string, env map[string]string, readOSRelease osReleaseReader) Guidance {
	switch detectPlatform(goos, env, readOSRelease) {
	case "macos":
		return Guidance{
			Platform: "macos",
			Summary:  "Install the backend tools with Homebrew.",
			Steps: []string{
				"brew install ffmpeg gstreamer",
				"brew upgrade ffmpeg gstreamer",
			},
		}
	case "unsupported":
		return Guidance{
			Platform: "unsupported",
			Summary:  "No automated install guidance is available for this platform.",
			Steps: []string{
				unsupportedGuidanceStep,
			},
		}
	case "wsl":
		return Guidance{
			Platform: "wsl",
			Summary:  "Install the Linux packages inside the WSL distribution.",
			Steps: []string{
				"WSL: use the Linux distribution package manager inside the WSL environment.",
				"sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
			},
		}
	case "debian_ubuntu":
		return Guidance{
			Platform: "debian_ubuntu",
			Summary:  "Install the backend tools with apt.",
			Steps: []string{
				"sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
			},
		}
	default:
		return unsupportedGuidance()
	}
}

func detectPlatform(goos string, env map[string]string, readOSRelease osReleaseReader) string {
	if goos == "darwin" {
		return "macos"
	}
	if goos != "linux" {
		return "unsupported"
	}
	if isWSL(env) && hasDebianUbuntuEvidence(env, readOSRelease) {
		return "wsl"
	}
	if hasDebianUbuntuEvidence(env, readOSRelease) {
		return "debian_ubuntu"
	}
	return "unsupported"
}

func isWSL(env map[string]string) bool {
	if len(env) == 0 {
		return false
	}
	if _, ok := env["WSL_INTEROP"]; ok {
		return true
	}
	if _, ok := env["WSL_DISTRO_NAME"]; ok {
		return true
	}
	if v := strings.ToLower(env["WSLENV"]); strings.Contains(v, "wsl") {
		return true
	}
	return false
}

func hasDebianUbuntuEvidence(env map[string]string, readOSRelease osReleaseReader) bool {
	if hasDebianUbuntuEnvEvidence(env) {
		return true
	}
	if readOSRelease == nil {
		readOSRelease = defaultOSReleaseReader
	}
	content, err := readOSRelease()
	if err != nil {
		return false
	}
	return hasDebianUbuntuOSReleaseEvidence(content)
}

func hasDebianUbuntuEnvEvidence(env map[string]string) bool {
	if len(env) == 0 {
		return false
	}
	for _, key := range []string{"DISTRO_FAMILY", "ID", "ID_LIKE"} {
		value := strings.ToLower(env[key])
		if strings.Contains(value, "debian") || strings.Contains(value, "ubuntu") {
			return true
		}
	}
	return false
}

func hasDebianUbuntuOSReleaseEvidence(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToUpper(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		lowerValue := strings.ToLower(value)
		switch key {
		case "ID", "ID_LIKE":
			if strings.Contains(lowerValue, "debian") || strings.Contains(lowerValue, "ubuntu") {
				return true
			}
		}
	}
	return false
}

func defaultOSReleaseReader() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unsupportedGuidance() Guidance {
	return Guidance{
		Platform: "unsupported",
		Summary:  "No automated install guidance is available for this platform.",
		Steps:    []string{unsupportedGuidanceStep},
	}
}
