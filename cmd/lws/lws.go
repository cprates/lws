package main

import (
	"log"
	"net/http"

	"github.com/cprates/lws/cmd/lws/internal/api"
)

func main() {
	log.Println("Strating LWS...")

	s := newServer()

	awsCli := api.NewAwsCli()
	awsCli.InstallSQS()
	s.regRoute("/", awsCli.Dispatcher())

	log.Fatal(http.ListenAndServe(":8080", nil))
}
