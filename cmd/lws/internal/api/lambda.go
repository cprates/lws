package api

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
}

// createFunction creates a Lambda function. To create a function, you need a deployment package
// and an execution role.
func createFunction(api *LambdaAPI) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := uuid.NewRandom()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Unexpected error", err)
			return
		}
		reqID := u.String()

		log.Debugf("Req %s %q, %s", r.Method, r.RequestURI, reqID)

		w.Header().Add("Content-Type", "application/xml")

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Failed to read body,", reqID, err)
			return
		}

		params := llambda.ReqCreateFunction{}
		err = json.Unmarshal(body, &params)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Failed to read query params,", reqID, err)
			return
		}

		// performs some basic check on parameters
		var reqRes common.Result
		if len(params.Code) == 0 {
			reqRes = common.ErrMissingParamRes("Code is a required parameter", reqID)
		} else if len(params.Description) > 256 {
			reqRes = common.ErrInvalidParameterValueRes(
				"Description has a maximum length of 256", reqID,
			)
		} else if len(params.FunctionName) == 0 {
			// TODO: needs a better check
			reqRes = common.ErrMissingParamRes("FunctionName is a required parameter", reqID)
		} else if len(params.Description) > 128 {
			reqRes = common.ErrInvalidParameterValueRes(
				"Handler has a maximum length of 128", reqID,
			)
		} else if len(params.Role) == 0 {
			reqRes = common.ErrMissingParamRes("Role is a required parameter", reqID)
		} else if !stringInSlice(params.Runtime, runtimes) {
			reqRes = common.ErrMissingParamRes(
				"Runtime supported:"+strings.Join(runtimes, ","),
				reqID,
			)
		}

		if reqRes.Status != 0 {
			log.Debugln("Failed creating function", reqID, reqRes.Err)
			onLwsErr(w, reqRes)
			return
		}

		log.Debugf("Creating function %q, %s", params.FunctionName, reqID)

		lambdaRes := api.pushReq("CreateFunction", reqID, params)
		if lambdaRes.Err != nil {
			log.Debugln(reqID, lambdaRes.Err)
			onLwsErr(w, common.ErrInternalErrorRes(lambdaRes.Err.Error(), reqID))
			return
		}

		buf, err := json.Marshal(lambdaRes.Data)
		if err != nil {
			log.Debugln(reqID, reqRes.Err)
			onLwsErr(w, common.ErrInternalErrorRes(err.Error(), reqID))
			return
		}

		reqRes = common.SuccessRes(buf, reqID)
		w.WriteHeader(201)
		_, err = w.Write(reqRes.Result)
		if err != nil {
			log.Errorln("Unexpected error, request", reqID, err)
		}
	}
}
