package lsns

import (
	"context"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
)

// API is the receiver for all LSns API methods.
type API struct {
	controller lSns
}

var resNotImplemented = common.Result{
	Status: 400,
	Err: lerr.Result{
		// note the absence of a request ID - it is set by the caller
		Result: lerr.Details{
			Type:    "Sender",
			Code:    "InvalidAction",
			Message: "not implemented",
		},
	},
}

// New creates an LSns instance.
func New() *API {
	return &API{}
}

//a.regAction("AddPermission", notImplemented).
//	regAction("CheckIfPhoneNumberIsOptedOut", notImplemented).
//	regAction("ConfirmSubscription", notImplemented).
//	regAction("CreatePlatformApplication", createQueue).
//	regAction("CreatePlatformEndpoint", notImplemented).
//	regAction("CreateTopic", notImplemented).
//	regAction("DeleteEndpoint", notImplemented).
//	regAction("DeletePlatformApplication", notImplemented).
//	regAction("DeleteTopic", notImplemented).
//	regAction("GetEndpointAttributes", notImplemented).
//	regAction("GetPlatformApplicationAttributes", notImplemented).
//	regAction("GetSMSAttributes", notImplemented).
//	regAction("GetSubscriptionAttributes", notImplemented).
//	regAction("GetTopicAttributes", notImplemented).
//	regAction("ListEndpointsByPlatformApplication", notImplemented).
//	regAction("ListPhoneNumbersOptedOut", notImplemented).
//	regAction("ListPlatformApplications", notImplemented).
//	regAction("ListSubscriptions", notImplemented).
//	regAction("ListSubscriptionsByTopic", notImplemented).
//	regAction("ListTopics", notImplemented).
//	regAction("OptInPhoneNumber", notImplemented).
//	regAction("Publish", notImplemented).
//	regAction("RemovePermission", notImplemented).
//	regAction("SetEndpointAttributes", notImplemented).
//	regAction("SetPlatformApplicationAttributes", notImplemented).
//	regAction("SetSMSAttributes", notImplemented).
//	regAction("SetSubscriptionAttributes", notImplemented).
//	regAction("SetTopicAttributes", notImplemented).
//	regAction("Subscribe", notImplemented).
//	regAction("Unsubscribe", notImplemented)

// CreateTopic creates a new queue.
func (a API) CreateTopic(ctx context.Context, params map[string][]string) common.Result {
	return resNotImplemented
}
