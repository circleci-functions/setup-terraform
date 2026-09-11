# setup-terraform

A CircleCI function that installs the [Terraform](https://www.terraform.io/) CLI and puts
it on `PATH` so later steps can run `terraform` by name. The download is verified against
HashiCorp's published `SHA256SUMS` before it is installed.

## Usage

Invoke it in a `run:` step with the full function path:

```yaml
- run:
    name: Install Terraform
    command: circleci run github.com/circleci-functions/setup-terraform@<version> --
```

Pin a Terraform version:

```yaml
- run:
    name: Install Terraform
    command: circleci run github.com/circleci-functions/setup-terraform@<version> -- --version 1.9.8
```

The `--` separates the function's flags from the runner's own; keep it even when passing no
flags.

## Flags

| Flag | Default | Description |
|---|---|---|
| `--version` | `latest` | Terraform version to install. `latest` for the newest stable release, an exact version (`1.9.8`), or a prefix (`1.9`, `1`) that resolves to the newest stable release starting with it. Prereleases are never selected. |
| `--os` | `linux` | Operating system to install for: `linux` or `darwin`. |
| `--arch` | `amd64` | Architecture to install for: `amd64` or `arm64`. |

`--os` and `--arch` describe the release to download; set them to match the executor when it
is not `linux`/`amd64`.

## Platform support

Runs on `linux` and `darwin`, for `amd64` and `arm64`. There is no Windows build; on Windows
the function exits with an error rather than installing anything.
