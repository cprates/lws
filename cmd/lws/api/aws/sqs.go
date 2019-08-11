package aws

import (
	"context"
	"encoding/xml"
	"net/url"
	"reflect"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/lsqs"
)

// SqsAPI is the receiver for all SQS API methods.
type SqsAPI struct {
	pushC chan<- lsqs.Action
}

// SqsResult contains the result of any request to LSQS.
type SqsResult struct {
	// varies depending on the action. For CreateQueue is just a string containing
	// the newly created queue's URL, for GetQueueAttributes is a map with queue's attributes
	Data interface{}
	Err  error
	// extra data to be used by some errors like custom messages
	ErrData interface{}
}

var sqsAction = map[string]reflect.Value{}

// InstallSQS routes on the given router
func (i Interface) InstallSQS(router *mux.Router, pushC chan<- lsqs.Action) {

	log.Println("Installing SQS service")

	api := &SqsAPI{
		pushC: pushC,
	}

	lt := reflect.TypeOf(api)
	lv := reflect.ValueOf(api)

	for n := 0; n < lt.NumMethod(); n++ {
		mt := lt.Method(n)
		mv := lv.Method(n)
		sqsAction[mt.Name] = mv
	}

	// {Account} is being ignored
	router.HandleFunc("/{Account}/{QueueName}", i.commonDispatcher(sqsDispatcher))
	router.HandleFunc("/queue/{QueueName}", i.commonDispatcher(sqsDispatcher))
	router.HandleFunc("/", i.commonDispatcher(sqsDispatcher))
}

// ErrNonExistentQueueRes is for generate a result when the specified queue doesn't exist.
func ErrNonExistentQueueRes(reqID string) Response {
	return Response{
		Status: 400,
		Err: &ResponseErr{
			Details: Details{
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
		Err: &ResponseErr{
			Details: Details{
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.CreateQueue(params, attributes, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrAlreadyExists:
		msg := "A queue already exists with the same name and a different value for attribute(s) " + res.ErrData.(string)
		return ErrQueueAlreadyExistsRes(msg, reqID)
	case lsqs.ErrInvalidParameterValue:
		return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.DeleteMessage(params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.DeleteQueue(params, resC)
	res := <-resC
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.GetQueueAttributes(params, attributes, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.GetQueueURL(params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.ListDeadLetterSourceQueues(params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.ListQueues(params, resC)
	res := <-resC
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.PurgeQueue(params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.ReceiveMessage(reqID, params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	case lsqs.ErrInvalidParameterValue:
		return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.SendMessage(params, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrInvalidParameterValue:
		return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
	case lsqs.ErrNonExistentQueue:
		return ErrNonExistentQueueRes(reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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

	resC := make(chan lsqs.ReqResult)
	s.pushC <- lsqs.SetQueueAttributes(params, attributes, resC)
	res := <-resC
	switch res.Err {
	case nil:
	case lsqs.ErrInvalidAttributeName:
		return ErrInvalidAttributeNameRes(res.ErrData.(string), reqID)
	case lsqs.ErrInvalidParameterValue:
		return ErrInvalidParameterValueRes(res.ErrData.(string), reqID)
	default:
		return ErrInternalErrorRes(res.Err.Error(), reqID)
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
