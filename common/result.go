package common

import "github.com/cprates/lws/pkg/lerr"

// Result is the result of a call to a service API method.
type Result struct {
	// Maps directly to HTTP status codes
	Status int
	ReqID  string
	Result []byte // XML
	Err    *lerr.Result
}

// ErrMissingParamRes is for generating AWS compatible MissingParameter error.
func ErrMissingParamRes(msg, reqID string) Result {
	return Result{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "MissingParameter",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrNotImplementedRes generates our custom error for not implemented actions.
func ErrNotImplementedRes(reqID string) Result {
	return Result{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "InvalidAction",
				Message:   "not implemented",
				RequestID: reqID,
			},
		},
	}
}

// ErrInternalErrorRes is for make our life easier when generating internal error results.
func ErrInternalErrorRes(msg, reqID string) Result {
	return Result{
		Status: 500,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Internal",
				Code:      "InternalFailure",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidActionRes generates an InvalidAction error.
func ErrInvalidActionRes(msg, reqID string) Result {
	return Result{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "InvalidAction",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidAttributeNameRes generates an InvalidAttributeName error.
func ErrInvalidAttributeNameRes(msg, reqID string) Result {
	return Result{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "InvalidAttributeName",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidParameterValueRes generates an InvalidParameterValue error.
func ErrInvalidParameterValueRes(msg, reqID string) Result {
	return Result{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "InvalidParameterValue",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// SuccessRes is for make our life easier when generating a success result.
func SuccessRes(res []byte, reqID string) Result {
	return Result{
		Status: 200,
		ReqID:  reqID,
		Result: res,
	}
}
