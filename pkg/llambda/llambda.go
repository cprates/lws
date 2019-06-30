package llambda

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// LLambda is the interface to implement if you want to implement your own Lambda service.
type LLambda interface {
	Process(<-chan Request, <-chan struct{})
}

// Instance represents an instance of lLambda core.
type Instance struct {
	AccountID string
	Region    string
	Proto     string
	Host      string

	codePath  string
	functions map[string]*function // by function name
	// TODO: align
}

var (
	// ErrResourceConflict maps to ResourceConflictException
	ErrResourceConflict = errors.New("ResourceConflictException")
)

// New returns a ready to use instance of LLambda.
func New(account, region, proto, host, codePath string) *Instance {
	return &Instance{
		AccountID: account,
		Region:    region,
		Proto:     proto,
		Host:      host,
		codePath:  codePath,
		functions: map[string]*function{},
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
			Err:     ErrResourceConflict,
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

	fName := params.FunctionName // TODO: params.FunctionName has to be parsed. it may contain the arn
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

	codeFolder := path.Join(i.codePath, fName)
	codeSize, err := storeFunctionCode(codeFolder, "code.zip", buf)
	if err != nil {
		req.resC <- &ReqResult{Err: err}
		return
	}

	f := function{
		codeFolder:  codeFolder,
		description: params.Description,
		envVars:     params.Environment.Variables,
		handler:     params.Handler,
		memorySize:  params.MemorySize,
		name:        fName,
		lrn:         lrn,
		publish:     params.Publish,
		revID:       revID,
		role:        params.Role,
		runtime:     params.Runtime,
		version:     "$LATEST",
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

func storeFunctionCode(dstDir, fileName string, code []byte) (n int, err error) {

	if _, err = os.Stat(dstDir); os.IsNotExist(err) {
		err = os.MkdirAll(dstDir, 0766)
		if err != nil {
			return
		}
	}

	f, err := os.OpenFile(path.Join(dstDir, fileName), os.O_CREATE|os.O_WRONLY, 0766)
	if err != nil {
		return
	}

	defer func() {
		e := f.Close()
		if err == nil && e != nil {
			err = e
		}
	}()

	return f.Write(code)
}
