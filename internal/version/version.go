package version

var version = "dev"

func String() string {
	if version == "" {
		return "dev"
	}
	return version
}
