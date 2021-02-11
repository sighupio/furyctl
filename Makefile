.DEFAULT_GOAL: help
SHELL := /bin/bash

PROJECTNAME := $(shell basename "$(PWD)")
CURRENT_DIR := $(shell pwd)

.PHONY: help
all: help
help: Makefile
	@echo
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo

.PHONY: deps
## deps: download requires dependencies
deps:
	@go get -u github.com/gobuffalo/packr/v2/packr2

.PHONY:
## policeman: Execute policeman
policeman:
	@docker pull quay.io/sighup/policeman
	@docker run --rm -v ${CURRENT_DIR}:/app -w /app quay.io/sighup/policeman

.PHONY: lint
## lint: Execute linter. Can cause modifications
lint:
	@gofmt -s -w .

.PHONY: test
## test: Check the linter and unit tests results
test:
	@test -z $(gofmt -l .)
	@go test -v ./...

.PHONY: clean
## clean: Removes temporal and build results
clean: deps
	@GO111MODULE=on packr2 clean
	@rm -rf bin furyctl dist
	@go mod tidy

.PHONY: build
## build: Builds the solution for linux and macos amd64 
build: deps clean test
	@GO111MODULE=on packr2 build
	@GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/linux/$(version)/furyctl  .
	@GO111MODULE=on CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/darwin/$(version)/furyctl .
	@mkdir -p bin/{darwin,linux}/latest
	@cp bin/darwin/$(version)/furyctl bin/darwin/latest/furyctl
	@cp bin/linux/$(version)/furyctl bin/linux/latest/furyctl
