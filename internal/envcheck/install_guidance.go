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
	case "wsl":
		return Guidance{
			Platform: "wsl",
			Summary:  "Install the Linux packages inside the WSL distribution.",
			Steps: []string{
				"WSL: use the Linux distribution package manager inside the WSL environment.",
				"sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
			},
		}
	default:
		return Guidance{
			Platform: "debian_ubuntu",
			Summary:  "Install the backend tools with apt.",
			Steps: []string{
				"sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
			},
		}
	}
}

func detectPlatform(goos string, env map[string]string) string {
	if goos == "darwin" {
		return "macos"
	}
	if goos != "linux" {
		return "debian_ubuntu"
	}
	if isWSL(env) {
		return "wsl"
	}
	return "debian_ubuntu"
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
