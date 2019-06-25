package awscli

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/llambda"
)

// LambdaAPI is the receiver for all LLambda API methods.
type LambdaAPI struct {
	instance llambda.LLambda
	pushC    chan llambda.Request
	stopC    chan struct{}
}

var runtimes = []string{
	"go1.x",
}

func (l LambdaAPI) pushReq(action, reqID string, params interface{}) *llambda.ReqResult {

	rq := newReq(action, reqID, params)
	l.pushC <- rq
	return <-rq.ResC
}

func newReq(action, reqID string, params interface{}) llambda.Request {
	return llambda.Request{
		Action: action,
		ID:     reqID,
		Params: params,
		ResC:   make(chan *llambda.ReqResult),
	}
}

// NewLLambdaInstance creates and launches a LLambda instance.
func NewLLambdaInstance(region, accountID, proto, host, codePath string) *LambdaAPI {

	instance := llambda.New(accountID, region, proto, host, codePath)
	pushC := make(chan llambda.Request)
	stopC := make(chan struct{})
	go instance.Process(pushC, stopC)

	return &LambdaAPI{
		instance: instance,
		pushC:    pushC,
		stopC:    stopC,
	}
}

// InstallLambda installs Lambda service and starts a new instance of LLambda.
func (a AwsCli) InstallLambda(
	router *mux.Router,
	region, accountID, scheme, host, codePath string,
) {

	log.Println("Installing Lambda service")

	root := "/2015-03-31/functions"
	api := NewLLambdaInstance(region, accountID, scheme, host, codePath)
	router.HandleFunc(root, createFunction(api)).Methods(http.MethodPost)
	router.HandleFunc(
		root+"/{FunctionName}/invocations",
		invokeFunction(api),
	).Methods(http.MethodPost)
}

func onLambdaErr(statusCode int, code, message string, w http.ResponseWriter) error {

	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)

	e := struct {
		Code    string
		Message string
	}{
		code,
		message,
	}

	return enc.Encode(e)
}

func onLambdaInternalError(message string, w http.ResponseWriter) {
	err := onLambdaErr(http.StatusInternalServerError, "ServiceException", message, w)
	if err != nil {
		log.Debugln(err)
	}
}

func onLambdaInvalidParameterValue(message string, w http.ResponseWriter) {
	err := onLambdaErr(http.StatusBadRequest, "InvalidParameterValue", message, w)
	if err != nil {
		log.Debugln(err)
	}
}

// createFunction creates a Lambda function. To create a function, you need a deployment package
// and an execution role.
func createFunction(api *LambdaAPI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add("Content-Type", "application/json")

		u, err := uuid.NewRandom()
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Debugln("Unexpected error", err)
			return
		}
		reqID := u.String()

		log.Debugf("Req %s %q, %s", r.Method, r.RequestURI, reqID)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Debugln("Failed to read body,", reqID, err)
			return
		}

		params := llambda.ReqCreateFunction{}
		err = json.Unmarshal(body, &params)
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Debugln("Failed to read query params,", reqID, err)
			return
		}

		// performs some basic check on parameters
		if len(params.Code) == 0 {
			onLambdaInvalidParameterValue("Code is a required parameter", w)
			return
		} else if len(params.Description) > 256 {
			onLambdaInvalidParameterValue("Description has a maximum length of 256", w)
			return
		} else if len(params.FunctionName) == 0 {
			// TODO: needs a better check
			onLambdaInvalidParameterValue("FunctionName is a required parameter", w)
			return
		} else if len(params.Description) > 128 {
			onLambdaInvalidParameterValue("Handler has a maximum length of 128", w)
			return
		} else if len(params.Role) == 0 {
			onLambdaInvalidParameterValue("Role is a required parameter", w)
			return
		} else if !stringInSlice(params.Runtime, runtimes) {
			onLambdaInvalidParameterValue("Runtime supported:"+strings.Join(runtimes, ","), w)
			return
		}

		log.Debugf("Creating function %q, %s", params.FunctionName, reqID)

		lambdaRes := api.pushReq("CreateFunction", reqID, params)
		if lambdaRes.Err != nil {
			log.Debugln(reqID, lambdaRes.Err, lambdaRes.ErrData)

			switch lambdaRes.Err {
			case llambda.ErrResourceConflict:
				e := onLambdaErr(
					http.StatusConflict, lambdaRes.Err.Error(), lambdaRes.ErrData.(string), w,
				)
				if e != nil {
					log.Debugln(e)
				}
			default:
				onLambdaInternalError(lambdaRes.Err.Error(), w)
			}
			return
		}

		buf, err := json.Marshal(lambdaRes.Data)
		if err != nil {
			log.Debugln(reqID, err)
			onLambdaInternalError(err.Error(), w)
			return
		}

		reqRes := common.SuccessRes(buf, reqID)
		w.WriteHeader(201)
		_, err = w.Write(reqRes.Result)
		if err != nil {
			log.Debugln("Unexpected error, request", reqID, err)
		}
	}
}

func invokeFunction(api *LambdaAPI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}
