package lsqs

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
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
	// extra data to be used by some errors
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
)

// TODO: doc
func (l *lSqs) Process(reqC chan request) {

	// TODO: add a default to return the not implemented
	for ac := range reqC {
		switch ac.action {
		case "CreateQueue":
			l.createQueue(ac)
		case "ListQueues":
			l.listQueues(ac)
		case "DeleteQueue":
			l.deleteQueue(ac)
		case "GetQueueUrl":
			l.getQueueURL(ac)
		}
	}

	log.Println("Shutting down LSQS...")
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
