# terraformer

A small Go wrapper around Terraform that makes the same command behave the same
way on a laptop, in Docker and in CI. It ships as a multi-arch image
(`linux/amd64`, `linux/arm64`) with Terraform, a pre-mirrored provider cache,
the AWS CLI and the Vault CLI.

What it adds on top of plain `terraform`:

- **One command, many services** – run in a path, or for every name in
  `SERVICES` under `SERVICES_DIR`.
- **Silent init** – `init` runs quietly before `plan`/`apply`/`destroy`, and
  `plan` hides state-refresh noise so only the plan is shown.
- **Optional deployment context** – `REPO`, `ACCOUNT`, `ENVIRONMENT`, `REGION`
  isolate `TF_DATA_DIR` per context, are exported as `TF_VAR_*`, can template
  the backend key, and drive per-service enablement via `state.yaml`.
- **CI friendly** – GitHub step summaries, saved plan + plan JSON, opt-in
  auto-approve, Terraform's exit code propagated, graceful lock release on
  cancel.
- **Fast, offline provider installs** – providers from `providers.tf` are
  mirrored into the image at build time.

## Quick start

```bash
make build                                  # build (and test) the image for your machine
make example CMD=plan                       # plan the example services
make run ARGS="plan examples/services/main" # any command, any path
make shell                                  # bash inside the image
```

Run the image directly:

```bash
docker run --rm -it -v "$PWD:/terraform" -v "$HOME/.aws:/root/.aws:ro" \
  -e AWS_PROFILE -e ENVIRONMENT=dev -e REGION=us-east-1 \
  stephenwolfe/terraformer plan services/network
```

## Commands

| Command | What it runs |
|---|---|
| `terraformer init [path]` | `terraform init` (output shown) |
| `terraformer validate [path]` | `init -backend=false`, then `validate` |
| `terraformer plan [path]` | silent init, then `plan -input=false -lock=false` with refresh output hidden |
| `terraformer apply [path]` | silent init, then `apply -lock=true -lock-timeout=0s` |
| `terraformer destroy [path]` | silent init, then `destroy -lock=true -lock-timeout=0s` |
| `terraformer state [path]` | init, then an interactive bash shell with the same environment |
| `terraformer version [--json]` | wrapper and Terraform versions |

Pass any Terraform flags after `--`; they are appended last, so they override
the defaults:

```bash
terraformer plan services/vpc -- -target=module.subnets -lock=true
terraformer destroy services/vpc -- -auto-approve
```

### Choosing what to run

1. an explicit `path` argument
2. otherwise each name in `SERVICES` (space separated), resolved as `SERVICES_DIR/<name>` (default `services/`)
3. otherwise `DIRECTORY`
4. otherwise the current directory

Directories with no `*.tf` / `*.tf.json` files are skipped.

## Configuration

Every flag can also be set as `TERRAFORMER_<FLAG>` (dashes become underscores,
e.g. `TERRAFORMER_AUTO_APPROVE=true`). The short names below are also read for
convenience; the `TERRAFORMER_` form wins if both are set.

| Flag | Env | Default | Description |
|---|---|---|---|
| `--repo` | `REPO` | | Repository/project name (context) |
| `--account` | `ACCOUNT` | | Account / subscription / project (context) |
| `-e, --environment` | `ENVIRONMENT` | | Environment (context) |
| `-r, --region` | `REGION` | | Region (context) |
| `--services` | `SERVICES` | | Service names to run when no path is given |
| `--services-dir` | `SERVICES_DIR` | `services` | Where service names are resolved |
| `-d, --directory` | `DIRECTORY` | | Default directory when no path or services are given |
| `--state-file` | | `state.yaml` | Per-service enablement file; `''` disables gating |
| `--require-state-file` | | `false` | Skip services that have no state file |
| `--backend-key` | | | Template for `-backend-config=key=…` |
| `--extra-args` | | | Extra flags appended to the subcommand |
| `--auto-approve` | | `false` | `apply` without prompting (`-auto-approve -input=false`); never applies to `destroy` |
| `--save-plan` | | `false` | `plan -out=terraform.tfplan` plus a JSON rendering |
| `--plan-json-file` | | `<account>-<environment>-<region>-<service>.json` | Plan JSON filename |
| `--clean` | | `false` | Remove the data dir, `.terraform.lock.hcl` and plan before/after each run |
| `--upgrade` | | `true` | `init -upgrade`, so providers resolve to the newest mirrored versions even if a lock file pins others; `--upgrade=false` to honour lock files strictly |
| `--binary` | | `terraform` | Terraform-compatible binary to run |
| `-p, --profile` | | | Exported as `AWS_PROFILE` / `AWS_DEFAULT_PROFILE` |
| `--vault-addr` | | | Exported as `VAULT_ADDR` |
| `--no-color` | `NO_COLOR` | `false` | Plain wrapper output |
| | `TERRAFORMER_SUMMARY_MIRROR_PATH` | | Also write the CI step summary to this file |

### Deployment context

Context is entirely optional. When any of `REPO`, `ACCOUNT`, `ENVIRONMENT` or
`REGION` is set:

- **`TF_DATA_DIR`** becomes `.terraform-<account>-<environment>-<region>-<service>`,
  so different contexts never share a `.terraform` directory. An explicit
  `TF_DATA_DIR` always wins.
- **`TF_VAR_account`, `TF_VAR_environment`, `TF_VAR_region`, `TF_VAR_repo_name`**
  are exported. Terraform ignores these for configurations that don't declare
  the variables, and existing `TF_VAR_*` values are never overridden.
- **Backend key** placeholders `{repo}`, `{service}`, `{account}`,
  `{environment}`, `{region}` are expanded in `--backend-key`:

  ```bash
  export TERRAFORMER_BACKEND_KEY='{repo}/{service}/{account}/{environment}/{region}/terraform.tfstate'
  ```

### Service gating with `state.yaml`

If a service directory contains the state file, the service only runs where it
is enabled. `ACCOUNT`, `ENVIRONMENT` and `REGION` must be set when the file is
present.

```yaml
# services/network/state.yaml
prod:
  production:
    us-east-1:
      enabled: true
dev:
  staging:
    us-east-1:
      enabled: true
```

Services without the file always run, unless `--require-state-file` is set.

### CI

- With `CI=true` and `GITHUB_STEP_SUMMARY` (or `TERRAFORMER_SUMMARY_MIRROR_PATH`)
  set, each command's output is appended to the step summary as markdown.
- `TF_IN_AUTOMATION=1` is set when `CI=true`.
- Terraform's exit code is returned unchanged, so
  `terraformer plan -- -detailed-exitcode` exits `2` when there are changes.
- On `SIGTERM` (and `SIGINT` without a terminal) the signal is forwarded so
  Terraform can release state locks before exiting.

Example GitHub Actions step:

```yaml
- name: Plan
  run: |
    docker run --rm -v "$PWD:/terraform" \
      -e CI -e GITHUB_STEP_SUMMARY=/terraform/summary.md \
      -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN \
      -e ACCOUNT=prod -e ENVIRONMENT=production -e REGION=us-east-1 \
      -e SERVICES="network compute" -e TERRAFORMER_SAVE_PLAN=true \
      stephenwolfe/terraformer plan
    cat summary.md >> "$GITHUB_STEP_SUMMARY"
```

## Providers

`providers.tf` lists the providers mirrored into `/providers` at build time.
The image's `terraform.rc` installs `registry.terraform.io` providers **only**
from that mirror (no registry downloads at runtime); providers from other
registries are downloaded directly. Edit `providers.tf` and rebuild to change
the set, or set `TF_CLI_CONFIG_FILE` to your own CLI config.

## Building

Everything builds in Docker; no local Go toolchain is needed.

```bash
make check   # golangci-lint (lint + gofmt/goimports), go test -race, hadolint, terraform fmt
make build   # image for this machine
make push    # multi-arch image (linux/amd64, linux/arm64), pushed
make help    # all targets
```

`make check` runs the Dockerfile's `check` stage, so it needs only Docker.
Lint rules live in `.golangci.yml` and `.hadolint.yaml`.

Tool versions are pinned once, as `ARG`s in the `Dockerfile`: Terraform
`1.17.1`, Vault `2.2.1`, Go `1.27.3`, Alpine `3.24.3`, golangci-lint `2.15.0`,
hadolint `2.15.1`. Override per build, e.g.
`make build BUILD_ARGS="--build-arg TERRAFORM_VERSION=1.17.0"`.

CI runs the same `check` stage on every push and pull request, then builds the
multi-arch image. Pushes publish `<sha>`; `develop`
also publishes `latest` and `<terraform version>` (read from the Dockerfile);
`v*` tags publish the tag. It needs the `DOCKER_USERNAME` and
`DOCKER_PASSWORD` secrets, and optionally a `DOCKER_IMAGE` repository variable.

## Upgrading from terraformer 1.x

| 1.x | Now |
|---|---|
| `DIRECTORY=/terraform/services/main terraformer plan` | `terraformer plan services/main` (`DIRECTORY` still works) |
| `OPTIONS="-target=x"` | `terraformer plan -- -target=x` or `TERRAFORMER_EXTRA_ARGS` |
| `GLOBAL_OPTIONS` | removed; the wrapper always uses `-chdir` |
| `TF_DATA_DIR` / `--data-directory` | `TF_DATA_DIR` is respected as-is; otherwise derived from context |
| `VAULT_TOKEN` / `~/.vault-token` handling | passed through to Terraform/Vault unchanged |
| `apply` always auto-approved | prompts unless `TERRAFORMER_AUTO_APPROVE=true` |
| `init -upgrade -plugin-dir=/providers` | `init -upgrade` (default) with the mirror configured in `terraform.rc` |
| errors exited 0 | Terraform's exit code is propagated |
| `src/` layout | `cmd/terraformer` + `internal/{cli,terraform}` |
