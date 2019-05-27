package api

import (
	"context"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lsns"
)

var snsAction = map[string]reflect.Value{}

// InstallSNS installs SNS service and starts a new instance of LSns.
func (a AwsCli) InstallSNS(region, accountID, scheme, host string) {

	log.Println("Installing SNS service")

	api := lsns.New(region, accountID, scheme, host)
	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		snsAction[mt.Name] = mv
	}

	a.regService("sns", snsDispatcher)
}

func snsDispatcher(
	ctx context.Context,
	reqID string,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	action := params["Action"]
	actionM, ok := snsAction[action]
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
