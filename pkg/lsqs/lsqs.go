package lsqs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/list"
	"github.com/cprates/lws/pkg/params"
)

// LSqs is the interface to implement if you want to implement your own SQS service.
type LSqs interface {
	Process(<-chan Action, <-chan struct{})
}

// Action to be executed on the given LSQS instance. The instance type is not exported
// so the user can not call these actions directly, making the pushC returned on Start
// the only way to communicate with an instance.
type Action func(l *lSqs)

// lSqs represents an instance of LSqs core.
type lSqs struct {
	accountID string
	region    string
	scheme    string
	host      string
	queues    map[string]*queue // TODO: may be the key should be the queue's URL
	// TODO: align
}

// ReqResult contains the result of any request to LSQS.
type ReqResult struct {
	Data interface{}
	Err  error
	// extra data to be used by some errors like custom messages
	ErrData interface{}
}

var (
	// ErrAlreadyExists maps to QueueAlreadyExists
	ErrAlreadyExists = errors.New("already exists")
	// ErrNonExistentQueue maps to NonExistentQueue
	ErrNonExistentQueue = errors.New("non existent queue")
	// ErrInvalidParameterValue maps to InvalidAttributeValue
	ErrInvalidParameterValue = errors.New("invalid parameter value")
	// ErrInvalidAttributeName maps to InvalidAttributeName
	ErrInvalidAttributeName = errors.New("invalid attribute name")
)

// FmtURL defines the format of a Queue URL
const FmtURL = "%s://%s/%s/%s"

// Start a new LSQS instance with the given configuration and returns a channel to
// communicate with it.
func Start(accountID, region, scheme, host string, stopC <-chan struct{}) chan<- Action {

	pushC := make(chan Action)
	instance := &lSqs{
		accountID: accountID,
		region:    region,
		scheme:    scheme,
		host:      host,
		queues:    map[string]*queue{},
	}

	go instance.Process(pushC, stopC)

	return pushC
}

// Process holds the main loop of an LSQS instance, receive actions sent by users, move
// messages around (inflight, dead letters, delayed messages) and controls long polling.
func (l *lSqs) Process(reqC <-chan Action, stopC <-chan struct{}) {

	inflightTick := time.NewTicker(250 * time.Millisecond)
	retentionTick := time.NewTicker(500 * time.Millisecond)
	delayTick := time.NewTicker(250 * time.Millisecond)
	longPollTick := time.NewTicker(500 * time.Millisecond)

	running := true
	for running {
		select {
		case action := <-reqC:
			action(l)
		case <-inflightTick.C:
			l.handleInflight()
		case <-retentionTick.C:
			l.handleRetention()
		case <-delayTick.C:
			l.handleDelayed()
		case <-longPollTick.C:
			l.handleLongPoll()
		case <-stopC:
			running = false
		}
	}

	log.Println("Shutting down LSQS...")
	inflightTick.Stop()
	retentionTick.Stop()
	delayTick.Stop()
	longPollTick.Stop()
}

func (l *lSqs) handleInflight() {

	now := time.Now().UTC()
	for _, queue := range l.queues {
		var toRemove []*list.Element
		for elem := queue.inflightMessages.Front(); elem != nil; elem = elem.Next() {
			msg := elem.Value.(*Message)
			if msg.deadline.After(now) {
				// the list is sorted by deadline so, move to the next queue
				break
			}

			toRemove = append(toRemove, elem)

			// if retention deadline has been exceeded while inflight, drop the message
			if msg.retentionDeadline.Before(now) {
				log.Debugln(
					"Message", msg.MessageID,
					"has exceed retention period after inflight period. Dropping...",
				)
				continue
			}

			// if a dead-letter queue is configured, check if it needs to be redrived
			if queue.deadLetterTargetArn != "" && msg.Received >= queue.maxReceiveCount {
				// aws allow to delete a dead-letter queue without removing it from source
				// queues so, we need to check if it exists
				deadLetterQ := queueByArn(queue.deadLetterTargetArn, l.queues)
				if deadLetterQ != nil {
					// move it right away unlike aws does, which only redrive the message when the user
					// tries to receive a message again
					msg.ReceiptHandle = ""
					deadLetterQ.messages.PushBack(msg)
					log.Debugln(
						"Message", msg.MessageID,
						"moved to dead-letter queue", deadLetterQ.url,
					)
					continue
				}
				// if the target dead-letter doesn't exist, requeue the message
				log.Warnln(
					"Dead-letter configured on queue", queue.url,
					"but doesn't exists:", queue.deadLetterTargetArn,
				)
			}

			log.Debugln("Message", msg.MessageID, "moved from inflight to main queue")
			queue.messages.PushBack(msg)
		}

		// remove expired ones
		for _, elem := range toRemove {
			queue.inflightMessages.Remove(elem)
		}
	}
}

func (l *lSqs) handleRetention() {

	now := time.Now().UTC()
	for _, queue := range l.queues {
		var toRemove []*list.Element
		for node := queue.messages.Front(); node != nil; node = node.Next() {
			msg := node.Value.(*Message)
			if msg.retentionDeadline.Before(now) {
				toRemove = append(toRemove, node)
				log.Debugln("Message", msg.MessageID, "expired retention period")
			}
		}

		for _, node := range toRemove {
			queue.messages.Remove(node)
		}
	}
}

func (l *lSqs) handleDelayed() {

	now := time.Now().UTC()
	for _, queue := range l.queues {
		var toMove []*list.Element
		for elem := queue.delayedMessages.Front(); elem != nil; elem = elem.Next() {
			msg := elem.Value.(*Message)
			if msg.deadline.After(now) {
				// the list is sorted by deadline so, move to the next queue
				break
			}

			toMove = append(toMove, elem)

			log.Debugln("Message", msg.MessageID, "moved from delayed to main queue")
			queue.messages.PushBack(msg)
		}

		// remove moved ones
		for _, elem := range toMove {
			queue.delayedMessages.Remove(elem)
		}
	}
}

func (l *lSqs) handleLongPoll() {

	now := time.Now().UTC()
	for _, queue := range l.queues {
		var servedOrExpired []*list.Element
		for elem := queue.longPollQueue.Front(); elem != nil; elem = elem.Next() {
			req := elem.Value.(*longPollRequest)
			if req.deadline.Before(now) {
				log.Debugln(
					"LongPoll waiter with request", req.reqID,
					"has expired on queue", queue.url,
				)
				req.resC <- ReqResult{Data: []*Message{}}
				servedOrExpired = append(servedOrExpired, elem)
				continue
			}

			// TODO: This is ok because for now there is only one instance running.
			//  After implementing the support for multiple instances, the sync between
			//  instances must be assured
			msgs := queue.takeUpTo(req.maxNumberOfMessages)
			if len(msgs) == 0 {
				// no more messages to read but still need to check for expired deadlines
				// Doing this means that one waiter may not get any messages but the next one does
				continue
			}

			log.Debugln("Serving LongPoll request", req.reqID, "on queue", queue.url)

			queue.setInflight(msgs)
			req.resC <- ReqResult{Data: msgs}
			servedOrExpired = append(servedOrExpired, elem)
		}

		// remove served or expired waiters
		for _, elem := range servedOrExpired {
			queue.longPollQueue.Remove(elem)
		}
	}
}

func validateRedrivePolicy(
	val string,
	queues map[string]*queue,
) (
	deadLetterTarget *queue,
	maxReceiveCount uint32,
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

	maxReceiveCount, err = params.ValUI32(
		"MaxReceiveCount",
		0,
		1,
		1000,
		map[string]string{"MaxReceiveCount": tmp.MaxReceiveCount},
	)
	if err != nil {
		return
	}

	var targetQueue *queue
	if targetQueue = queueByArn(tmp.DeadLetterTargetArn, queues); targetQueue == nil {
		err = fmt.Errorf(
			"value %s for parameter RedrivePolicy is invalid. Reason: Dead letter target does not exist",
			val,
		)
		log.Debugln(err)
		return
	}

	// On the original service it's not possible to use a dead-letter queue in a different region
	// account as the source but, this project doesn't care much about that, at least for now
	deadLetterTarget = targetQueue
	return
}

// CreateQueue creates a new standard or FIFO queue. You can pass one or more attributes in the
// request. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_CreateQueue.html
func CreateQueue(
	parameters map[string]string,
	attributes map[string]string,
	resC chan<- ReqResult,
) Action {

	return func(l *lSqs) {
		name := parameters["QueueName"]
		url := fmt.Sprintf(FmtURL, l.scheme, l.host, l.accountID, name)
		lrn := fmt.Sprintf("arn:aws:sqs:%s:%s:%s", l.region, l.accountID, name)
		ts := time.Now().Unix()
		newQ := &queue{
			name:                  name,
			lrn:                   lrn,
			url:                   url,
			createdTimestamp:      ts,
			lastModifiedTimestamp: ts,
			messages:              list.New(),
			inflightMessages:      list.New(),
			delayedMessages:       list.New(),
			longPollQueue:         list.New(),
			sourceQueues:          map[string]*queue{},
		}

		log.Debugln("Creating new queue", url)

		if q, exists := l.queues[name]; exists {
			if attrNames := configDiff(q, parameters); len(attrNames) > 0 {
				log.Debugf("Queue %q already exists", name)
				attrNames := strings.Join(attrNames, ",")
				resC <- ReqResult{Err: ErrAlreadyExists, ErrData: attrNames}
				return
			}
		}

		// set properties
		delaySeconds, err := params.ValUI32("DelaySeconds", 0, 0, 900, attributes)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.delaySeconds = delaySeconds

		maximumMessageSize, err := params.ValUI32(
			"MaximumMessageSize",
			262144,
			1024,
			262144,
			attributes,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.maximumMessageSize = maximumMessageSize

		messageRetentionPeriod, err := params.ValUI32(
			"MessageRetentionPeriod", 345600, 60, 1209600, attributes,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.messageRetentionPeriod = messageRetentionPeriod

		receiveMessageWaitTimeSeconds, err := params.ValUI32(
			"ReceiveMessageWaitTimeSeconds", 0, 0, 20, attributes,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.receiveMessageWaitTimeSeconds = receiveMessageWaitTimeSeconds

		if p := params.ValString("RedrivePolicy", attributes); p != "" {
			var maxReceiveCount uint32
			var deadLetterTarget *queue
			deadLetterTarget, maxReceiveCount, err = validateRedrivePolicy(p, l.queues)
			if err != nil {
				log.Debugln(err)
				resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
				return
			}

			deadLetterTarget.sourceQueues[newQ.url] = newQ
			newQ.deadLetterTargetArn = deadLetterTarget.lrn
			newQ.maxReceiveCount = maxReceiveCount
		}

		visibilityTimeout, err := params.ValUI32("VisibilityTimeout", 30, 0, 43200, attributes)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.visibilityTimeout = visibilityTimeout

		fifoQueue, err := params.ValBool("FifoQueue", false, attributes)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.fifoQueue = fifoQueue

		contentBasedDeduplication, err := params.ValBool(
			"ContentBasedDeduplication",
			false,
			attributes,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}
		newQ.contentBasedDeduplication = contentBasedDeduplication

		// loo for previous source queues (dead-letter related)
		for _, q := range l.queues {
			if q.deadLetterTargetArn == newQ.lrn {
				log.Debugln("Re-adding source queue", q.url)
				newQ.sourceQueues[q.url] = q
			}
		}

		l.queues[name] = newQ

		resC <- ReqResult{Data: url}
	}
}

// DeleteMessage deletes the specified message from the specified queue. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_DeleteMessage.html
func DeleteMessage(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		url := parameters["QueueUrl"]
		receiptHandle := parameters["ReceiptHandle"]

		log.Debugln(
			"Deleting message with receipt handle", receiptHandle,
			"on queue", url,
		)

		var q *queue
		if q = queueByURL(url, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		if deleted := q.deleteMessage(receiptHandle); deleted {
			log.Debugln(
				"Message deleted:", receiptHandle,
				"on queue", url,
			)
		} else {
			log.Debugln(
				"Message not found:", receiptHandle,
				"on queue", url,
			)
		}
		resC <- ReqResult{}
	}
}

// DeleteQueue deletes the queue specified by the QueueUrl, regardless of the queue's contents.
// If the specified queue doesn't exist, Amazon SQS returns a successful response. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_DeleteQueue.html
//
// At the time of writing this method, the AWS doc states that if deleting an non-existing
// queue, no error is returned, but testing with the aws-cli it returns an
// 'AWS.SimpleQueueService.NonExistentQueue'.
//
// Currently the queue is immediately deleted (no 60 sec limit as stated by AWS docs)
func DeleteQueue(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		url := parameters["QueueUrl"]

		log.Debugln("Deleting queue", url)

		defer func() {
			log.Debugln("Queue deleted:", url)
			resC <- ReqResult{}
		}()

		var q *queue
		if q = queueByURL(url, l.queues); q == nil {
			return
		}

		// update the dead-letter queue
		if q.deadLetterTargetArn != "" {
			deadLetterQ := queueByArn(q.deadLetterTargetArn, l.queues)
			delete(deadLetterQ.sourceQueues, q.url)
			log.Debugln(
				"Source queue", url,
				"deleted from", deadLetterQ.url,
			)
		}

		delete(l.queues, q.name)
	}
}

// GetQueueAttributes gets attributes for the specified queue. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html
func GetQueueAttributes(
	parameters map[string]string,
	attributes map[string]string,
	resC chan<- ReqResult,
) Action {

	return func(l *lSqs) {
		queueURL := parameters["QueueUrl"]

		log.Debugln("Getting Attributes for queue", queueURL)

		var q *queue
		if q = queueByURL(queueURL, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		var toGet []string
		if _, all := attributes["All"]; all {
			toGet = []string{
				"ApproximateNumberOfMessages", "ApproximateNumberOfMessagesDelayed",
				"ApproximateNumberOfMessagesNotVisible", "CreatedTimestamp", "DelaySeconds",
				"LastModifiedTimestamp", "MaximumMessageSize", "MessageRetentionPeriod", "QueueArn",
				"ReceiveMessageWaitTimeSeconds", "RedrivePolicy", "VisibilityTimeout", "FifoQueue",
				"ContentBasedDeduplication",
			}
		} else {
			for k := range attributes {
				toGet = append(toGet, k)
			}
		}

		qAttrs := map[string]string{}
		for _, attr := range toGet {
			switch attr {
			case "ApproximateNumberOfMessages":
				qAttrs[attr] = strconv.FormatUint(uint64(q.messages.Len()), 10)
			case "ApproximateNumberOfMessagesDelayed":
				qAttrs[attr] = strconv.FormatUint(uint64(q.delayedMessages.Len()), 10)
			case "ApproximateNumberOfMessagesNotVisible":
				qAttrs[attr] = strconv.FormatUint(uint64(q.inflightMessages.Len()), 10)
			case "CreatedTimestamp":
				qAttrs[attr] = strconv.FormatInt(q.createdTimestamp, 10)
			case "DelaySeconds":
				qAttrs[attr] = strconv.FormatUint(uint64(q.delaySeconds), 10)
			case "LastModifiedTimestamp":
				qAttrs[attr] = strconv.FormatInt(q.lastModifiedTimestamp, 10)
			case "MaximumMessageSize":
				qAttrs[attr] = strconv.FormatUint(uint64(q.maximumMessageSize), 10)
			case "MessageRetentionPeriod":
				qAttrs[attr] = strconv.FormatUint(uint64(q.messageRetentionPeriod), 10)
			case "QueueArn":
				qAttrs[attr] = q.lrn
			case "ReceiveMessageWaitTimeSeconds":
				qAttrs[attr] = strconv.FormatUint(uint64(q.receiveMessageWaitTimeSeconds), 10)
			case "RedrivePolicy":
				if q.deadLetterTargetArn != "" {
					rp := map[string]string{
						"DeadLetterTargetArn": q.deadLetterTargetArn,
						"MaxReceiveCount":     strconv.FormatUint(uint64(q.maxReceiveCount), 10),
					}
					b, _ := json.Marshal(rp)
					qAttrs[attr] = string(b)
				}
			case "VisibilityTimeout":
				qAttrs[attr] = strconv.FormatUint(uint64(q.visibilityTimeout), 10)
			case "FifoQueue":
				// TODO
			case "ContentBasedDeduplication":
				// TODO
			}
		}

		resC <- ReqResult{Data: qAttrs}
	}
}

// GetQueueURL returns the URL of an existing Amazon SQS queue. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueUrl.html
func GetQueueURL(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		queueName := parameters["QueueName"]

		log.Debugln("Getting URL for queue", queueName)

		if q := queueByName(queueName, l.queues); q != nil {
			resC <- ReqResult{Data: q.url}
			return
		}

		resC <- ReqResult{Err: ErrNonExistentQueue}
	}
}

// ListDeadLetterSourceQueues returns a list of your queues that have the RedrivePolicy queue
// attribute configured with a dead-letter queue. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ListDeadLetterSourceQueues.html
func ListDeadLetterSourceQueues(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		queueURL := parameters["QueueUrl"]

		log.Debugln("Getting list of Dead Letter source queues for queue", queueURL)

		if q := queueByURL(queueURL, l.queues); q != nil {
			var sources []string
			for k := range q.sourceQueues {
				sources = append(sources, k)
			}
			resC <- ReqResult{Data: sources}
			return
		}

		resC <- ReqResult{Err: ErrNonExistentQueue}
	}
}

// ListQueues returns a list of your queues. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ListQueues.html
func ListQueues(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		prefix := parameters["QueueNamePrefix"]

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

		resC <- ReqResult{Data: urls}
	}
}

// PurgeQueue deletes the messages in a queue specified by the QueueURL parameter. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_PurgeQueue.html
func PurgeQueue(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		url := parameters["QueueUrl"]

		log.Debugln("Purging queue", url)

		var q *queue
		if q = queueByURL(url, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		q.purgeQueue()
		resC <- ReqResult{}
	}
}

// ReceiveMessage retrieves one or more messages (up to 10), from the specified queue. Using the
// WaitTimeSeconds parameter enables long-poll support. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html
func ReceiveMessage(reqID string, parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		queueURL := parameters["QueueUrl"]

		log.Debugln("Receiving messages from queue", queueURL)

		var q *queue
		if q = queueByURL(queueURL, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		waitTimeSeconds, err := params.ValUI32(
			"WaitTimeSeconds", q.receiveMessageWaitTimeSeconds, 0, 20, parameters,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}

		maxNumberOfMessages, err := params.ValUI32("MaxNumberOfMessages", 1, 1, 10, parameters)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}

		messages := q.takeUpTo(int(maxNumberOfMessages))

		if len(messages) > 0 || waitTimeSeconds == 0 {
			// move messages to inflight queue
			q.setInflight(messages)
			resC <- ReqResult{Data: messages}
			return
		}

		q.setOnWait(
			reqID,
			parameters,
			resC,
			int(maxNumberOfMessages),
			time.Duration(waitTimeSeconds)*time.Second,
		)
	}
}

// SendMessage delivers a message to the specified queue. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SendMessage.html
func SendMessage(parameters map[string]string, resC chan<- ReqResult) Action {

	return func(l *lSqs) {
		queueURL := parameters["QueueUrl"]

		log.Debugln("Sending message to queue", queueURL)

		var q *queue
		if q = queueByURL(queueURL, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		body := []byte(parameters["MessageBody"])
		if uint32(len(body)) > q.maximumMessageSize {
			err := fmt.Sprintf(
				"One or more parameters are invalid. Reason: Message must be shorter than %d bytes.",
				q.maximumMessageSize,
			)
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err}
			return
		}
		// TODO: check allowed chars

		delaySeconds, err := params.ValUI32("DelaySeconds", q.delaySeconds, 0, 900, parameters)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
			return
		}

		msg, err := newMessage(
			l,
			body,
			time.Duration(delaySeconds)*time.Second,
			time.Duration(q.messageRetentionPeriod)*time.Second,
		)
		if err != nil {
			log.Debugln(err)
			resC <- ReqResult{Err: err}
			return
		}

		if delaySeconds == 0 {
			q.messages.PushBack(msg)
		} else {
			q.setDelayed(msg, time.Duration(delaySeconds)*time.Second)
		}

		log.Debugln(
			"Sent message", msg.MessageID,
			"to queue", q.url,
		)

		resC <- ReqResult{
			Data: map[string]string{
				//"MD5OfMessageAttributes": "", // TODO: attributes not supported yet
				"MD5OfMessageBody": msg.Md5OfMessageBody,
				"MessageId":        msg.MessageID,
				//"SequenceNumber":         "", // TODO: FIFO not supported yet
			},
		}
	}
}

// SetQueueAttributes sets the value of one or more queue attributes. Unlike SQS, the changes are
// immediate. More info at:
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SetQueueAttributes.html
func SetQueueAttributes(
	parameters map[string]string,
	attributes map[string]string,
	resC chan<- ReqResult,
) Action {

	return func(l *lSqs) {
		queueURL := parameters["QueueUrl"]

		log.Debugln("Setting Attributes for queue", queueURL)

		var q *queue
		if q = queueByURL(queueURL, l.queues); q == nil {
			resC <- ReqResult{Err: ErrNonExistentQueue}
			return
		}

		var err error
		for attr, val := range attributes {
			switch attr {
			case "DelaySeconds":
				delaySeconds, e := params.UI32(val, 0, 900)
				if e != nil {
					err = fmt.Errorf("%s %s", e, attr)
					break
				}
				q.delaySeconds = delaySeconds
			case "MaximumMessageSize":
				maximumMessageSize, e := params.UI32(val, 1024, 262144)
				if e != nil {
					err = fmt.Errorf("%s %s", e, attr)
					break
				}
				q.maximumMessageSize = maximumMessageSize
			case "MessageRetentionPeriod":
				messageRetentionPeriod, e := params.UI32(val, 60, 1209600)
				if e != nil {
					err = fmt.Errorf("%s %s", e, attr)
					break
				}
				q.messageRetentionPeriod = messageRetentionPeriod
			case "Policy":
			case "ReceiveMessageWaitTimeSeconds":
				receiveMessageWaitTimeSeconds, e := params.UI32(val, 0, 20)
				if e != nil {
					err = fmt.Errorf("%s %s", e, attr)
					break
				}
				q.receiveMessageWaitTimeSeconds = receiveMessageWaitTimeSeconds
			case "RedrivePolicy":
				deadLetterTarget, maxReceiveCount, e := validateRedrivePolicy(val, l.queues)
				if e != nil {
					err = e
					break
				}
				deadLetterTarget.sourceQueues[q.url] = q
				q.deadLetterTargetArn = deadLetterTarget.lrn
				q.maxReceiveCount = maxReceiveCount
			case "VisibilityTimeout":
				visibilityTimeout, e := params.UI32(val, 0, 43200)
				if e != nil {
					err = fmt.Errorf("%s %s", e, attr)
					break
				}
				q.visibilityTimeout = visibilityTimeout
			case "KmsMasterKeyId":
			case "KmsDataKeyReusePeriodSeconds":
			case "ContentBasedDeduplication":
			// TODO
			default:
				log.Debugln("Unknown attribute", attr)
				resC <- ReqResult{
					Err:     ErrInvalidAttributeName,
					ErrData: "Unknown Attribute " + attr,
				}
				return
			}

			if err != nil {
				log.Debugln(err)
				resC <- ReqResult{Err: ErrInvalidParameterValue, ErrData: err.Error()}
				return
			}
		}

		resC <- ReqResult{}
	}
}

func queueByArn(arn string, queues map[string]*queue) *queue {

	for _, q := range queues {
		if arn == q.lrn {
			return q
		}
	}

	return nil
}

func queueByName(name string, queues map[string]*queue) *queue {

	for _, q := range queues {
		if name == q.name {
			return q
		}
	}

	return nil
}

func queueByURL(url string, queues map[string]*queue) *queue {

	for _, q := range queues {
		if url == q.url {
			return q
		}
	}

	return nil
}

// configDiff compares the current attributes with the new ones provided returning a datastructs
// with the names of the attributes that change.
func configDiff(q *queue, newAttrs map[string]string) []string {
	// TODO
	/*
		An error occurred (QueueAlreadyExists) when calling the CreateQueue operation: A queue
		already exists with the same name and a different value for attribute MessageRetentionPeriod
	*/
	return []string{}
}
