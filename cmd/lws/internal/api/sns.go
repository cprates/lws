package api

import (
	"context"
	"net/url"
	"reflect"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
	"github.com/cprates/lws/pkg/lsns"
)

var snsAction = map[string]reflect.Value{}

// InstallSNS installs SNS service and starts a new instance of LSns.
func (a AwsCli) InstallSNS() {

	api := lsns.New()
	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		snsAction[mt.Name] = mv
	}

	a.regService("sns", snsDispatcher)
}

func snsDispatcher(ctx context.Context, reqID string, params url.Values) (res *common.Result) {

	action := params.Get("Action")
	actionM, ok := snsAction[action]
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
