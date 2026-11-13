package terraform

import (
	"context"
	"fmt"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
)

// Options configures an Install.
type Options struct {
	// BaseURL is the release server; DefaultBaseURL in production.
	BaseURL string
	// Version is the spec to resolve: "latest", an exact version, or a prefix.
	Version string
	OS      string
	Arch    string
	// InstallDir is where the terraform binary is placed and linked from. It
	// must persist for the rest of the job.
	InstallDir string
}

// Install resolves opts.Version, downloads and verifies that Terraform release,
// links it onto PATH, and returns the concrete version installed.
func Install(ctx context.Context, opts Options) (string, error) {
	version, err := ResolveVersion(ctx, opts.BaseURL, opts.Version)
	if err != nil {
		return "", err
	}
	if _, err := Download(ctx, opts.BaseURL, version, opts.OS, opts.Arch, opts.InstallDir); err != nil {
		return "", err
	}
	if err := linkOnPath(opts.InstallDir, version); err != nil {
		return "", err
	}
	return version, nil
}

// linkOnPath makes the terraform binary in installDir resolvable by name in
// later steps, verifying that PATH then resolves terraform to the requested
// version rather than a copy already present.
func linkOnPath(installDir, version string) error {
	_, err := pathlink.Ensure(pathlink.Options{
		Names:     []string{binaryName},
		FromDir:   installDir,
		Preferred: pathlink.UserBinDirs(),
		Verify: func(bin string) error {
			out, err := pathlink.Command(bin, "version").CombinedOutput()
			if err != nil {
				return fmt.Errorf("run terraform version: %w", err)
			}
			if !strings.Contains(string(out), "Terraform v"+version) {
				return fmt.Errorf("linked terraform is not v%s: %s", version, strings.TrimSpace(string(out)))
			}
			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("put terraform on PATH: %w", err)
	}
	return nil
}
