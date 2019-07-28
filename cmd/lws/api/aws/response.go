package aws

import "fmt"

// Details contains details of an AWS compatible error.
type Details struct {
	Code      string `xml:"Code,omitempty"`
	Message   string `xml:"Message,omitempty"`
	RequestID string `xml:"RequestId,omitempty"`
	Type      string `xml:"Type,omitempty"`
}

// ResponseErr is the root of an AWS compatible error response.
type ResponseErr struct {
	Details Details `xml:"Error"`
}

func (e ResponseErr) Error() string {
	return fmt.Sprintf("error %s(%s)", e.Details.Code, e.Details.Message)
}

// Response is the result of a call to a service API method.
type Response struct {
	// Maps directly to HTTP status codes
	Status int
	ReqID  string
	Result []byte // XML
	Err    *ResponseErr
}

// ErrMissingParamRes is for generating AWS compatible MissingParameter error.
func ErrMissingParamRes(msg, reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Sender",
				Code:      "MissingParameter",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrNotImplementedRes generates our custom error for not implemented actions.
func ErrNotImplementedRes(reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Sender",
				Code:      "InvalidAction",
				Message:   "not implemented",
				RequestID: reqID,
			},
		},
	}
}

// ErrInternalErrorRes is for make our life easier when generating internal error results.
func ErrInternalErrorRes(msg, reqID string) Response {
	return Response{
		Status: 500,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Internal",
				Code:      "InternalFailure",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidActionRes generates an InvalidAction error.
func ErrInvalidActionRes(msg, reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Sender",
				Code:      "InvalidAction",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidAttributeNameRes generates an InvalidAttributeName error.
func ErrInvalidAttributeNameRes(msg, reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Sender",
				Code:      "InvalidAttributeName",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// ErrInvalidParameterValueRes generates an InvalidParameterValue error.
func ErrInvalidParameterValueRes(msg, reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
				Type:      "Sender",
				Code:      "InvalidParameterValue",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// SuccessRes is for make our life easier when generating a success result.
func SuccessRes(res []byte, reqID string) Response {
	return Response{
		Status: 200,
		ReqID:  reqID,
		Result: res,
	}
}
