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
	Process(<-chan Request, <-chan struct{})
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
	// TODO: align
}

// LambdaArgs sent to the lambda application. Must be synchronized with the
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

// TODO: temporary, til I get the zfs implemented
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
	}, nil
}

// Process requests from the API.
func (l *LLambda) Process(reqC <-chan Request, stopC <-chan struct{}) {
	lifecycleTick := time.NewTicker(time.Second)

forloop:
	for {
		select {
		case req := <-reqC:
			switch req.action {
			case "CreateFunction":
				l.createFunction(req)
			case "InvokeFunction":
				l.invokeFunction(req)
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

	// TODO: params.FunctionName has to be parsed. it may contain the arn. In case of an ANR, we
	//  MUST extract the name ONLY
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

	// TODO
	// for now don't support versions
	if params.Qualifier != "" {
		req.resC <- &ReqResult{Err: errors.New("versions not implemented")}
		return
	}

	// TODO: simulate random delay of a lambda being created/invoked? - should be configurable
	//time.Sleep(500 * time.Millisecond)

	instanceID := strconv.FormatUint(l.instanceCounter, 10)
	l.instanceCounter++

	var fInstance funcInstancer
	var err error
	// if no idle instances available, create a new one if the limit wasn't reached
	switch elem := function.idleInstances.PullFront(); elem {
	case nil:
		log.Debugln("No instances available, creating....")
		// TODO: when it supports configuring max concurrency on a function, this must be
		//  aware of it

		ip := l.ipPool.IPv4()
		fInstance, err = function.Instance(
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
			req.resC <- &ReqResult{Err: err}
			return
		}
	default:
		fInstance = elem.(funcInstancer)
	}

	defer func() {
		if err != nil {
			return
		}
		log.Debugln("Putting instance back to idle", fInstance.ID())
		// put instance back to the idle stack
		function.idleInstances.PushFront(fInstance)
	}()

	log.Debugln("Launching instance", fInstance.ID())

	reply, err := fInstance.Exec(
		&LambdaArgs{
			FunctionName: function.name,
			RequestId:    req.id,
			Body:         params.Payload,
			Arn:          function.lrn,
		},
	)

	if err != nil {
		e := fInstance.Shutdown()
		if e != nil {
			err = fmt.Errorf(
				"failed to init lambda runtime: %s, AND failed to shutdown lambda %q, instance %q: %s",
				err, function.name, fInstance.ID(), e,
			)
		}

		// free ip
		l.ipPool.Release4(fInstance.IP())

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
