package llambda

import "github.com/cprates/lws/pkg/list"

type function struct {
	description   string
	envVars       map[string]string
	handler       string
	lrn           string
	memorySize    int
	name          string
	publish       bool
	revID         string
	role          string
	runtime       string
	version       string
	idleInstances *list.List
}

type instance struct {
	id string
}
