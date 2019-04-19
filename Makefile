


fmt:
	./build.sh

build:
	go build ./cmd/lws

install:
	go install ./cmd/lws

run: install
	$(GOPATH)/bin/lws
