package llambda

import (
	"context"

	"github.com/cprates/lws/common"
)

// API is the receiver for all LLambda API methods.
type API struct {
	controller LLambda
	pushC      chan request
	stopC      chan struct{}
}

// New creates and launches a LLambda instance.
func New(region, accountID, scheme, host string) *API {

	ctl := &lLambda{
		accountID: accountID,
		region:    region,
		scheme:    scheme,
		host:      host,
	}
	pushC := make(chan request)
	stopC := make(chan struct{})
	go ctl.Process(pushC, stopC)

	return &API{
		controller: ctl,
		pushC:      pushC,
		stopC:      stopC,
	}
}

// CreateFunction creates a Lambda function. To create a function, you need a deployment package
// and an execution role.
func (a API) CreateFunction(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)
	return common.ErrMissingParamRes("DUMMY", reqID)
}
