# AGENTS.md

Guidance for coding agents working in this repository.

## Overview

`terraformer` is a Go CLI (cobra + viper, Go 1.27) that wraps `terraform` for
consistent local/Docker/CI runs, shipped as a multi-arch Docker image with
Terraform, a pre-mirrored provider cache, the AWS CLI and the Vault CLI. It is
intentionally generic: no company-specific hosts, registries, providers or
naming conventions belong in this repo. Deployment context
(`REPO`/`ACCOUNT`/`ENVIRONMENT`/`REGION`), `state.yaml` gating and backend
keys are all optional features.

## Commands

There is no local Go build: everything goes through Docker.

```bash
make check                # Dockerfile `check` stage: golangci-lint (lint + fmt), go test -race, hadolint, terraform fmt
make build                # image for this machine
make run ARGS="plan path" # run the image with the repo mounted at /terraform
make example CMD=plan     # run the examples with a sample context
make help                 # all targets
go test ./internal/terraform -run TestSubcommandArgs -v   # single test, if Go is installed locally
```

## Layout

- `cmd/terraformer/main.go` → `cli.Execute()` returns the exit code (terraform's own code is propagated).
- `internal/cli/`
  - `root.go`: the `settings` table defines every flag, its env aliases and default. Each is bound through viper to `TERRAFORMER_<FLAG>` first, then legacy/short env names (`REPO`, `ENVIRONMENT`, `SERVICES`, …). Add new config here, then read it in `optionsFor`.
  - `targets.go`: resolves directories (path arg → `SERVICES` under `SERVICES_DIR` → `DIRECTORY` → cwd) and splits passthrough args after `--`.
  - `service_check.go`: skips dirs without `*.tf`, applies optional `state.yaml` gating.
  - `commands.go`: `init`, `validate`, `plan`, `apply`, `destroy`, `state`; `version.go` (ldflags-injected `version`, `gitCommit`, `gitTreeState`).
- `internal/terraform/`
  - `runner.go`: `Options`, `Init`, `Run` (silent init + subcommand), `Shell`, env construction (`buildEnv`), default flags per subcommand (`subcommandArgs`), signal forwarding, `CleanCache`.
  - `context.go`: context slug, backend-key template expansion, `TF_VAR_*` export.
  - `output.go`: status banners (respects `NO_COLOR`) and `PlanFilter` that hides refresh noise.
  - `github_summary.go`: CI step summary (`GITHUB_STEP_SUMMARY`, `TERRAFORMER_SUMMARY_MIRROR_PATH`).

## Behaviour worth knowing

- `init` runs with `-upgrade` by default (`TERRAFORMER_UPGRADE=false` to disable) so stale lock files don't pin versions missing from the image mirror; this matches terraformer 1.x.
- Passthrough flags after `--` and `TERRAFORMER_EXTRA_ARGS` are appended *after* wrapper defaults so they override them.
- `apply` only adds `-input=false` together with `-auto-approve` (terraform otherwise refuses to apply). `--auto-approve` never affects `destroy`.
- Context values are exported as `TF_VAR_*` (not `-var`) so configurations that don't declare them still work; existing `TF_VAR_*` and `TF_DATA_DIR` values are never overridden.
- With a terminal, SIGINT is not forwarded (the tty already delivers it; a second one makes terraform exit without releasing locks).
- Providers are mirrored at image build time from `providers.tf`; `terraform.rc` makes `registry.terraform.io` providers install only from `/providers`.

## Versions

Terraform, Vault, Go, Alpine, golangci-lint and hadolint versions are pinned once, as `ARG`s in the `Dockerfile`; Go also in `go.mod`. CI (`.github/workflows/ci.yml`) reads the Terraform version from the Dockerfile for the image tag, runs the `check` stage, then builds the multi-arch image and pushes to Docker Hub on push (`develop` → `latest` and the Terraform version tag).

## Consumers

`github.com/stephenwolfe1/terraform` (homelab) runs this image with `DIRECTORY=/terraform/services/<name>`, an explicit `TF_DATA_DIR=/terraform/.terraform`, backend keys hard-coded in each service, no context and no `state.yaml`. Keep those paths working.
