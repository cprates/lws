.DEFAULT_GOAL := docker_dev

export LWS_DEBUG ?= 1
export LWS_ACCOUNT_ID ?= 0000000000
export LWS_PROTO ?= http
# if changed, for now the makefile and Docker files must be updated as well
export LWS_LAMBDA_WORKDIR ?= repo
LWS_LOGS_FOLDER ?= "$(shell pwd)"/logs

# network
# is used by lambdas as gateway
LWS_IP ?= 172.18.0.2
# lambda's name server
LWS_NAMESERVER ?= 8.8.8.8
export LWS_PORT ?= 8080
LWS_DOCKER_SUBNET ?= 172.18.0.0/24
LWS_NETWORK_BITS ?= 24
LWS_DOCKER_GW ?= 172.18.0.1

# boot
LWS_IF_BRIDGE=br0
LWS_IF_HOST=eth0
LWS_WORK_DIR=/lws


export CURDIR

init:
	docker network create --subnet=$(LWS_DOCKER_SUBNET) lwsnetwork

docker_base:
	docker build -t cprates/lws_base:latest -f Dockerfile.base .

docker_lws:
	docker build --build-arg LWS_LAMBDA_WORKDIR --build-arg CURDIR \
	 --build-arg LWS_PORT -t cprates/lws:latest -f Dockerfile.lws .

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
	mkdir -p $(LWS_LOGS_FOLDER) && \
	docker run --privileged \
		--env LWS_DEBUG=$(LWS_DEBUG) \
		--env LWS_ACCOUNT_ID=$(LWS_ACCOUNT_ID) \
		--env LWS_PROTO=$(LWS_PROTO) \
		--env LWS_LAMBDA_WORKDIR=$(LWS_LAMBDA_WORKDIR) \
		--env LWS_IP=$(LWS_IP) \
		--env LWS_PORT=$(LWS_PORT) \
		--env LWS_DOCKER_SUBNET=$(LWS_DOCKER_SUBNET) \
		--env LWS_NETWORK_BITS=$(LWS_NETWORK_BITS) \
		--env LWS_DOCKER_GW=$(LWS_DOCKER_GW) \
		--env LWS_IF_BRIDGE=$(LWS_IF_BRIDGE) \
		--env LWS_NAMESERVER=$(LWS_NAMESERVER) \
		--env LWS_IF_HOST=$(LWS_IF_HOST) \
		--env LWS_WORK_DIR=$(LWS_WORK_DIR) \
		--env USER_UID=$(shell id -u) \
		--env GROUP_UID=$(shell id -g) \
		-p $(LWS_PORT):$(LWS_PORT) \
		--net lwsnetwork \
		--mount src=$(LWS_LOGS_FOLDER),target=/var/log,type=bind \
		cprates/lws:latest
