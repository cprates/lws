
build:
	go build ./cmd/lws

build_docker:
	docker build -t cprates/lws:latest -f Dockerfile.lws .

build_docker_all: gobox build_docker

deploy: fmt build test

fmt:
	./check.sh

install:
	go install ./cmd/lws

run: install
	$(GOPATH)/bin/lws

test:
	go test ./...

install_runbox:
	go install ./cmd/runbox

build_gobox:
	docker build -t cprates/gobox:latest -f Dockerfile.gobox . \
		&& docker container rm lws_gobox || true \
		&& docker create --name lws_gobox cprates/gobox:latest

install_gobox:
	docker export lws_gobox > repo/golang_base.tar

gobox: build_gobox install_gobox