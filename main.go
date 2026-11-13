// Command setup-terraform installs the Terraform CLI at a requested version and
// links it onto PATH so later steps can run terraform by name.
//
// The download is verified against HashiCorp's published SHA256SUMS before
// install.
//
// It also points Terraform at a provider plugin cache that is restored now and
// saved at the end of the job.
//
// Supported platforms: linux, darwin and windows, amd64 and arm64.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alecthomas/kong"

	"github.com/circleci-functions/toolkit/bashenv"
	"github.com/circleci-functions/toolkit/functionyaml"
	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-terraform/plugincache"
	"github.com/circleci-functions/setup-terraform/terraform"
)

type cli struct {
	Version     string `help:"Terraform version to install: 'latest', an exact version (1.9.8), or a prefix (1.9, 1) resolving to the newest stable match. When neither this nor --version-file is set, latest is installed." xor:"version"`
	VersionFile string `name:"version-file" help:"Path to a .terraform-version file to read the version from, in the form --version takes. Can't be used with --version." xor:"version" example:".terraform-version"`
	OS          string `help:"Operating system to install for: linux, darwin or windows. Defaults to the one the step runs on."`
	Arch        string `help:"Architecture to install for: amd64 or arm64. Defaults to the one the step runs on."`

	Cache    bool   `help:"Cache Terraform's provider plugins: restore them now and save them at the end of the job. Set to false to turn caching off." default:"true"`
	Key      string `help:"Optional cache key component, to keep caches apart, e.g. per root module in a monorepo."`
	Checksum string `help:"Optional checksum to start a fresh cache entry when it changes, usually the checksum of .terraform.lock.hcl."`
}

func (c *cli) Run(ctx context.Context, client *sdk.Client) error {
	osName, arch, err := terraform.Platform(c.OS, c.Arch, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	spec := c.Version
	switch {
	case c.VersionFile != "":
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
		OS:         osName,
		Arch:       arch,
		InstallDir: installDir(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "setup-terraform: installed Terraform v%s\n", version)

	if c.Cache {
		c.cachePlugins(ctx, client, osName+"-"+arch)
	}
	return nil
}

// cachePlugins points Terraform at a plugin cache, restores it, and saves it at
// the end of the job. Caching is a convenience, so a problem with it is
// reported and never fails the step.
func (c *cli) cachePlugins(ctx context.Context, client *sdk.Client, platform string) {
	home, _ := os.UserHomeDir()
	setup, err := plugincache.Configure(plugincache.Env{
		Getenv:    os.Getenv,
		GOOS:      runtime.GOOS,
		Home:      home,
		ExportEnv: bashenv.ExportEnvVars,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup-terraform: not caching provider plugins: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "setup-terraform: provider plugin cache at %s, %s\n", setup.Dir, setup.How)

	key := plugincache.Key{Key: c.Key, Job: os.Getenv("CIRCLE_JOB"), Checksum: c.Checksum, Platform: platform}
	plugincache.Restore(ctx, client, key)
	plugincache.Save(ctx, client.AtTeardown(), key, setup.Dir)
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
