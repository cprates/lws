package api

import (
	"context"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lsqs"
)

var sqsAction = map[string]reflect.Value{}

// InstallSQS installs SQS service and starts a new instance of LSqs.
func (a AwsCli) InstallSQS(region, accountID, scheme, host string) {

	log.Println("Installing SQS service")

	api := lsqs.New(region, accountID, scheme, host)
	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		sqsAction[mt.Name] = mv
	}

	a.regService("sqs", sqsDispatcher)
}

func sqsDispatcher(ctx context.Context, reqID string, params map[string]string) common.Result {

	action := params["Action"]
	actionM, ok := sqsAction[action]
	if !ok {
		msg := "Not implemented or unknown action " + action
		return common.ErrInvalidActionRes(msg, reqID)
	}

	input := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(params),
	}

	rv := actionM.Call(input)
	return rv[0].Interface().(common.Result)
}
