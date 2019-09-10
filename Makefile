.DEFAULT_GOAL := docker_dev

export LWS_DEBUG ?= 1
export LWS_ACCOUNT_ID ?= 0000000000
export LWS_PROTO ?= http
export LWS_HOST ?= 172.18.0.2
export LWS_PORT ?= 8080
# if changed, for now the makefile and Docker files must be updated as well
export LWS_LAMBDA_WORKDIR ?= repo

# infrastructure configs
LWS_DOCKER_SUBNET ?= 172.18.0.0/16
LWS_DOCKER_IP ?= 172.18.0.2


init:
	docker network create --subnet=$(LWS_DOCKER_SUBNET) lwsnetwork

docker_base:
	docker build -t cprates/lws_base:latest -f Dockerfile.base .

docker_lws:
	docker build --build-arg LWS_LAMBDA_WORKDIR --build-arg LWS_PORT -t cprates/lws:latest -f Dockerfile.lws .

build_gobox:
	docker build -t cprates/gobox:latest -f Dockerfile.gobox . \
		&& docker container rm lws_gobox || true \
		&& docker create --name lws_gobox cprates/gobox:latest

install_gobox:
	rm -rf $(LWS_LAMBDA_WORKDIR) 2>/dev/null \
		&& mkdir $(LWS_LAMBDA_WORKDIR) 2>/dev/null \
		&& docker export lws_gobox > $(LWS_LAMBDA_WORKDIR)/golang_base.tar

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
	# use docker compose instead?
	docker run --privileged \
		--env LWS_DEBUG=$(LWS_DEBUG) \
		--env LWS_ACCOUNT_ID=$(LWS_ACCOUNT_ID) \
		--env LWS_PROTO=$(LWS_PROTO) \
		--env LWS_HOST=$(LWS_HOST) \
		--env LWS_PORT=$(LWS_PORT) \
		--env LWS_LAMBDA_WORKDIR=$(LWS_LAMBDA_WORKDIR) \
		-p $(LWS_PORT):$(LWS_PORT) \
		--add-host lws:127.0.0.1 \
		--net lwsnetwork \
		--ip $(LWS_DOCKER_IP) cprates/lws:latest
