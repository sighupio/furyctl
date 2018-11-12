
build:
	gox -osarch="linux/amd64 darwin/amd64"
	mv furyctl_* bin/

install_deps:
	go get github.com/mitchellh/gox
