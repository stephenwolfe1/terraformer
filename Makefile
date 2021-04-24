BINDIR     := $(CURDIR)/bin
DIST_DIRS  := find * -type d -exec
TARGETS    := darwin/amd64 linux/amd64 windows/amd64
BINNAME    ?= terraformer

GOPATH        = $(shell go env GOPATH 2>/dev/null)
DEP           = $(GOPATH)/bin/dep
GOX           = $(GOPATH)/bin/gox
GOIMPORTS     = $(GOPATH)/bin/goimports

# go option
PKG        :=
TAGS       :=
TESTS      :=
TESTFLAGS  :=
LDFLAGS    := -w -s
GOFLAGS    :=
SRC        := $(shell find . -type f -name '*.go' -print)

# Required for globs to work correctly
SHELL      = /bin/bash

GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

ifdef VERSION
	BINARY_VERSION = $(VERSION)
endif
BINARY_VERSION ?= ${GIT_TAG}

# Only set Version if building a tag or VERSION is set
ifneq ($(BINARY_VERSION),)
	LDFLAGS += -X github.com/ghostgroup/terraformer/src/cmd/terraformer.version=${BINARY_VERSION}
endif

# Clear the "unreleased" string in BuildMetadata
ifneq ($(GIT_TAG),)
	LDFLAGS += -X github.com/ghostgroup/terraformer/src/cmd/terraformer.metadata=
endif

LDFLAGS += -X github.com/ghostgroup/terraformer/src/cmd/terraformer.gitCommit=${GIT_COMMIT}
LDFLAGS += -X github.com/ghostgroup/terraformer/src/cmd/terraformer.gitTreeState=${GIT_DIRTY}

# ------------------------------------------------------------------------------
# Docker stuff
IMAGE_TAG ?= ${GIT_COMMIT}
COMMAND   ?= version
TYPE      ?= services
NAME      ?= main
IMAGE     := stephenwolfe/terraformer

build:
	docker build --build-arg LDFLAGS="${LDFLAGS}" --target=app -t ${IMAGE}:${IMAGE_TAG} .

shell: build
	docker run -it ${IMAGE}:${IMAGE_TAG} /bin/sh

push:
	docker push ${IMAGE}:${IMAGE_TAG}

tag:
	docker tag ${IMAGE}:${GIT_COMMIT} ${IMAGE}:${IMAGE_TAG}

# Used to test the container
export VAULT_ADDR := https://vault.stephenwolfe.int

terraformer := docker run \
	-it \
	--rm \
	-v $(PWD)/examples/:/terraform \
	-v $(HOME)/.aws:/root/.aws \
	-v $(HOME)/.ssh:/root/.ssh \
	-v $(HOME)/.vault-token:/root/.vault-token \
	-e TF_DATA_DIR=/terraform/.terraform \
	-e VAULT_TOKEN \
	-e VAULT_ADDR \
	-e DIRECTORY=/terraform/${TYPE}/${NAME} \
	${IMAGE}:${IMAGE_TAG}

test:
	${terraformer} ${COMMAND} # help render validate diff

# ------------------------------------------------------------------------------
#  dependencies

# If go get is run from inside the project directory it will add the dependencies
# to the go.mod file. To avoid that we change to a directory without a go.mod file
# when downloading the following dependencies

$(GOX):
	(cd /; GO111MODULE=on go get -u github.com/mitchellh/gox)

$(GOIMPORTS):
	(cd /; GO111MODULE=on go get -u golang.org/x/tools/cmd/goimports)

# ------------------------------------------------------------------------------
# Build go binaries
#  build
.PHONY: build-go
build-go: $(BINDIR)/$(BINNAME)

$(BINDIR)/$(BINNAME): $(SRC)
	GO111MODULE=on go build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINNAME) ./src

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross: $(GOX)
	GO111MODULE=on CGO_ENABLED=0 $(GOX) -parallel=3 -output="_dist/{{.OS}}-{{.Arch}}/$(BINNAME)" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' ./src

.PHONY: dist
dist:
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf terraformer-${VERSION}-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r terraformer-${VERSION}-{}.zip {} \; \
	)
# ------------------------------------------------------------------------------
.PHONY: info
info:
	 @echo "Version:           ${VERSION}"
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"
.DEFAULT_GOAL := info
