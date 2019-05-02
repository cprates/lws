package lsqs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/params"
)

// LSqs is the interface to implement if you want to implement your own SQS service.
type LSqs interface {
	Process(chan request)
}

// lSqs represents an instance of LSqs core.
type lSqs struct {
	accountID string
	region    string
	scheme    string
	host      string
	queues    map[string]*queue
	// TODO: align
}

type reqResult struct {
	data interface{}
	err  error
	// extra data to be used by some errors like custom messages
	errData interface{}
}

type request struct {
	action string
	id     string
	// request params
	params map[string]string
	// channel to read the request result from
	resC chan *reqResult
}

var (
	// ErrAlreadyExists maps to QueueAlreadyExists
	ErrAlreadyExists = errors.New("already exists")
	// ErrNonExistentQueue maps to NonExistentQueue
	ErrNonExistentQueue = errors.New("non existent queue")
	// ErrInvalidParameterValue maps to InvalidAttributeValue
	ErrInvalidParameterValue = errors.New("invalid parameter value")
)

// TODO: doc
func (l *lSqs) Process(reqC chan request) {

	// TODO: add a default to return the not implemented
	for req := range reqC {
		switch req.action {
		case "CreateQueue":
			l.createQueue(req)
		case "ListQueues":
			l.listQueues(req)
		case "DeleteQueue":
			l.deleteQueue(req)
		case "GetQueueUrl":
			l.getQueueURL(req)
		}
	}

	log.Println("Shutting down LSQS...")
}

func validateRedrivePolicy(
	val string,
	queues map[string]*queue,
) (
	dlta string,
	mrc uint32,
	err error,
) {

	tmp := struct {
		DeadLetterTargetArn string
		MaxReceiveCount     string
	}{}

	err = json.Unmarshal([]byte(val), &tmp)
	if err != nil {
		return
	}

	mrc, err = params.ValUI32(
		"MaxReceiveCount",
		0,
		0,
		1000000,
		map[string]string{"MaxReceiveCount": tmp.MaxReceiveCount},
	)
	if err != nil {
		return
	}

	if queueByArn(tmp.DeadLetterTargetArn, queues) == nil {
		err = fmt.Errorf(
			"value %s for parameter RedrivePolicy is invalid. Reason: Dead letter target does not exist",
			val,
		)
		log.Debugln(err)
		return
	}

	// the arn will need to match an existing queue arn so, no need to check its structure.
	// On the original service it's no possible to use a dead-letter queue in a different region
	// account as the source but, this project doesn't care much about that, at least for now
	dlta = tmp.DeadLetterTargetArn
	return
}

func (l *lSqs) createQueue(req request) {

	name := req.params["QueueName"]
	url := fmt.Sprintf("%s://%s.queue.%s/%s/%s", l.scheme, l.region, l.host, l.accountID, name)
	lrn := fmt.Sprintf("arn:aws:sqs:%s:%s:%s", l.region, l.accountID, name)
	q := &queue{
		name: name,
		lrn:  lrn,
		url:  url,
	}

	log.Debugln("Creating new queue", url)

	if q, exists := l.queues[name]; exists {
		if attrNames := configDiff(q, req.params); len(attrNames) > 0 {
			log.Debugf("Queue %q already exists", name)
			attrNames := strings.Join(attrNames, ",")
			req.resC <- &reqResult{err: ErrAlreadyExists, errData: attrNames}
			return
		}
	}

	// set properties
	delaySeconds, err := params.ValUI32("DelaySeconds", 0, 0, 900, req.params)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.delaySeconds = delaySeconds

	maximumMessageSize, err := params.ValUI32("MaximumMessageSize", 262144, 1024, 262144, req.params)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.maximumMessageSize = maximumMessageSize

	messageRetentionPeriod, err := params.ValUI32(
		"MessageRetentionPeriod", 345600, 60, 1209600, req.params,
	)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.messageRetentionPeriod = messageRetentionPeriod

	receiveMessageWaitTimeSeconds, err := params.ValUI32(
		"ReceiveMessageWaitTimeSeconds", 0, 0, 20, req.params,
	)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.receiveMessageWaitTimeSeconds = receiveMessageWaitTimeSeconds

	var dlta string
	var mrc uint32
	if p := params.ValString("RedrivePolicy", req.params); p != "" {
		dlta, mrc, err = validateRedrivePolicy(p, l.queues)
		if err != nil {
			log.Debugln(err)
			req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
			return
		}

		// TODO: add the src arn to the dead-letter to link them in both ways. Not sure if this
		// is really need tbh
	}
	q.deadLetterTargetArn = dlta
	q.maxReceiveCount = mrc

	visibilityTimeout, err := params.ValUI32("VisibilityTimeout", 30, 0, 43200, req.params)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.visibilityTimeout = visibilityTimeout

	fifoQueue, err := params.ValBool("FifoQueue", false, req.params)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.fifoQueue = fifoQueue

	contentBasedDeduplication, err := params.ValBool("ContentBasedDeduplication", false, req.params)
	if err != nil {
		log.Debugln(err)
		req.resC <- &reqResult{err: ErrInvalidParameterValue, errData: err.Error()}
		return
	}
	q.contentBasedDeduplication = contentBasedDeduplication

	l.queues[name] = q

	req.resC <- &reqResult{data: url}
}

// at the time of writing this method, the AWS doc states that if deleting an non-existing
// queue, no error is returned, but testing with the aws-cli it returns an
// 'AWS.SimpleQueueService.NonExistentQueue'.
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_DeleteQueue.html
//
// Currently the queue is immediately deleted (no 60 sec limit as stated by AWS docs)
func (l *lSqs) deleteQueue(req request) {

	url := req.params["QueueUrl"]

	log.Debugln("Deleting queue", url)

	for name, q := range l.queues {
		if q.url == url {
			delete(l.queues, name)
		}
	}

	req.resC <- &reqResult{}
}

func (l *lSqs) getQueueURL(req request) {

	queueName := req.params["QueueName"]

	log.Debugln("Getting URL for queue", queueName)

	for name, q := range l.queues {
		if name == queueName {
			req.resC <- &reqResult{data: q.url}
			return
		}
	}

	req.resC <- &reqResult{err: ErrNonExistentQueue}
}

func (l *lSqs) listQueues(req request) {

	prefix := req.params["QueueNamePrefix"]

	log.Debugf("Listing queues for prefix %q", prefix)

	var urls []string
	var count int
	for _, q := range l.queues {
		// limit applied by AWS
		if count >= 1000 {
			break
		}

		if strings.HasPrefix(q.name, prefix) {
			urls = append(urls, q.url)
			count++
		}
	}

	req.resC <- &reqResult{data: urls}
}

func queueByArn(arn string, queues map[string]*queue) *queue {

	for _, q := range queues {
		if arn == q.lrn {
			return q
		}
	}

	return nil
}

// configDiff compares the current attributes with the new ones provided returning a list with
// the names of the attributes that change.
func configDiff(q *queue, newAttrs map[string]string) []string {
	// TODO
	/*
		An error occurred (QueueAlreadyExists) when calling the CreateQueue operation: A queue already
		exists with the same name and a different value for attribute MessageRetentionPeriod
	*/
	return []string{}
}
