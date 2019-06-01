package main

import "github.com/gorilla/mux"

type server struct {
	router *mux.Router
}

func newServer() *server {
	return &server{
		router: mux.NewRouter(),
	}
}
