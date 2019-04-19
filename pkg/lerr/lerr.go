package lerr

import "fmt"

// Details contains details of an AWS compatible error.
type Details struct {
	Code      string `xml:"Code,omitempty"`
	Message   string `xml:"Message,omitempty"`
	RequestID string `xml:"RequestId,omitempty"`
	Type      string `xml:"Type,omitempty"`
}

// Result is the root of an AWS compatible error response.
type Result struct {
	Result Details `xml:"Error"`
}

func (e Result) Error() string {
	return fmt.Sprintf("error %s(%s)", e.Result.Code, e.Result.Message)
}
