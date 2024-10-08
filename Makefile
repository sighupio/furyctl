_PROJECT_DIRECTORY = $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
_GOLANG_IMAGE = golang:1.22.0
_PROJECTNAME = furyctl
_GOARCH = "amd64"
_BIN_OPEN = "open"

NETRC_FILE ?= ~/.netrc

ifeq ("$(shell uname -s)", "Linux")
	_BIN_OPEN = "xdg-open"
endif

ifeq ("$(shell uname -m)", "arm64")
	_GOARCH = "arm64"
endif

#1: docker image
#2: make target
define run-docker
	@docker run --rm \
		-e CGO_ENABLED=0 \
		-e GOARCH=${_GOARCH} \
		-e GOOS=linux \
		-w /app \
		-v ${NETRC_FILE}:/root/.netrc \
		-v ${_PROJECT_DIRECTORY}:/app \
		$(1) $(2)
endef

#1: message
define yes-or-no
	@while true; do \
		read -r -p ${1}" [y/n]: " yn ; \
		case "$${yn}" in \
			[Yy]) break ;; \
			[Nn]) echo "Aborted, exiting..."; exit 1 ;; \
		esac \
	done
endef

.PHONY: env tools

env:
	@echo 'export CGO_ENABLED=0'
	@echo 'export GOARCH=${_GOARCH}'
	@grep -v '^#' .env | sed 's/^/export /'

tools:
	@go install github.com/daixiang0/gci@v0.13.4
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
	@go install github.com/google/addlicense@v1.1.1
	@go install github.com/nikolaydubina/go-cover-treemap@v1.4.2
	@go install github.com/onsi/ginkgo/v2/ginkgo@v2.19.0
	@go install golang.org/x/tools/cmd/goimports@v0.22.0
	@go install mvdan.cc/gofumpt@v0.6.0
	@go install github.com/momaek/formattag@v0.0.9

.PHONY: mod-download mod-tidy mod-verify

mod-download:
	@go mod download

mod-tidy:
	@go mod tidy

mod-verify:
	@go mod verify

.PHONY: mod-check-upgrades mod-upgrade

mod-check-upgrades:
	@go list -mod=readonly -u -f "{{if (and (not (or .Main .Indirect)) .Update)}}{{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}" -m all

mod-upgrade:
	@go get -u ./... && go mod tidy

.PHONY: license-add license-check

license-add:
	@addlicense -c "SIGHUP s.r.l" -y 2017-present -v -l bsd \
	-ignore "scripts/e2e/libs/**/*" \
	-ignore "vendor/**/*" \
	-ignore "*.gen.go" \
	-ignore ".idea/*" \
	-ignore ".vscode/*" \
	-ignore "*.js" \
	-ignore "kind-config.yaml" \
	-ignore ".husky/**/*" \
	-ignore ".go/**/*" \
	.

license-check:
	@addlicense -c "SIGHUP s.r.l" -y 2017-present -v -l bsd \
	-ignore "scripts/e2e/libs/**/*" \
	-ignore "vendor/**/*" \
	-ignore "*.gen.go" \
	-ignore ".idea/*" \
	-ignore ".vscode/*" \
	-ignore "*.js" \
	-ignore "kind-config.yaml" \
	-ignore ".husky/**/*" \
	-ignore ".go/**/*" \
	--check .

.PHONY: format-go fmt fumpt imports gci formattag

format-go: fmt fumpt imports gci formattag

fmt:
	@find . -name "*.go" -type f -not -path '*/vendor/*' \
	| sed 's/^\.\///g' \
	| xargs -I {} -S 5000 sh -c 'echo "formatting {}.." && gofmt -w -s {}'

fumpt:
	@find . -name "*.go" -type f -not -path '*/vendor/*' \
	| sed 's/^\.\///g' \
	| xargs -I {} -S 5000 sh -c 'echo "formatting {}.." && gofumpt -w -extra {}'

imports:
	@goimports -v -w -e -local github.com/sighupio main.go
	@goimports -v -w -e -local github.com/sighupio cmd/
	@goimports -v -w -e -local github.com/sighupio internal/

gci:
	@find . -name "*.go" -type f -not -path '*/vendor/*' \
	| sed 's/^\.\///g' \
	| xargs -I {} -S 5000 sh -c 'echo "formatting imports for {}.." && \
	gci write --skip-generated  -s standard -s default -s "Prefix(github.com/sighupio)" {}'

formattag:
	@find . -name "*.go" -type f -not -path '*/vendor/*' \
	| sed 's/^\.\///g' \
	| xargs -I {} -S 5000 sh -c 'formattag -file {}'

.PHONY: lint lint-go

lint: lint-go

lint-go:
	@GOFLAGS=-mod=mod golangci-lint -v run --color=always --max-same-issues 25 --config=${_PROJECT_DIRECTORY}/.rules/.golangci.yml ./...

.PHONY: test-unit test-integration test-e2e test-all show-coverage

test-unit:
	@GOFLAGS=-mod=mod go test -v -tags=unit ./...

test-integration:
	@GOFLAGS=-mod=mod go test -v -tags=integration -timeout 120s ./...

test-e2e:
	@export KFD_AUTH_DEX_CONNECTORS_GITHUB_CLIENT_ID=dummy && \
	export KFD_AUTH_DEX_CONNECTORS_GITHUB_CLIENT_SECRET=dummy && \
	export KFD_BASIC_AUTH_PASSWORD=dummy && \
	export KFD_AUTH_POMERIUM_COOKIE_SECRET=dummy && \
	export KFD_AUTH_POMERIUM_IDP_CLIENT_SECRET=dummy && \
	export KFD_AUTH_POMERIUM_SHARED_SECRET=dummy && \
	GOFLAGS=-mod=mod ginkgo run -vv --trace -tags=e2e -timeout 600s -p test/e2e

test-expensive:
	$(call yes-or-no, "WARNING: This test will create a cluster on AWS. Are you sure you want to continue?")
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -tags=expensive -timeout 36000s --procs=4 test/expensive

test-expensive-ekscluster:
	$(call yes-or-no, "WARNING: This test will create a cluster on AWS. Are you sure you want to continue?")
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -tags=expensive -timeout 36000s --procs=4 test/expensive/ekscluster

test-expensive-kfddistribution:
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -tags=expensive -timeout 36000s --procs=4 test/expensive/kfddistribution

test-expensive-onpremises:
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -tags=expensive -timeout 36000s --procs=4 test/expensive/onpremises

test-most:
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -coverpkg=./... -covermode=count -coverprofile=coverage.out -tags=unit,integration,e2e,expensive --skip-package=expensive -timeout 300s -p ./...

test-all:
	$(call yes-or-no, "WARNING: This test will create a cluster on AWS. Are you sure you want to continue?")
	@GOFLAGS=-mod=mod ginkgo run -vv --trace -coverpkg=./... -covermode=count -coverprofile=coverage.out -tags=unit,integration,e2e,expensive -timeout 300s --procs=2 ./...

show-coverage:
	@go tool cover -html=coverage.out -o coverage.html
	@go-cover-treemap -coverprofile coverage.out > coverage.svg && ${_BIN_OPEN} coverage.svg

.PHONY: clean build release

clean: deps
	@if [ -d bin ]; then rm -rf bin; fi
	@if [ -d dist ]; then rm -rf dist; fi
	@if [ -f furyctl ]; then rm furyctl; fi

build:
	@export GO_VERSION=$$(go version | cut -d ' ' -f 3) && \
	goreleaser check && \
	goreleaser release --verbose --snapshot --clean

release:
	@export GO_VERSION=$$(go version | cut -d ' ' -f 3) && \
	goreleaser check && \
	goreleaser release --verbose --clean

# Helpers

%-docker:
	$(call run-docker,${_GOLANG_IMAGE},make $*)

check-variable-%: # detection of undefined variables.
	@[[ "${${*}}" ]] || (echo '*** Please define variable `${*}` ***' && exit 1)
