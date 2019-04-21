package lsqs

import (
	"context"
	"encoding/xml"

	"github.com/cprates/lws/common"
)

// API is the receiver for all LSqs API methods.
type API struct {
	controller LSqs
	pushC      chan request
}

// New creates an LSqs instance.
func New(region, accountID, scheme, host string) *API {

	ctl := &lSqs{
		accountID: accountID,
		region:    region,
		scheme:    scheme,
		host:      host,
		queues:    map[string]*queue{},
	}
	pushC := make(chan request)
	go ctl.Process(pushC)

	return &API{
		controller: ctl,
		pushC:      pushC,
	}
}

func (a API) pushReq(action, reqID string, params map[string]string) *reqResult {

	rq := newReq(action, reqID, params)
	a.pushC <- rq
	return <-rq.resC
}

//a.regAction("AddPermission", notImplemented).
//	regAction("ChangeMessageVisibility", notImplemented).
//	regAction("ChangeMessageVisibilityBatch", notImplemented).
//*	regAction("CreateQueue", createQueue).
//	regAction("DeleteMessage", notImplemented).
//	regAction("DeleteMessageBatch", notImplemented).
//*	regAction("DeleteQueue", notImplemented).
//	regAction("GetQueueAttributes", notImplemented).
//	regAction("GetQueueUrl", notImplemented).
//	regAction("ListDeadLetterSourceQueues", notImplemented).
//*	regAction("ListQueues", notImplemented).
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
func (a API) CreateQueue(ctx context.Context, params map[string]string) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueName"]; !present {
		return common.MissingParamRes("QueueName is a required parameter", reqID)
	}

	res := a.pushReq("CreateQueue", reqID, params)
	if res.err != nil {
		switch res.err {
		case ErrAlreadyExists:
			msg := "A queue already exists with the same name and a different value for attribute(s) " + res.errData.(string)
			return ErrQueueAlreadyExistsRes(msg, reqID)
		}
	}

	xmlData := struct {
		XMLName           xml.Name `xml:"CreateQueueResponse"`
		CreateQueueResult struct {
			QueueURL string `xml:"QueueUrl"`
		}
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID
	xmlData.CreateQueueResult.QueueURL = res.data.(string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// ListQueues return a list o existing queues on this instance.
func (a API) ListQueues(ctx context.Context, params map[string]string) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	res := a.pushReq("ListQueues", reqID, params)
	if res.err != nil {
		return common.ErrInternalErrorRes(res.err.Error(), reqID)
	}

	xmlData := struct {
		XMLName          xml.Name `xml:"ListQueuesResponse"`
		ListQueuesResult struct {
			QueueURL []string `xml:"QueueUrl"`
		}
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID
	xmlData.ListQueuesResult.QueueURL = res.data.([]string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// DeleteQueue deletes the specified queue on this instance.
func (a API) DeleteQueue(ctx context.Context, params map[string]string) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.MissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := a.pushReq("DeleteQueue", reqID, params)
	if res.err != nil {
		return common.ErrInternalErrorRes(res.err.Error(), reqID)
	}

	xmlData := struct {
		XMLName          xml.Name `xml:"DeleteQueueResponse"`
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}
