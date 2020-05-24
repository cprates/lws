package llambda

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cprates/ippool"
	"github.com/cprates/lws/pkg/list"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// LLambdaer is the interface to implement if you want to implement your own Lambda service.
type LLambdaer interface {
	Process(<-chan struct{})
}

// LLambda represents an instance of lLambda core.
type LLambda struct {
	AccountID string
	Region    string
	Proto     string
	Addr      string
	Workdir   string

	functions       map[string]*function // by function name
	instanceCounter uint64               // used as lambda instance ids
	logger          *log.Entry
	ipPool          *ippool.Dhcpc
	ipNet           *net.IPNet
	bridgeIfName    string
	gatewayIP       string
	nameServerIP    string

	PushC chan Request
}

// LambdaArgs sent to the lambda application. Must be synchronized with the
// struct in the agent.
type LambdaArgs struct {
	FunctionName string
	RequestId    string
	Body         []byte
	Arn          string
}

type reqSetIdle struct {
	functionName string
	instance     funcInstancer
}

var (
	ErrFunctionAlreadyExist = errors.New("already exist")
	// ErrFunctionNotFound is for when the given function does not exist.
	ErrFunctionNotFound      = errors.New("not found")
	ErrMaxConcurrencyReached = errors.New("max concurrency reached")
)

// Runtime contains all supported runtimes along with some configs.
var Runtime = map[string]struct {
	imageFile  string
	entrypoint string
}{
	"go1.x": {
		imageFile:  "golang_base.tar",
		entrypoint: "/bin/gobox",
	},
}

// TODO: temporary, till zfs is not in place
// name of the file containing the code to run in a box instance
const codeFileName = "code.zip"

// New returns a ready to use instance of LLambda, addr must be of the form host:port.
func New(
	account, region, proto, addr, network, gatewayIP, bridgeIfName, nameServerIP, workdir string,
	logger *log.Entry,
) (
	*LLambda, error,
) {
	if err := initTemplates(); err != nil {
		return nil, fmt.Errorf("unable to parse templates: %s", err)
	}

	ipPool, err := ippool.New(network)
	if err != nil {
		return nil, err
	}
	// TODO: burn the first ip because the bridge iface is using the first one. This can be
	//  fixed after the network scheme is sorted
	ipPool.IPv4()
	ipPool.IPv4()

	_, ipNet, _ := net.ParseCIDR(network)

	return &LLambda{
		AccountID:    account,
		Region:       region,
		Proto:        proto,
		Addr:         addr,
		Workdir:      workdir,
		functions:    map[string]*function{},
		logger:       logger,
		ipPool:       ipPool,
		ipNet:        ipNet,
		bridgeIfName: bridgeIfName,
		gatewayIP:    gatewayIP,
		nameServerIP: nameServerIP,
		PushC:        make(chan Request),
	}, nil
}

// Process requests from the API.
func (l *LLambda) Process(stopC <-chan struct{}) {
	lifecycleTick := time.NewTicker(time.Second)

forloop:
	for {
		select {
		case req := <-l.PushC:
			switch req.action {
			case "CreateFunction":
				l.createFunction(req)
			case "InvokeFunction":
				l.invokeFunction(req)
			case "setIdle":
				l.setIdle(req)
			default:
				req.resC <- &ReqResult{Err: fmt.Errorf("%q not implemented", req.action)}
				break
			}
		case <-lifecycleTick.C:
			// TODO: because handleLifecycle is doing operations that may take a while like
			//  inst.Shutdown(), this can be a problem because while this function is executing
			//  the service can not serve requests from clients...
			//  To avoid this, this function could be running on its own in a separate goroutine
			//  and if we do that, the access to function.idleInstances MUST be thread safe
			l.handleLifecycle()
		case <-stopC:
			break forloop
		}
	}

	log.Println("Shutting down LLambda...")
	lifecycleTick.Stop()
}

func (l *LLambda) handleLifecycle() {
	for _, function := range l.functions {
		var toRemove []*list.Element
		// safe to access function.idleInstances because only one action is processed at a time
		for elem := function.idleInstances.Front(); elem != nil; elem = elem.Next() {
			inst := elem.Value.(funcInstancer)
			if time.Now().Before(inst.LifeDeadline()) {
				continue
			}

			log.Debugf(
				"Destroying instance %q, lambda %q, after being idle for %d seconds",
				inst.ID(), function.name, function.timeout,
			)

			err := inst.Shutdown()
			if err != nil {
				log.Errorf(
					"Failed to shutdown lambda %q, instance %q: %s",
					function.name, inst.ID(), err,
				)
			}
			// free ip
			l.ipPool.Release4(inst.IP())

			toRemove = append(toRemove, elem)
		}

		for _, elem := range toRemove {
			function.idleInstances.Remove(elem)
		}
	}
}

func (l *LLambda) createFunction(req Request) {
	params := req.params.(ReqCreateFunction)

	if _, exists := l.functions[params.FunctionName]; exists {
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

	fName := params.FunctionName
	lrn := "arn:aws:lambda:" + l.Region + ":" + l.AccountID + ":function:" + fName

	if src, ok := params.Code["ZipFile"]; !ok {
		req.resC <- &ReqResult{Err: fmt.Errorf("unsupported code source %q", src)}
		return
	}

	buf, err := base64.StdEncoding.DecodeString(params.Code["ZipFile"])
	if err != nil {
		req.resC <- &ReqResult{Err: err}
		return
	}

	fFolder := path.Join(l.Workdir, fName)
	f := function{
		folder:            fFolder,
		runtimeImage:      path.Join(l.Workdir, Runtime[params.Runtime].imageFile),
		runtimeEntryPoint: Runtime[params.Runtime].entrypoint,
		usercodeFile:      filepath.Join(l.Workdir, fName, codeFileName),
		description:       params.Description,
		envVars:           params.Environment.Variables,
		handler:           filepath.Join("/", "app", params.Handler),
		memorySize:        params.MemorySize,
		name:              fName,
		lrn:               lrn,
		publish:           params.Publish,
		revID:             revID,
		role:              params.Role,
		timeout:           params.Timeout,
		version:           "$LATEST",
		idleInstances:     list.New(),
		containerManager:  &cmanager{workdir: fFolder},
		bridgeIfName:      l.bridgeIfName,
		gatewayIP:         l.gatewayIP,
		nameServerIP:      l.nameServerIP,
		logger:            l.logger,
	}

	codeSize, err := f.Init(bytes.NewReader(buf))
	if err != nil {
		req.resC <- &ReqResult{Err: err}
		return
	}

	l.functions[f.name] = &f
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
			"Runtime":      params.Runtime,
			"Timeout":      f.timeout,
			"Version":      f.version,
		},
	}
}

func (l *LLambda) invokeFunction(req Request) {
	params := req.params.(ReqInvokeFunction)

	function, exist := l.functions[params.FunctionName]
	if !exist {
		req.resC <- &ReqResult{
			Err:     ErrFunctionNotFound,
			ErrData: "function not found: " + params.FunctionName,
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

	// TODO: add versions support
	if params.Qualifier != "" {
		req.resC <- &ReqResult{Err: errors.New("versions not implemented")}
		return
	}

	// TODO: simulate random delay of a lambda being created/invoked? - should be configurable
	//time.Sleep(500 * time.Millisecond)

	var fInstance funcInstancer
	var newInstanceID string

	elem := function.idleInstances.PullFront()
	if elem == nil {
		newInstanceID = strconv.FormatUint(l.instanceCounter, 10)
		l.instanceCounter++
	} else {
		fInstance = elem.(funcInstancer)
	}

	// lengthy operations go in a separate goroutine
	go func() {
		req.resC <- l.parallelInvoke(function, fInstance, newInstanceID, req.id, params.Payload)
	}()
}

// parallelInvoke holds all the invocation code that is safe to run concurrently.
func (l *LLambda) parallelInvoke(
	f *function, fInstance funcInstancer, instanceID, reqID string, payload []byte,
) *ReqResult {
	var err error
	// if no idle instances available, create a new one if the limit wasn't reached
	if fInstance == nil {
		// TODO: when it supports configuring max concurrency on a function, this must be
		//  aware of it. This control needs to be done thread safe
		log.Debugln("No instances available, creating new....")

		ip := l.ipPool.IPv4()
		if ip == nil {
			err = fmt.Errorf("unable to acquire IP for lambda %s[%s]: %s", f.name, instanceID, err)
			return &ReqResult{Err: err}
		}

		fInstance, err = f.Instance(
			instanceID,
			ip,
			l.ipNet,
			// TODO: pass std io properly
			os.Stdin,
			os.Stdout,
			os.Stderr,
			runtimeListenPort,
		)
		if err != nil {
			err = fmt.Errorf("creating new lambda %s[%s]: %s", f.name, instanceID, err)
			// free ip on error
			l.ipPool.Release4(ip)
			return &ReqResult{Err: err}
		}
	}

	log.Debugf("Launching lambda instance %s[%s]", f.name, fInstance.ID())

	reply, err := fInstance.Exec(
		&LambdaArgs{
			FunctionName: f.name,
			RequestId:    reqID,
			Body:         payload,
			Arn:          f.lrn,
		},
	)
	if err != nil {
		err = fmt.Errorf("failed to init lambda %s[%s] runtime: %s", f.name, fInstance.ID(), err)
		if e := fInstance.Shutdown(); e != nil {
			err = fmt.Errorf("%s, AND failed to shut it down: %s", err, e)
		}

		// free ip on error
		l.ipPool.Release4(fInstance.IP())

		return &ReqResult{Err: err}
	}

	log.Debugf("Putting lambda %s[%s] back to idle\n", f.name, fInstance.ID())
	// put instance back to the idle stack
	l.PushC <- Request{
		action: "setIdle",
		params: reqSetIdle{
			functionName: f.name,
			instance:     fInstance,
		},
	}

	return &ReqResult{
		Data: map[string]string{
			"result":  string(reply.Payload),
			"version": f.version,
		},
	}
}

func (l *LLambda) setIdle(req Request) {
	params := req.params.(reqSetIdle)

	function, exist := l.functions[params.functionName]
	if !exist {
		panic("function must exist")
	}
	function.idleInstances.PushFront(params.instance)
}
