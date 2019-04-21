
build:
	go build ./cmd/lws

deploy: fmt build test

fmt:
	./build.sh

install:
	go install ./cmd/lws

run: install
	$(GOPATH)/bin/lws

test:
	go test ./...