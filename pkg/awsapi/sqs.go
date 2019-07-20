package awsapi

import (
	"context"
	"encoding/xml"
	"net/url"
	"reflect"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/lerr"
	"github.com/cprates/lws/pkg/lsqs"
)

// SqsAPI is the receiver for all SQS API methods.
type SqsAPI struct {
	instance lsqs.LSqs
	pushC    chan lsqs.Request
	stopC    chan struct{}
}

var sqsAction = map[string]reflect.Value{}

// InstallSQS installs SQS service and starts a new instance of LSqs.
func (a AwsAPI) InstallSQS(router *mux.Router, region, account, proto, addr string) {

	log.Println("Installing SQS service")

	instance := lsqs.New(account, region, proto, addr)
	pushC := make(chan lsqs.Request)
	stopC := make(chan struct{})
	go instance.Process(pushC, stopC)

	api := &SqsAPI{
		instance: instance,
		pushC:    pushC,
		stopC:    stopC,
	}

	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		sqsAction[mt.Name] = mv
	}

	router.HandleFunc("/queue/{QueueName}", a.commonDispatcher(sqsDispatcher))
	router.HandleFunc("/", a.commonDispatcher(sqsDispatcher))
}

// ErrNonExistentQueueRes is for generate a result when the specified queue doesn't exist.
func ErrNonExistentQueueRes(reqID string) Response {
	return Response{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "AWS.SimpleQueueService.NonExistentQueue",
				Message:   "The specified queue does not exist.",
				RequestID: reqID,
			},
		},
	}
}

// ErrQueueAlreadyExistsRes is for make our life easier when generating QueueAlreadyExists errors.
func ErrQueueAlreadyExistsRes(msg, reqID string) Response {
	return Response{
		Status: 400,
		Err: &lerr.Result{
			Result: lerr.Details{
				Type:      "Sender",
				Code:      "QueueAlreadyExists",
				Message:   msg,
				RequestID: reqID,
			},
		},
	}
}

// CreateQueue creates a new queue.
func (s SqsAPI) CreateQueue(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueName"]; !present {
		return ErrMissingParamRes("QueueName is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "CreateQueue", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrAlreadyExists:
			msg := "A queue already exists with the same name and a different value for attribute(s) " + res.ErrData.(string)
			return ErrQueueAlreadyExistsRes(msg, reqID)
		case lsqs.ErrInvalidParameterValue:
			return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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
	xmlData.CreateQueueResult.QueueURL = res.Data.(string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// DeleteMessage deletes a message with the given receipt handle on the specified queue.
func (s SqsAPI) DeleteMessage(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}
	if _, present := params["ReceiptHandle"]; !present {
		return ErrMissingParamRes("ReceiptHandle is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "DeleteMessage", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName          xml.Name `xml:"DeleteMessageResponse"`
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// DeleteQueue deletes the specified queue on this instance.
func (s SqsAPI) DeleteQueue(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "DeleteQueue", reqID, params, attributes)
	if res.Err != nil {
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// GetQueueAttributes returns the requested attributes of an specified queue.
func (s SqsAPI) GetQueueAttributes(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "GetQueueAttributes", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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
	attrs := res.Data.(map[string]string)
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
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// GetQueueUrl returns the URL of an existing Amazon SQS queue.
func (s SqsAPI) GetQueueUrl(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueName"]; !present {
		return ErrMissingParamRes("QueueName is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "GetQueueUrl", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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
	xmlData.GetQueueURLResult.QueueURL = res.Data.(string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// ListDeadLetterSourceQueues Returns a list of your queues that have the RedrivePolicy queue
// attribute configured with a dead-letter queue.
func (s SqsAPI) ListDeadLetterSourceQueues(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "ListDeadLetterSourceQueues", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName           xml.Name `xml:"ListDeadLetterSourceQueuesResponse"`
		GetQueueURLResult struct {
			QueueURL []string `xml:"QueueUrl"`
		} `xml:"ListDeadLetterSourceQueuesResult"`
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID
	xmlData.GetQueueURLResult.QueueURL = res.Data.([]string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// ListQueues return a datastructs of existing queues on this instance.
func (s SqsAPI) ListQueues(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	res := lsqs.PushReq(s.pushC, "ListQueues", reqID, params, attributes)
	if res.Err != nil {
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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
	xmlData.ListQueuesResult.QueueURL = res.Data.([]string)

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// PurgeQueue deletes the messages in a queue specified by the QueueURL parameter.
func (s SqsAPI) PurgeQueue(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "PurgeQueue", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
		}
	}

	xmlData := struct {
		XMLName          xml.Name `xml:"PurgeQueueResponse"`
		ResponseMetadata struct {
			RequestID string `xml:"RequestId"`
		}
	}{}
	xmlData.ResponseMetadata.RequestID = reqID

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// ReceiveMessage return a datastructs of messages from the specified queue.
func (s SqsAPI) ReceiveMessage(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "ReceiveMessage", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrInvalidParameterValue:
			return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	messages := res.Data.([]*lsqs.Message)
	for _, msg := range messages {
		xmlData.ReceiveMessageResult.Message = append(
			xmlData.ReceiveMessageResult.Message,
			&Message{
				MessageID:     msg.MessageID,
				ReceiptHandle: msg.ReceiptHandle,
				MD5OfBody:     msg.Md5OfMessageBody,
				Body:          string(msg.Body),
			},
		)
	}

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// SendMessage a message to the specified queue.
func (s SqsAPI) SendMessage(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	if _, present := params["MessageBody"]; !present {
		return ErrMissingParamRes("MessageBody is a required parameter", reqID)
	}
	escaped, err := url.QueryUnescape(params["MessageBody"])
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}
	params["MessageBody"] = escaped

	res := lsqs.PushReq(s.pushC, "SendMessage", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrInvalidParameterValue:
			return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
		case lsqs.ErrNonExistentQueue:
			return ErrNonExistentQueueRes(reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	m := res.Data.(map[string]string)
	xmlData.SendMessageResult.MD5OfMessageBody = m["MD5OfMessageBody"]
	xmlData.SendMessageResult.MessageID = m["MessageId"]
	xmlData.ResponseMetadata.RequestID = reqID

	buf, err := xml.Marshal(xmlData)
	if err != nil {
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}

// SetQueueAttributes sets the given attributes to the specified queue.
func (s SqsAPI) SetQueueAttributes(
	ctx context.Context,
	params map[string]string,
	attributes map[string]string,
) Response {

	reqID := ctx.Value(ReqIDKey{}).(string)

	if _, present := params["QueueUrl"]; !present {
		return ErrMissingParamRes("QueueUrl is a required parameter", reqID)
	}

	res := lsqs.PushReq(s.pushC, "SetQueueAttributes", reqID, params, attributes)
	if res.Err != nil {
		switch res.Err {
		case lsqs.ErrInvalidAttributeName:
			return ErrInvalidAttributeNameRes(res.ErrData.(string), reqID)
		case lsqs.ErrInvalidParameterValue:
			return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
		default:
			return ErrInternalErrorRes(res.Err.Error(), reqID)
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
		return ErrInternalErrorRes(err.Error(), reqID)
	}

	return SuccessRes(buf, reqID)
}
