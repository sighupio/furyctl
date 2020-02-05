
version = v0.1.7

tag:
	git tag $(version)

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/linux/$(version)/furyctl  .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/darwin/$(version)/furyctl .
	mkdir -p bin/{darwin,linux}/latest
	cp bin/darwin/$(version)/furyctl bin/darwin/latest/furyctl
	cp bin/linux/$(version)/furyctl bin/linux/latest/furyctl

upload-to-s3:
	aws s3 sync bin s3://sighup-releases --endpoint-url=https://s3.wasabisys.com --exclude '*' --include 'linux/$(version)/furyctl' --include 'darwin/$(version)/furyctl' --include 'darwin/latest/furyctl' --include 'linux/latest/furyctl'


vendor:
	go mod vendor

install_deps:
	go get github.com/mitchellh/gox
	
up:
	docker-compose up -d && docker-compose logs -f

down: 
	docker-compose down -v
