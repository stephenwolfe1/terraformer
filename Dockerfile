# syntax=docker/dockerfile:1
ARG GO_VERSION="1.27.3"
ARG ALPINE_VERSION="3.24.3"
ARG TERRAFORM_VERSION="1.17.1"
ARG VAULT_VERSION="2.2.1"
ARG GOLANGCI_LINT_VERSION="2.15.0"
ARG HADOLINT_VERSION="2.15.1"

########################
# Terraform for the build platform (provider mirroring and checks)
########################
FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION} AS terraform-cli

ARG TERRAFORM_VERSION
ARG BUILDARCH

RUN apk add --no-cache curl unzip ca-certificates \
  && curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${BUILDARCH}.zip" -o /tmp/tf.zip \
  && unzip -q /tmp/tf.zip -d /usr/local/bin \
  && rm /tmp/tf.zip

########################
# Provider mirror + Terraform for the target platform
########################
FROM terraform-cli AS terraform

ARG TERRAFORM_VERSION
ARG TARGETOS=linux
ARG TARGETARCH

# Mirror providers for the *target* platform. Edit providers.tf to change them.
COPY providers.tf /tmp/providers/providers.tf
RUN terraform -chdir=/tmp/providers providers mirror -platform=${TARGETOS}_${TARGETARCH} /mirror

# Terraform for the target platform (copied into the final image)
RUN curl -fsSL "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_${TARGETOS}_${TARGETARCH}.zip" -o /tmp/tf.zip \
  && unzip -q /tmp/tf.zip -d /target-bin \
  && rm /tmp/tf.zip

########################
# Go wrapper (cross-compiled on the build platform)
########################
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION="dev"
ARG GIT_COMMIT=""
ARG GIT_TREE_STATE=""

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
  -ldflags "-s -w \
    -X github.com/stephenwolfe1/terraformer/internal/cli.version=${VERSION} \
    -X github.com/stephenwolfe1/terraformer/internal/cli.gitCommit=${GIT_COMMIT} \
    -X github.com/stephenwolfe1/terraformer/internal/cli.gitTreeState=${GIT_TREE_STATE}" \
  -o /out/terraformer ./cmd/terraformer

########################
# Checks: docker buildx build --target check .  (make check)
# Not part of the default image build.
########################
FROM --platform=$BUILDPLATFORM golangci/golangci-lint:v${GOLANGCI_LINT_VERSION} AS golangci-lint
FROM --platform=$BUILDPLATFORM hadolint/hadolint:v${HADOLINT_VERSION} AS hadolint

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS check

COPY --from=golangci-lint /usr/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=hadolint /bin/hadolint /usr/local/bin/hadolint
COPY --from=terraform-cli /usr/local/bin/terraform /usr/local/bin/terraform

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/root/.cache/golangci-lint <<'EOF'
set -eu
echo "==> golangci-lint config";   golangci-lint config verify
echo "==> go format";              golangci-lint fmt --diff
echo "==> go lint";                golangci-lint run
echo "==> go test";                go test -race -count=1 ./...
echo "==> dockerfile lint";        hadolint Dockerfile
echo "==> terraform fmt";          terraform fmt -check -diff -recursive .
echo "==> all checks passed"
EOF

########################
# Runtime image
########################
FROM alpine:${ALPINE_VERSION}

ARG TARGETOS=linux
ARG TARGETARCH
ARG VAULT_VERSION

RUN apk add --no-cache bash git openssh-client make jq curl unzip ca-certificates aws-cli \
  && curl -fsSL "https://releases.hashicorp.com/vault/${VAULT_VERSION}/vault_${VAULT_VERSION}_${TARGETOS}_${TARGETARCH}.zip" -o /tmp/vault.zip \
  && unzip -q /tmp/vault.zip vault -d /usr/local/bin \
  && rm /tmp/vault.zip \
  && git config --system --add safe.directory '*'

COPY --from=terraform /target-bin/terraform /usr/local/bin/terraform
COPY --from=terraform --chmod=755 /mirror /providers
COPY --from=build /out/terraformer /usr/local/bin/terraformer
COPY terraform.rc /etc/terraform.rc

ENV TF_CLI_CONFIG_FILE=/etc/terraform.rc \
  AWS_PAGER="" \
  LANG=C.UTF-8

RUN terraform version && vault version && aws --version && terraformer version

WORKDIR /terraform
ENTRYPOINT ["terraformer"]
