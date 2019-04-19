package common

import "github.com/cprates/lws/pkg/lerr"

// Result is the result of a call to a service API method.
type Result struct {
	// Maps directly to HTTP status codes
	Status int
	Err    lerr.Result
}
