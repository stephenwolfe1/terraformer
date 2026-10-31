# Tool versions are pinned in the Dockerfile (ARG lines). Override with e.g.
#   make build BUILD_ARGS="--build-arg TERRAFORM_VERSION=1.17.0"
IMAGE      ?= stephenwolfe/terraformer
TAG        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
PLATFORMS  ?= linux/amd64,linux/arm64
BUILD_ARGS ?=
ARGS       ?= version
CMD        ?= plan

BUILD = docker buildx build $(BUILD_ARGS) \
	--build-arg VERSION=$(TAG) \
	--build-arg GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null) \
	-t $(IMAGE):$(TAG)

# Repo mounted at /terraform, credential dirs mounted when present, context and
# TERRAFORMER_* settings passed through from your shell.
RUN = docker run --rm -it -v "$(CURDIR):/terraform" \
	$(foreach d,.aws .kube .ssh .config/gcloud .vault-token,$(if $(wildcard $(HOME)/$(d)),-v "$(HOME)/$(d):/root/$(d)")) \
	$(foreach v,AWS_PROFILE VAULT_ADDR VAULT_TOKEN REPO ACCOUNT ENVIRONMENT REGION SERVICES SERVICES_DIR $(filter TERRAFORMER_%,$(.VARIABLES)),-e $(v))

.DEFAULT_GOAL := help
.PHONY: help check build push run shell example clean

help: ## show this help
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-8s\033[0m %s\n", $$1, $$2}'

check: ## lint, format-check and test everything (golangci-lint, go test -race, hadolint, terraform fmt)
	docker buildx build $(BUILD_ARGS) --target check --progress plain .

build: ## build the image for this machine
	$(BUILD) --load .

push: ## build and push the multi-arch image
	$(BUILD) --platform $(PLATFORMS) --push .

run: ## run the image: make run ARGS="plan examples/services/main"
	$(RUN) $(IMAGE):$(TAG) $(ARGS)

shell: ## open a shell in the image
	$(RUN) --entrypoint bash $(IMAGE):$(TAG)

example: ## run the example services: make example CMD=plan|apply|destroy
	$(MAKE) --no-print-directory run ARGS="$(CMD)" \
		REPO=terraformer ACCOUNT=example ENVIRONMENT=dev REGION=local \
		SERVICES_DIR=examples/services SERVICES="main disabled"

clean: ## remove example state, plans and caches
	find examples \( -name '.terraform*' -o -name 'terraform.tfstate*' -o -name '*.tfplan' \) -prune -exec rm -rf {} +