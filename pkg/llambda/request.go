package llambda

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
}
