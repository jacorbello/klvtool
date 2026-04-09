package envcheck

import "strings"

// Guidance summarizes install steps for the detected platform.
type Guidance struct {
	Platform string
	Summary  string
	Steps    []string
}

// InstallGuidance returns platform-aware install advice for the media tools.
func InstallGuidance(goos string, env map[string]string) Guidance {
	switch detectPlatform(goos, env) {
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
				"Install ffmpeg and gstreamer using the platform's native package manager or manual binaries.",
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
		return Guidance{
			Platform: "unsupported",
			Summary:  "No automated install guidance is available for this platform.",
			Steps: []string{
				"Install ffmpeg and gstreamer using the platform's native package manager or manual binaries.",
			},
		}
	}
}

func detectPlatform(goos string, env map[string]string) string {
	if goos == "darwin" {
		return "macos"
	}
	if goos != "linux" {
		return "unsupported"
	}
	if isWSL(env) && isDebianUbuntu(env) {
		return "wsl"
	}
	if isDebianUbuntu(env) {
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

func isDebianUbuntu(env map[string]string) bool {
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
