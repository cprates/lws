package lsqs

import (
	"context"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
)

// API is the receiver for all LSqs API methods.
type API struct {
	controller lSqs
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

// New creates an LSqs instance.
func New() *API {
	return &API{}
}

//a.regAction("AddPermission", notImplemented).
//	regAction("ChangeMessageVisibility", notImplemented).
//	regAction("ChangeMessageVisibilityBatch", notImplemented).
//*	regAction("CreateQueue", createQueue).
//	regAction("DeleteMessage", notImplemented).
//	regAction("DeleteMessageBatch", notImplemented).
//	regAction("DeleteQueue", notImplemented).
//	regAction("GetQueueAttributes", notImplemented).
//	regAction("GetQueueUrl", notImplemented).
//	regAction("ListDeadLetterSourceQueues", notImplemented).
//	regAction("ListQueues", notImplemented).
//	regAction("ListQueueTags", notImplemented).
//	regAction("PurgeQueue", notImplemented).
//	regAction("ReceiveMessage", notImplemented).
//	regAction("RemovePermission", notImplemented).
//	regAction("SendMessage", notImplemented).
//	regAction("SendMessageBatch", notImplemented).
//	regAction("SetQueueAttributes", notImplemented).
//	regAction("TagQueue", notImplemented).
//	regAction("UntagQueue", notImplemented)

// CreateQueue creates a new queue.
func (a API) CreateQueue(ctx context.Context, params map[string][]string) common.Result {
	return resNotImplemented
}
