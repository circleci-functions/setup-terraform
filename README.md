# setup-terraform

Installs the [Terraform](https://developer.hashicorp.com/terraform) CLI, puts it on `PATH` for
the rest of the job, and caches the provider plugins `terraform init` downloads. The download
is checked against HashiCorp's published `SHA256SUMS` before anything is installed.

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
            checksum: '{{ checksum ".terraform.lock.hcl" }}'
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

A prefix matches whole version components, so `1.1` picks the newest `1.1.x`, never `1.10.0`.
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

The directory is chosen by preference, `~/.local/bin` then `~/bin` then `/usr/local/bin`, as
long as it comes ahead of any `terraform` the image already has. Failing that, setup-terraform
takes over the directory that one is in. If that needs root, it uses `sudo -n`. A binary it
replaces is kept as `terraform.pathlink-old` in the same directory.

On Windows, steps pick `terraform` up in any shell (bash, cmd or PowerShell) through small
forwarding shims rather than symlinks.

Once linked, setup-terraform runs the `terraform` that `PATH` resolves and fails unless it
reports the version just installed.

## Caching

Caching is on by default. setup-terraform restores Terraform's
[provider plugin cache](https://developer.hashicorp.com/terraform/cli/config/config-file#provider-plugin-cache)
before your steps run, and saves it once the job is done. So `terraform init` in a later step
reuses providers an earlier job downloaded, and nothing in between needs a cache step of its
own. A job that fails, is cancelled or times out still saves what it downloaded.

Pass `--checksum` so the cache refreshes when your providers change. CircleCI expands
`{{ checksum ".terraform.lock.hcl" }}` before setup-terraform runs:

```yaml
- setup-terraform:
    with:
      checksum: '{{ checksum ".terraform.lock.hcl" }}'
```

When the checksum changes, the cache starts from the newest entry saved in the current 72-hour
window, so only new providers are downloaded. The cache rotates every 72 hours, which keeps it
from growing without limit. Each job keeps its own cache, since jobs that run different modules
need different providers. Use `--key` to keep caches apart within a job, for example per root
module in a monorepo, and `--cache=false` to turn caching off. Cache problems never fail the job.

Terraform uses a cached provider only when `.terraform.lock.hcl` records a checksum for it on
the platform the job runs on. A lock file made on a Mac often has only that Mac's. Record the
platforms CI uses as well, and commit the result:

```sh
terraform providers lock -platform=linux_amd64 -platform=darwin_arm64
```

Without that, `terraform init` downloads the provider every time, though the job still works.

setup-terraform tells Terraform where the cache is without touching your own CLI configuration:

- If the job sets `TF_PLUGIN_CACHE_DIR`, or your CLI configuration file sets
  `plugin_cache_dir`, that directory is cached.
- Otherwise it caches `~/.terraform.d/plugin-cache` (`%APPDATA%\terraform.d\plugin-cache` on
  Windows), and names it in `~/.terraform.d/setup-terraform.tfrc`, a file Terraform reads in
  every later step.
- Terraform doesn't read that file when `TF_CLI_CONFIG_FILE` is set. In that case
  setup-terraform exports `TF_PLUGIN_CACHE_DIR` through `$BASH_ENV` instead, which only bash
  steps read. To reach every shell, set `plugin_cache_dir` in that file yourself.

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
| `--os` | (the step's) | Operating system to install for: `linux`, `darwin` or `windows` |
| `--arch` | (the step's) | Architecture to install for: `amd64` or `arm64` |
| `--cache` | true | Set to `false` to turn off the provider plugin cache |
| `--key` | (empty) | Cache key component, for keeping caches apart within a job, e.g. per root module in a monorepo |
| `--checksum` | (empty) | Refresh the cache when this changes, usually `{{ checksum ".terraform.lock.hcl" }}` |

setup-terraform installs the release built for the platform the step runs on. `--os` and
`--arch` override that, for example to run the `amd64` build under Rosetta.

## Platform support

Linux, macOS and Windows, on amd64 and arm64. HashiCorp publishes Windows arm64 builds only
for recent Terraform releases. For an older one, pass `--arch amd64`, which runs under
emulation.
