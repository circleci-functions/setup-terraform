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
	Version string `help:"Terraform version to install: 'latest', an exact version (1.9.8), or a prefix (1.9, 1) resolving to the newest stable match." default:"latest"`
	OS      string `help:"Operating system to install for." enum:"linux,darwin" default:"linux"`
	Arch    string `help:"Architecture to install for." enum:"amd64,arm64" default:"amd64"`
}

func (c *cli) Run(ctx context.Context) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("setup-terraform supports only linux and darwin, not %s", runtime.GOOS)
	}

	version, err := terraform.Install(ctx, terraform.Options{
		BaseURL:    terraform.DefaultBaseURL,
		Version:    c.Version,
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
		kong.Description("Install the Terraform CLI and put it on PATH.\n\nExample:\n  circleci run github.com/circleci-functions/setup-terraform@<version> -- --version 1.9.8"),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run(ctx)
}
