package lsqs

import (
	"context"
	"encoding/xml"
	"net/url"

	"github.com/cprates/lws/common"
)

// API is the receiver for all LSqs API methods.
type API struct {
	controller LSqs
	pushC      chan request
	stopC      chan struct{}
}

// New creates and launches a LSqs instance.
func New(region, accountID, scheme, host string) *API {

	ctl := &lSqs{
		accountID: accountID,
		region:    region,
		scheme:    scheme,
		host:      host,
		queues:    map[string]*queue{},
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

func (a API) pushReq(action, reqID string, params, attributes map[string]string) *reqResult {

	rq := newReq(action, reqID, params, attributes)
	a.pushC <- rq
	return <-rq.resC
}

func newReq(action, reqID string, params, attributes map[string]string) request {
	return request{
		action:     action,
		id:         reqID,
		params:     params,
		attributes: attributes,
		resC:       make(chan *reqResult),
	}
}

// CreateQueue creates a new queue.
func (a API) CreateQueue(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueName"]; !present {
		return common.ErrMissingParamRes("QueueName is a required parameter", reqID)
	}

	res := a.pushReq("CreateQueue", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrAlreadyExists:
			msg := "A queue already exists with the same name and a different value for attribute(s) " + res.errData.(string)
			return ErrQueueAlreadyExistsRes(msg, reqID)
		case ErrInvalidParameterValue:
			return common.ErrInvalidParameterValueRes(res.errData.(string), reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
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

// DeleteQueue deletes the specified queue on this instance.
func (a API) DeleteQueue(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := a.pushReq("DeleteQueue", reqID, params, attributes)
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

// GetQueueAttributes returns the requested attributes of an specified queue.
func (a API) GetQueueAttributes(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := a.pushReq("GetQueueAttributes", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName                  xml.Name `xml:"GetQueueAttributes"`
		GetQueueAttributesResult struct {
			Attribute []struct {
				Name  string
				Value string
			}
		}
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID
	attrs := res.data.(map[string]string)
	for k, v := range attrs {
		xmlData.GetQueueAttributesResult.Attribute = append(
			xmlData.GetQueueAttributesResult.Attribute,
			struct {
				Name  string
				Value string
			}{k, v},
		)
	}

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// GetQueueUrl returns the URL of an existing Amazon SQS queue.
func (a API) GetQueueUrl(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueName"]; !present {
		return common.ErrMissingParamRes("QueueName is a required parameter", reqID)
	}

	res := a.pushReq("GetQueueUrl", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName           xml.Name `xml:"GetQueueUrlResponse"`
		GetQueueURLResult struct {
			QueueURL string `xml:"QueueUrl"`
		} `xml:"GetQueueUrlResult"`
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID
	xmlData.GetQueueURLResult.QueueURL = res.data.(string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// ListQueues return a datastructs of existing queues on this instance.
func (a API) ListQueues(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	res := a.pushReq("ListQueues", reqID, params, attributes)
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

// ReceiveMessage return a datastructs of messages from the specified queue.
func (a API) ReceiveMessage(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := a.pushReq("ReceiveMessage", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrInvalidParameterValue:
			return common.ErrInvalidParameterValueRes(res.errData.(string), reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
		}
	}

	type Message struct {
		MessageID     string `xml:"MessageId"`
		ReceiptHandle string
		MD5OfBody     string
		Body          string
	}
	xmlData := struct {
		XMLName              xml.Name `xml:"ReceiveMessageResponse"`
		ReceiveMessageResult struct {
			Message []*Message
		}
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID

	messages := res.data.([]*message)
	for _, msg := range messages {
		xmlData.ReceiveMessageResult.Message = append(
			xmlData.ReceiveMessageResult.Message,
			&Message{
				MessageID:     msg.messageID,
				ReceiptHandle: msg.receiptHandle,
				MD5OfBody:     msg.md5OfMessageBody,
				Body:          string(msg.body),
			},
		)
	}

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// SendMessage a message to the specified queue.
func (a API) SendMessage(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	if _, present := params["MessageBody"]; !present {
		return common.ErrMissingParamRes("MessageBody is a required parameter", reqID)
	}
	escaped, err := url.QueryUnescape(params["MessageBody"])
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}
	params["MessageBody"] = escaped

	res := a.pushReq("SendMessage", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrInvalidParameterValue:
			return common.ErrInvalidParameterValueRes(res.errData.(string), reqID)
		case ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName           xml.Name `xml:"SendMessageResponse"`
		SendMessageResult struct {
			MD5OfMessageBody string
			MessageID        string `xml:"MessageId"`
		}
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}

	m := res.data.(map[string]string)
	xmlData.SendMessageResult.MD5OfMessageBody = m["MD5OfMessageBody"]
	xmlData.SendMessageResult.MessageID = m["MessageId"]
	xmlData.ResponseMetadata.RequestID = reqID

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return common.ErrInternalErrorRes(err.Error(), reqID)
	}

	return common.SuccessRes(buf, reqID)
}

// SetQueueAttributes sets the given attributes to the specified queue.
func (a API) SetQueueAttributes(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) common.Result {

	reqID := ctx.Value(common.ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return common.ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := a.pushReq("SetQueueAttributes", reqID, params, attributes)
	if res.err != nil {
		switch res.err {
		case ErrInvalidAttributeName:
			return common.ErrInvalidAttributeNameRes(res.errData.(string), reqID)
		case ErrInvalidParameterValue:
			return common.ErrInvalidParameterValueRes(res.errData.(string), reqID)
		default:
			return common.ErrInternalErrorRes(res.err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName          xml.Name `xml:"SetQueueAttributesResponse"`
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
