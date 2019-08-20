
docker_base:
	docker build -t cprates/lws_base:latest -f Dockerfile.base .

docker_lws:
	docker build -t cprates/lws:latest -f Dockerfile.lws .

build_gobox:
	docker build -t cprates/gobox:latest -f Dockerfile.gobox . \
		&& docker container rm lws_gobox || true \
		&& docker create --name lws_gobox cprates/gobox:latest

install_gobox:
	docker export lws_gobox > repo/golang_base.tar

gobox: build_gobox install_gobox

docker_dev: gobox docker_lws

docker_all: docker_base gobox docker_lws


build:
	go build ./cmd/lws

deploy: fmt build test

fmt:
	./check.sh

install:
	go install ./cmd/lws

run_local: install
	$(GOPATH)/bin/lws

test:
	go test ./...

install_runbox:
	go install ./cmd/runbox

run:
	docker run --privileged -p 8080:8080 --add-host lws:127.0.0.1 --net lwsnetwork --ip 172.18.0.2  cprates/lws:latest
