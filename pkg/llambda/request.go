package llambda

// ReqResult of a request process
type ReqResult struct {
	Data interface{}
	Err  error
	// extra data to be used by some errors like custom messages
	ErrData interface{}
}

// Request is used to interact with the LLambda processor.
type Request struct {
	action string
	id     string
	// request params
	params interface{}
	// channel to read the request result from
	resC chan *ReqResult
}

//ReqEnvironment is a list of environment variables to be configured for a function.
type ReqEnvironment struct {
	Variables map[string]string
}

// ReqCreateFunction is for create a function.
type ReqCreateFunction struct {
	Code         map[string]string
	Description  string
	Environment  ReqEnvironment
	FunctionName string
	Handler      string
	MemorySize   int
	Publish      bool
	Role         string
	Runtime      string
	Timeout      int
}

// ReqInvokeFunction is for invoke a function.
type ReqInvokeFunction struct {
	FunctionName   string
	Qualifier      string
	InvocationType string
	LogType        string
	Payload        []byte
}

// NewReq returns a new Request to interact with the LLambda Processor.
func NewReq(action, reqID string, params interface{}) Request {
	return Request{
		action: action,
		id:     reqID,
		params: params,
		resC:   make(chan *ReqResult),
	}
}

// PushReq pushes a Request to the LLambda Processor.
func PushReq(pC chan Request, action, reqID string, params interface{}) *ReqResult {

	rq := NewReq(action, reqID, params)
	pC <- rq
	return <-rq.resC
}
