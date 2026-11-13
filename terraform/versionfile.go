package terraform

import (
	"fmt"
	"os"
	"strings"
)

// ReadVersionFile reads the version spec from a tfenv-style .terraform-version
// file: a single version, such as 1.14.7, with surrounding whitespace and #
// comments ignored. The spec is returned as written, for ResolveVersion.
func ReadVersionFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // reading the file the user named is the point
	if err != nil {
		return "", err
	}
	var specs []string
	for line := range strings.Lines(string(data)) {
		line, _, _ = strings.Cut(line, "#")
		specs = append(specs, strings.Fields(line)...)
	}
	switch len(specs) {
	case 0:
		return "", fmt.Errorf("%s has no version in it", path)
	case 1:
		return specs[0], nil
	default:
		return "", fmt.Errorf("%s has more than one version in it: %s", path, strings.Join(specs, " "))
	}
}
