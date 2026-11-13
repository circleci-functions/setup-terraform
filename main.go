// Command setup-terraform installs the Terraform CLI at a requested version and
// links it onto PATH so later steps can run terraform by name.
//
// The download is verified against HashiCorp's published SHA256SUMS before
// install.
//
// Supported platforms: linux and darwin, amd64 and arm64.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alecthomas/kong"

	"github.com/circleci-functions/toolkit/functionyaml"
	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-terraform/terraform"
)

type cli struct {
	Version     string `help:"Terraform version to install: 'latest', an exact version (1.9.8), or a prefix (1.9, 1) resolving to the newest stable match. When neither this nor --version-file is set, latest is installed." xor:"version"`
	VersionFile string `name:"version-file" help:"Path to a .terraform-version file to read the version from, in the form --version takes. Can't be used with --version." xor:"version" example:".terraform-version"`
	OS          string `help:"Operating system to install for." enum:"linux,darwin" default:"linux"`
	Arch        string `help:"Architecture to install for." enum:"amd64,arm64" default:"amd64"`
}

func (c *cli) Run(ctx context.Context) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("setup-terraform supports only linux and darwin, not %s", runtime.GOOS)
	}

	spec := c.Version
	switch {
	case c.VersionFile != "":
		var err error
		spec, err = terraform.ReadVersionFile(c.VersionFile)
		if err != nil {
			return fmt.Errorf("read version file: %w", err)
		}
	case spec == "":
		spec = "latest"
	}

	version, err := terraform.Install(ctx, terraform.Options{
		BaseURL:    terraform.DefaultBaseURL,
		Version:    spec,
		OS:         c.OS,
		Arch:       c.Arch,
		InstallDir: installDir(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "setup-terraform: installed Terraform v%s\n", version)
	return nil
}

// installDir is where the terraform binary is placed before being linked onto
// PATH. It sits under the cache/home directory so it persists for the rest of
// the job and, in tests, follows the sandbox's redirected home.
func installDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "terraform")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "terraform")
	}
	return filepath.Join(os.TempDir(), "terraform-install")
}

func main() {
	sdk.Run(run)
}

func run(ctx context.Context, client *sdk.Client) error {
	var flags cli
	kCtx := kong.Parse(&flags,
		kong.Name("setup-terraform"),
		kong.Description("Install the Terraform CLI and put it on PATH for later steps. Use this function instead of the circleci/terraform orb's install command."),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run(ctx)
}
