package api

import (
	"context"
	"net/url"
	"reflect"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
	"github.com/cprates/lws/pkg/lsqs"
)

var sqsAction = map[string]reflect.Value{}

// InstallSQS installs SQS service and starts a new instance of LSqs.
func (a AwsCli) InstallSQS() {

	api := lsqs.New()
	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		sqsAction[mt.Name] = mv
	}

	a.regService("sqs", sqsDispatcher)
}

func sqsDispatcher(ctx context.Context, reqID string, params url.Values) (res *common.Result) {

	action := params.Get("Action")
	actionM, ok := sqsAction[action]
	if !ok {
		msg := "Not implemented or unknown action " + action
		res = &common.Result{
			Status: 400,
			Err: lerr.Result{
				Result: lerr.Details{
					Type:      "Sender",
					Code:      "InvalidAction",
					Message:   msg,
					RequestID: reqID,
				},
			},
		}
		return
	}

	input := []reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(params),
	}

	rv := actionM.Call(input)
	r := rv[0].Interface().(common.Result)
	r.Err.Result.RequestID = reqID
	res = &r

	return
}
