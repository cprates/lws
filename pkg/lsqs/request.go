package lsqs

// ReqResult contains the result of any request to LSQS.
type ReqResult struct {
	Data interface{}
	Err  error
	// extra data to be used by some errors like custom messages
	ErrData interface{}
}

// Request is used to interact with the LSQS processor.
type Request struct {
	action string
	id     string
	// request params
	params map[string]string
	// request attributes
	attributes map[string]string
	// channel to read the request result from
	resC chan *ReqResult
}

// NewReq returns a new Request to interact with the LSQS Processor.
func NewReq(action, reqID string, params, attributes map[string]string) Request {
	return Request{
		action:     action,
		id:         reqID,
		params:     params,
		attributes: attributes,
		resC:       make(chan *ReqResult),
	}
}

// PushReq pushes a Request to the LSQS Processor.
func PushReq(
	pC chan Request, action, reqID string, params, attributes map[string]string,
) *ReqResult {

	rq := NewReq(action, reqID, params, attributes)
	pC <- rq
	return <-rq.resC
}
