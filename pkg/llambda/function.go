package llambda

type function struct {
	codeFolder  string // ./{FunctionName}
	description string
	envVars     map[string]string
	handler     string
	lrn         string
	memorySize  int
	name        string
	publish     bool
	revID       string
	role        string
	runtime     string
	version     string
}
