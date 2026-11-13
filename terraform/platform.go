package terraform

import (
	"fmt"
	"slices"
	"strings"
)

// supportedOS and supportedArch are the platforms setup-terraform installs
// Terraform for.
var (
	supportedOS   = []string{"linux", "darwin"}
	supportedArch = []string{"amd64", "arm64"}
)

// Platform returns the release platform to install: osName and arch when set,
// and otherwise the platform the step runs on, goos and goarch. Either one
// outside the supported set is an error naming the flag that would fix it.
func Platform(osName, arch, goos, goarch string) (string, string, error) {
	if osName == "" {
		osName = goos
	}
	if arch == "" {
		arch = goarch
	}
	if !slices.Contains(supportedOS, osName) {
		return "", "", fmt.Errorf("--os must be one of %s, not %q", strings.Join(supportedOS, ", "), osName)
	}
	if !slices.Contains(supportedArch, arch) {
		return "", "", fmt.Errorf("--arch must be one of %s, not %q", strings.Join(supportedArch, ", "), arch)
	}
	return osName, arch, nil
}
