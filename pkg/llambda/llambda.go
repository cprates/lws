package llambda

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cprates/lws/pkg/box"
	"github.com/cprates/lws/pkg/list"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Runtime contains all supported runtimes along with some configs.
var Runtime = struct {
	Supported  []string
	imageFile  map[string]string
	entrypoint map[string]string
}{
	Supported: []string{
		"go1.x",
	},
	imageFile: map[string]string{
		"go1.x": "golang_base.tar",
	},
	entrypoint: map[string]string{
		"go1.x": "/bin/gobox",
	},
}

// LLambda is the interface to implement if you want to implement your own Lambda service.
type LLambda interface {
	Process(<-chan Request, <-chan struct{})
}

// Instance represents an instance of lLambda core.
type Instance struct {
	AccountID string
	Region    string
	Proto     string
	Addr      string

	functions map[string]*function // by function name
	// TODO: align

	// container's manager
	boxManager *box.Manager
}

// Args sent to the lambda application. Must be synchronized with the
// struct in the agent.
type LambdaArgs struct {
	FunctionName string
	RequestId    string
	Body         []byte
	Arn          string
}

var (
	ErrFunctionAlreadyExist = errors.New("already exist")
	// ErrFunctionNotFound is for when the given function does not exist.
	ErrFunctionNotFound      = errors.New("not found")
	ErrMaxConcurrencyReached = errors.New("max concurrency reached")
)

// New returns a ready to use instance of LLambda, addr must be of the form host:port.
func New(account, region, proto, addr, workdir string) (instance *Instance) {
	return &Instance{
		AccountID:  account,
		Region:     region,
		Proto:      proto,
		Addr:       addr,
		functions:  map[string]*function{},
		boxManager: box.New(workdir),
	}
}

// Process requests from the API.
func (i *Instance) Process(reqC <-chan Request, stopC <-chan struct{}) {

forloop:
	for {
		select {
		case req := <-reqC:
			switch req.action {
			case "CreateFunction":
				i.createFunction(req)
			case "InvokeFunction":
				i.invokeFunction(req)
			default:
				req.resC <- &ReqResult{Err: fmt.Errorf("%q not implemented", req.action)}
				break
			}
		case <-stopC:
			break forloop
		}
	}

	log.Println("Shutting down LLambda...")
}

func (i *Instance) createFunction(req Request) {

	params := req.params.(ReqCreateFunction)

	if _, exists := i.functions[params.FunctionName]; exists {
		req.resC <- &ReqResult{
			Err:     ErrFunctionAlreadyExist,
			ErrData: "function already exist: " + params.FunctionName,
		}
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		req.resC <- &ReqResult{Err: err}
		return
	}
	revID := u.String()

	// TODO: params.FunctionName has to be parsed. it may contain the arn. In case of an ANR, we MUST extract the name ONLY
	fName := params.FunctionName
	lrn := "arn:aws:lambda:" + i.Region + ":" + i.AccountID + ":function:" + fName

	if src, ok := params.Code["ZipFile"]; !ok {
		req.resC <- &ReqResult{Err: fmt.Errorf("unsupported code source %q", src)}
		return
	}

	encodedCode := params.Code["ZipFile"]
	buf := make([]byte, base64.StdEncoding.DecodedLen(len(encodedCode)))

	_, err = base64.StdEncoding.Decode(buf, []byte(params.Code["ZipFile"]))
	if err != nil {
		req.resC <- &ReqResult{Err: err}
		return
	}

	codeSize, err := i.boxManager.CreateBox(
		fName,
		Runtime.imageFile[params.Runtime],
		Runtime.entrypoint[params.Runtime],
		buf,
	)
	if err != nil {
		log.Println(":::::::", err) // TODO: delete
		req.resC <- &ReqResult{Err: err}
		return
	}

	f := function{
		description:   params.Description,
		envVars:       params.Environment.Variables,
		handler:       params.Handler,
		memorySize:    params.MemorySize,
		name:          fName,
		lrn:           lrn,
		publish:       params.Publish,
		revID:         revID,
		role:          params.Role,
		runtime:       params.Runtime,
		version:       "$LATEST",
		idleInstances: list.New(),
	}
	// set default memory value
	if f.memorySize == 0 {
		f.memorySize = 128
	}

	i.functions[f.name] = &f
	codeHash := sha256.Sum256(buf)
	req.resC <- &ReqResult{
		Data: map[string]interface{}{
			"CodeSha256":  fmt.Sprintf("%x", codeHash),
			"CodeSize":    codeSize,
			"Description": f.description,
			"Environment": struct {
				Variables map[string]string
			}{
				f.envVars,
			},
			"FunctionArn":  f.lrn,
			"FunctionName": f.name,
			"MemorySize":   f.memorySize,
			"RevisionId":   f.revID,
			"Role":         f.role,
			"Runtime":      f.runtime,
			"Version":      f.version,
		},
	}
}

func (i *Instance) invokeFunction(req Request) {

	params := req.params.(ReqInvokeFunction)

	function, exist := i.functions[params.FunctionName]
	if !exist {
		req.resC <- &ReqResult{
			Err:     ErrFunctionNotFound,
			ErrData: "Function not found: " + params.FunctionName,
		}
		return
	}

	// TODO
	// for now only supports synchronous calls
	if params.InvocationType != "" && params.InvocationType != "RequestResponse" {
		req.resC <- &ReqResult{
			Err: fmt.Errorf("invocation type %q not implemented", params.InvocationType),
		}
		return
	}

	// TODO
	// for now does not return the output
	if params.LogType != "" && params.LogType != "None" {
		req.resC <- &ReqResult{Err: fmt.Errorf("logtype %q not implemented", params.LogType)}
		return
	}

	// TODO
	// for now don't support versions
	if params.Qualifier != "" {
		req.resC <- &ReqResult{Err: errors.New("versions not implemented")}
		return
	}

	//err := i.functions[params.FunctionName].invoke(
	//	params.Payload, params.Qualifier, params.InvocationType, params.LogType,
	//)
	// TODO: simulate random delay of a lambda being created/invoked?
	time.Sleep(500 * time.Millisecond)

	instanceID := "0" // TODO: generate this... short uuid?
	port := 50123     // TODO: sequential ports on a ring buffer
	var inst *instance
	var errC chan error
	// if no idle instances available, create a new one if didn't reach the limit
	switch elem := function.idleInstances.PullFront(); elem {
	case nil:
		log.Debugln("No instances available, creating....")
		// TODO: when it supports configuring max concurrency on a function, this must be
		//  aware of it

		args := []string{
			strconv.Itoa(port),
			filepath.Join("/", "app", function.handler),
			function.name,
		}
		// add env vars
		for k, v := range function.envVars {
			args = append(args, []string{k, v}...) // TODO: improve this to avoid creating a temporary array
		}

		var err error
		errC, err = i.boxManager.LaunchBoxInstance(
			function.name,
			instanceID,
			// box arguments
			args...,
		)
		if err != nil {
			req.resC <- &ReqResult{Err: err}
			return
		}

		inst = &instance{
			parent: function,
			id:     instanceID,
			port:   port,
		}
	default:
		inst = elem.(*instance)
	}

	defer func() {
		// put instance back to the idle stack
		function.idleInstances.PushFront(inst)
	}()

	log.Debugln("Launching instance", inst.id)

	reply, err := inst.Exec(
		&LambdaArgs{
			FunctionName: function.name,
			RequestId:    req.id,
			Body:         params.Payload,
			Arn:          function.lrn,
		},
	)
	// TODO: handle errors
	if err != nil {
		// if errC has an error, the lambda instance didn't launch properly, hence, override err
		if e := <-errC; e != nil {
			err = e
		}
		req.resC <- &ReqResult{Err: err}
		return
	}

	req.resC <- &ReqResult{
		Data: map[string]string{
			"result":  string(reply.Payload),
			"version": function.version,
		},
	}
}
