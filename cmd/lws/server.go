package main

import (
	"net/http"
)

type server struct {
	router *http.ServeMux
}

func newServer() *server {
	return &server{
		router: http.DefaultServeMux,
	}
}

func (s server) regRoute(path string, f http.HandlerFunc) server {

	s.router.HandleFunc(path, f)
	return s
}
