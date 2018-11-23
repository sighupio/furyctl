
version = v0.0.3

tag:
	git tag $(version)

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/linux/$(version)/furyctl .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -ldflags '-extldflags "-static"' -o bin/darwin/$(version)/furyctl .

upload-to-s3:
	make tag || true
	aws s3 sync bin s3://sighup-releases --endpoint-url=https://s3.wasabisys.com --exclude '*' --include 'linux/$(version)/furyctl' --include 'darwin/$(version)/furyctl' 

vendor:
	go mod vendor

install_deps:
	go get github.com/mitchellh/gox
	
up:
	docker-compose up -d && docker-compose logs -f

down: 
	docker-compose down -v
