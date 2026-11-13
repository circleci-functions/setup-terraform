# setup-terraform

Installs the [Terraform](https://developer.hashicorp.com/terraform) CLI and puts it on `PATH`
for the rest of the job. The download is checked against HashiCorp's published `SHA256SUMS`
before anything is installed.

Use this function instead of the `install` command of the `circleci/terraform` orb.

```yaml
version: 2.1

functions:
  setup-terraform: github.com/circleci-functions/setup-terraform@v0.2.0

jobs:
  plan:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - setup-terraform:
          with:
            version-file: .terraform-version
      - run: terraform init -input=false
      - run: terraform plan -input=false -out=tfplan
```

Later steps pick up `terraform` automatically, in whatever shell they run. There is nothing
to source or export yourself.

`circleci function add setup-terraform` writes the `functions:` entry for you, pinned to the
latest release. Each [flag](#flags) is a `with:` key, named without its leading `--`.

Where the `functions:` block isn't available, as on CircleCI Server, call setup-terraform by
its full name with `circleci run` and pass the flags after `--`:

```yaml
- run: circleci run github.com/circleci-functions/setup-terraform@v0.2.0 -- --version 1.9.8
```

## Choosing a version

Without `--version` or `--version-file`, setup-terraform installs the newest stable release.

`--version` takes a spec, not just an exact release:

| Spec | Resolves to |
|------|-------------|
| `latest` | Newest stable release |
| `1` | Newest stable `1.x.y` |
| `1.9` | Newest stable `1.9.x` |
| `1.9.8` | Exactly `1.9.8` |

Prereleases (`1.10.0-rc1`, `1.10.0-alpha20240606`) are never picked by `latest` or a prefix.

To keep the version in one place, point at a [tfenv](https://github.com/tfutils/tfenv)-style
`.terraform-version` file:

```yaml
- setup-terraform:
    with:
      version-file: .terraform-version
```

The file holds one version, in any form `--version` takes. Blank lines and `#` comments are
ignored. A missing or empty file is an error, as is one with more than one version in it.
`--version` and `--version-file` can't be used together.

## How it gets onto PATH

A step inherits its environment from the job, not from the step before it, so setup-terraform
cannot just prepend to `PATH`. It installs `terraform` under `$XDG_CACHE_HOME/terraform`
(`~/.cache/terraform` by default) and links it into a directory `PATH` already holds, which
reaches every later process whatever shell it runs in.

The link goes in a writable directory on `PATH` ahead of any `terraform` the image already
has, or takes over the directory that one is in. If that needs root, setup-terraform uses
`sudo -n`. A binary it replaces is kept as `terraform.pathlink-old` in the same directory.

Once linked, setup-terraform runs the `terraform` that `PATH` resolves and fails unless it
reports the version just installed.

## HCP Terraform and Terraform Enterprise

setup-terraform writes no credentials. Terraform 1.2 and later reads an API token for a host
from an environment variable named `TF_TOKEN_` followed by the hostname, with dots written as
underscores. Put it in a context:

| Variable | Value |
|---|---|
| `TF_TOKEN_app_terraform_io` | An HCP Terraform API token |

For a Terraform Enterprise host such as `tfe.example.com`, name it
`TF_TOKEN_tfe_example_com`. For other CLI settings, point `TF_CLI_CONFIG_FILE` at a
[CLI configuration file](https://developer.hashicorp.com/terraform/cli/config/config-file)
in your repo.

## Running Terraform

setup-terraform only installs the CLI. Run `init`, `plan`, `apply` and the rest as ordinary
`run` steps, with `-input=false` so a missing variable fails rather than waits for input:

```yaml
- setup-terraform:
    with:
      version-file: .terraform-version
- run: terraform fmt -check -recursive
- run: terraform init -input=false
- run: terraform validate
- run: terraform plan -input=false -out=tfplan
- persist_to_workspace:
    root: .
    paths: [tfplan]
```

Set `TF_IN_AUTOMATION=1` on the job to drop the hints Terraform prints for interactive use.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | `latest` | Version spec: `latest`, `1`, `1.9`, `1.9.8`. See [Choosing a version](#choosing-a-version) |
| `--version-file` | (empty) | Path to a `.terraform-version` file to read the version spec from. Can't be used with `--version` |
| `--os` | `linux` | Operating system to install for: `linux` or `darwin` |
| `--arch` | `amd64` | Architecture to install for: `amd64` or `arm64` |

`--os` and `--arch` pick the release to download. Set them to match the executor when it is
not `linux`/`amd64`.

## Platform support

Linux and macOS, on amd64 and arm64. There is no Windows support yet: on Windows the function
exits with an error rather than installing anything.
