package lsns

import (
	"context"
	"encoding/xml"

	"github.com/cprates/lws/common"
)

// API is the receiver for all LSns API methods.
type API struct {
	controller LSns
	pushC      chan request
}

// New creates an LSns instance.
func New(region, accountID, scheme, host string) *API {
	ctl := &lSns{
		accountID: accountID,
		region:    region,
		scheme:    scheme,
		host:      host,
	}
	pushC := make(chan request)
	go ctl.Process(pushC)

	return &API{
		controller: ctl,
		pushC:      pushC,
	}
}

func (a API) pushReq(action, reqID string, data map[string]string) common.Result {

	rq := newReq(action, reqID, data)
	a.pushC <- rq
	r := <-rq.resC

	buf, err := xml.Marshal(r)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

//a.regAction("AddPermission", notImplemented).
//	regAction("CheckIfPhoneNumberIsOptedOut", notImplemented).
//	regAction("ConfirmSubscription", notImplemented).
//	regAction("CreatePlatformApplication", notImplemented).
//	regAction("CreatePlatformEndpoint", notImplemented).
//*	regAction("CreateTopic", notImplemented).
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

// CreateTopic creates a new topic.
func (a API) CreateTopic(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := "REQ-NOT-PRESENT" // TODO

	if _, present := params["Name"]; !present {
		return common.ErrMissingParamRes("TopicName is a required parameter", reqID)
	}

	return a.pushReq("CreateTopic", reqID, params)
}
