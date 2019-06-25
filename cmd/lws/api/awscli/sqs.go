package awscli

import (
	"context"
	"reflect"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lsqs"
)

var sqsAction = map[string]reflect.Value{}

// InstallSQS installs SQS service and starts a new instance of LSqs.
func (a AwsCli) InstallSQS(router *mux.Router, region, account, proto, addr string) {

	log.Println("Installing SQS service")

	api := lsqs.New(region, account, proto, addr)
	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		sqsAction[mt.Name] = mv
	}

	// SQS requests (and SNS) have no distinct path
	router.HandleFunc("/", commonDispatcher(sqsDispatcher))
}

func sqsDispatcher(
	ctx context.Context,
	reqID string,
	method string,
	path string,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	action := params["Action"]
	actionM, ok := sqsAction[action]
	if !ok {
		msg := "Not implemented or unknown action " + action
		return common.ErrInvalidActionRes(msg, reqID)
	}

	input := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(params),
		reflect.ValueOf(attributes),
	}

	rv := actionM.Call(input)
	return rv[0].Interface().(common.Result)
}
