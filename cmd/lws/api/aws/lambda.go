package aws

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/llambda"
)

// LambdaAPI is the receiver for all LLambda API methods.
type LambdaAPI struct {
	instance llambda.LLambdaer
	pushC    chan llambda.Request
	stopC    <-chan struct{}
}

// InstallLambda starts a new instance of LLambda.
func (i Interface) InstallLambda(
	router *mux.Router,
	region, accountID, scheme, host, network, gatewayIP, bridgeIfName, nameServerIP, lambdaWorkdir string,
	stopC <-chan struct{},
	shutdown *sync.WaitGroup,
	logger *log.Entry,
) (
	pushC chan llambda.Request,
	err error,
) {
	log.Println("Installing Lambda service")

	instance, err := llambda.New(
		accountID, region, scheme, host, network, gatewayIP, bridgeIfName, nameServerIP,
		lambdaWorkdir, logger,
	)
	if err != nil {
		return
	}
	pushC = make(chan llambda.Request)
	go func() {
		instance.Process(stopC)
		shutdown.Done()
	}()

	api := &LambdaAPI{
		instance: instance,
		pushC:    instance.PushC,
		stopC:    stopC,
	}
	root := "/2015-03-31/functions"
	router.HandleFunc(root, createFunction(api)).Methods(http.MethodPost)
	router.HandleFunc(
		root+"/{FunctionName}/invocations",
		invokeFunction(api),
	).Methods(http.MethodPost)

	return
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

func onLambdaAlreadyExist(message string, w http.ResponseWriter) {
	err := onLambdaErr(http.StatusConflict, "ResourceConflictException", message, w)
	if err != nil {
		log.Debugln(err)
	}
}

func onLambdaNotFound(message string, w http.ResponseWriter) {
	err := onLambdaErr(http.StatusNotFound, "ResourceNotFoundException", message, w)
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
			log.Errorln("Unexpected error", err)
			return
		}
		reqID := u.String()

		log.Debugf("Req %s %q, %s", r.Method, r.RequestURI, reqID)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Errorln("Failed to read body,", reqID, err)
			return
		}

		params := llambda.ReqCreateFunction{}
		err = json.Unmarshal(body, &params)
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Errorln("Failed to read query params,", reqID, err)
			return
		}

		// performs some basic check on parameters
		if len(params.Code) == 0 {
			onLambdaInvalidParameterValue("Code is a required parameter", w)
			return
		} else if len(params.Description) > 256 {
			onLambdaInvalidParameterValue("Description has a maximum length of 256", w)
			return
		} else if len(params.FunctionName) == 0 || len(params.FunctionName) > 140 {
			onLambdaInvalidParameterValue("FunctionName is a required parameter", w)
			return
		} else if params.MemorySize != 0 && (params.MemorySize < 128 || params.Timeout > 3072) {
			onLambdaInvalidParameterValue("Memory size must be >= 128MB and <= 3072MB", w)
			return
		} else if len(params.Handler) > 128 {
			onLambdaInvalidParameterValue("Handler has a maximum length of 128", w)
			return
		} else if len(params.Role) == 0 {
			onLambdaInvalidParameterValue("Role is a required parameter", w)
			return
		} else if _, ok := llambda.Runtime[params.Runtime]; !ok {
			var sp []string
			for k := range llambda.Runtime {
				sp = append(sp, k)
			}
			onLambdaInvalidParameterValue(
				"Runtime supported:"+strings.Join(sp, ","), w,
			)
			return
		} else if params.Timeout < 0 && params.Timeout <= 900 {
			onLambdaInvalidParameterValue("Timeout must be >= 1 and <= 900", w)
			return
		}

		// handle defaults
		if params.Timeout == 0 {
			params.Timeout = 3
		}
		if params.MemorySize == 0 {
			params.MemorySize = 128
		}

		functionName := parseFuncName(params.FunctionName)
		if functionName == "" {
			onLambdaInvalidParameterValue("invalid FunctionName: "+params.FunctionName, w)
			return
		}
		params.FunctionName = functionName

		log.Debugf("Creating function %q, %s", params.FunctionName, reqID)

		lambdaRes := llambda.PushReq(api.pushC, "CreateFunction", reqID, params)
		if lambdaRes.Err != nil {
			log.Debugln(reqID, lambdaRes.Err, lambdaRes.ErrData)

			switch lambdaRes.Err {
			case llambda.ErrFunctionAlreadyExist:
				onLambdaAlreadyExist(lambdaRes.ErrData.(string), w)
			default:
				onLambdaInternalError(lambdaRes.Err.Error(), w)
			}
			return
		}

		buf, err := json.Marshal(lambdaRes.Data)
		if err != nil {
			log.Errorln(reqID, err)
			onLambdaInternalError(err.Error(), w)
			return
		}

		w.WriteHeader(201)
		_, err = w.Write(buf)
		if err != nil {
			log.Errorln("Unexpected error, request", reqID, err)
		}
	}
}

func invokeFunction(api *LambdaAPI) http.HandlerFunc {
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

		payload, err := ioutil.ReadAll(r.Body)
		if err != nil {
			onLambdaInternalError(err.Error(), w)
			log.Debugln("Failed to read body,", reqID, err)
			return
		}

		nameParam, exists := mux.Vars(r)["FunctionName"]
		if !exists {
			onLambdaInvalidParameterValue("FunctionName is a required parameter", w)
			return
		}

		functionName := parseFuncName(nameParam)
		if functionName == "" {
			onLambdaInvalidParameterValue("invalid FunctionName: "+nameParam, w)
			return
		}

		headers, _ := flattAndParse(r.Header)
		query, _ := flattAndParse(r.URL.Query())

		qualifier, present := query["Qualifier"]
		// TODO: need better check, pattern: (|[a-zA-Z0-9$_-]+)
		if present && (len(qualifier) < 1 || len(qualifier) > 128) {
			onLambdaInvalidParameterValue("Qualifier must be between 1 and 128 characters", w)
			return
		}

		invocationType, present := headers["X-Amz-Invocation-Type"]
		invocationTypes := []string{"Event", "RequestResponse", "DryRun"}
		if present && !stringInSlice(invocationType, invocationTypes) {
			onLambdaInvalidParameterValue(
				"InvocationType mut be "+strings.Join(invocationTypes, " | "),
				w,
			)
			return
		}

		logType, present := headers["X-Amz-Log-Type"]
		logTypes := []string{"None", "Tail"}
		if present && !stringInSlice(logType, logTypes) {
			onLambdaInvalidParameterValue(
				"InvocationType mut be "+strings.Join(logTypes, " | "),
				w,
			)
			return
		}

		params := llambda.ReqInvokeFunction{
			FunctionName:   functionName,
			Qualifier:      qualifier,
			InvocationType: invocationType,
			LogType:        logType,
			Payload:        payload,
		}

		log.Debugf("Invoking function %q, %s", params.FunctionName, reqID)

		lambdaRes := llambda.PushReq(api.pushC, "InvokeFunction", reqID, params)
		if lambdaRes.Err != nil {
			log.Errorln(reqID, lambdaRes.Err, lambdaRes.ErrData)

			switch lambdaRes.Err {
			case llambda.ErrFunctionNotFound:
				onLambdaNotFound(lambdaRes.ErrData.(string), w)
			default:
				onLambdaInternalError(lambdaRes.Err.Error(), w)
			}
			return
		}

		res := lambdaRes.Data.(map[string]string)

		resPayload := []byte(res["result"])
		if resErr, ok := res["error"]; ok {
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			resPayload = []byte(resErr)
		}

		if logType == "Tail" { // TODO: and no error...
			//w.Header().Set("X-Amz-Log-Result", string(buf))
		}

		w.Header().Set("X-Amz-Executed-Version", res["version"])
		w.WriteHeader(200)

		_, err = w.Write(resPayload)
		if err != nil {
			log.Errorln("Unexpected error, request", reqID, err)
		}
	}
}

func parseFuncName(s string) string {
	parts := strings.Split(s, ":")

	switch len(parts) {
	case 1:
		// Function name - my-function
		if len(parts[0]) > 64 {
			return ""
		}
		return parts[0]
	case 7:
		// Function ARN - arn:aws:lambda:us-west-2:123456789012:function:my-function
		return parts[6]
	case 2:
		// Partial ARN - 123456789012:function:my-function
		return parts[2]
	default:
		return ""
	}
}
