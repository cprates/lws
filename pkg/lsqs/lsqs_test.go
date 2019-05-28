package lsqs

// create queue - no queue name
// create queue - success
// create queue - check returned url and make sure it is correct
// create queue - make sure the url, arn, name and all properties are correct
// create queue - test already exists without any difference in configs - must succeed
// create queue - test already exists difference difference in configs - must fail

// list queues - no queues - no result no error
// list queues - some queues
// list queues - prefix
// list queues - exceeding limit

// SendMessage - test escaped characters in message body

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/cprates/lws/pkg/list"
)

// Tests if a Queue is created and its properties are correctly set and within the limits.
func TestCreateQueueAndProperties(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues: map[string]*queue{
			"dummyQ": {
				lrn: "arn:aws:sqs:us-east-1:80398EXAMPLE:queue1",
			},
		},
	}

	testsSet := []struct {
		description string
		qName       string
		params      map[string]string
		attrs       map[string]string
		expectedQ   queue
		expectedErr error
	}{
		{
			"Tests all attribute's default values",
			"queue1",
			map[string]string{
				"QueueName": "queue1",
			},
			map[string]string{},
			queue{
				messages:                      list.New(),
				inflightMessages:              list.New(),
				delayedMessages:               list.New(),
				longPollQueue:                 list.New(),
				name:                          "queue1",
				lrn:                           "arn:aws:sqs:dummy-region:0000000000:queue1",
				url:                           "http://dummy-region.queue.localhost:1234/0000000000/queue1",
				delaySeconds:                  0,
				maximumMessageSize:            262144,
				messageRetentionPeriod:        345600,
				receiveMessageWaitTimeSeconds: 0,
				deadLetterTargetArn:           "",
				maxReceiveCount:               0,
				visibilityTimeout:             30,
				fifoQueue:                     false,
				contentBasedDeduplication:     false,
			},
			nil,
		},
		{
			"Tests if all queue's attributes are being correctly set",
			"queue2",
			map[string]string{
				"QueueName": "queue2",
			},
			map[string]string{
				"DelaySeconds":                  "900",
				"MaximumMessageSize":            "1024",
				"MessageRetentionPeriod":        "1209600",
				"ReceiveMessageWaitTimeSeconds": "20",
				"RedrivePolicy":                 `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:80398EXAMPLE:queue1","maxReceiveCount":"1000"}`,
				"VisibilityTimeout":             "43200",
				"FifoQueue":                     "true",
				"ContentBasedDeduplication":     "true",
			},
			queue{
				messages:                      list.New(),
				inflightMessages:              list.New(),
				delayedMessages:               list.New(),
				longPollQueue:                 list.New(),
				name:                          "queue2",
				lrn:                           "arn:aws:sqs:dummy-region:0000000000:queue2",
				url:                           "http://dummy-region.queue.localhost:1234/0000000000/queue2",
				delaySeconds:                  900,
				maximumMessageSize:            1024,
				messageRetentionPeriod:        1209600,
				receiveMessageWaitTimeSeconds: 20,
				deadLetterTargetArn:           "arn:aws:sqs:us-east-1:80398EXAMPLE:queue1",
				maxReceiveCount:               1000,
				visibilityTimeout:             43200,
				fifoQueue:                     true,
				contentBasedDeduplication:     true,
			},
			nil,
		},
		{
			"Tests DelaySeconds attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"DelaySeconds": "901",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MaximumMessageSize attribute lower limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"MaximumMessageSize": "1023",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MaximumMessageSize attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"MaximumMessageSize": "262145",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MessageRetentionPeriod attribute lower limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"MessageRetentionPeriod": "59",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MessageRetentionPeriod attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"MessageRetentionPeriod": "1209601",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests ReceiveMessageWaitTimeSeconds attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"ReceiveMessageWaitTimeSeconds": "21",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests VisibilityTimeout attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName": "dummyQ",
			},
			map[string]string{
				"VisibilityTimeout": "43201",
			},
			queue{},
			ErrInvalidParameterValue,
		},
	}

	for _, test := range testsSet {

		req := newReq("CreateQueue", "a", test.params, test.attrs)

		go func() {
			ctl.createQueue(req)
		}()
		res := <-req.resC

		if test.expectedErr != nil || res.err != nil {
			if test.expectedErr != res.err {
				t.Errorf(
					"Error mismatch. %s. Expects %q, got %q with errData: %+v",
					test.description, test.expectedErr, res.err, res.errData,
				)
			}
			continue
		}

		q, ok := ctl.queues[test.qName]
		if !ok {
			t.Error("Failed to create queue.", test.description)
			continue
		}

		if q.createdTimestamp == 0 {
			t.Errorf("CreatedTimestamp not set. %s.", test.description)
			continue
		}
		test.expectedQ.createdTimestamp = q.createdTimestamp

		if q.lastModifiedTimestamp == 0 {
			t.Errorf("LastModifiedTimestamp not set. %s.", test.description)
			continue
		}
		test.expectedQ.lastModifiedTimestamp = q.lastModifiedTimestamp

		diff := cmp.Diff(
			*q,
			test.expectedQ,
			cmp.AllowUnexported(*q, test.expectedQ),
			cmpopts.IgnoreTypes(test.expectedQ.messages),
		)
		if diff != "" {
			t.Errorf("Queue doesn't match. %s. %s", test.description, diff)
			continue
		}
	}
}

// Tests GetQueueAttributes action.
func TestGetQueueAttributes(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues:    map[string]*queue{},
	}

	req := newReq(
		"CreateQueue",
		"b",
		map[string]string{"QueueName": "deadletter"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC
	dlq := queueByName("deadletter", ctl.queues)

	req = newReq(
		"CreateQueue",
		"a",
		map[string]string{"QueueName": "queue1"},
		map[string]string{
			"RedrivePolicy": `{"deadLetterTargetArn":"` + dlq.lrn + `","maxReceiveCount":"1000"}`,
		},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC

	q := queueByName("queue1", ctl.queues)
	msg, err := newMessage(ctl, []byte("body"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	q.messages.PushBack(msg)
	// inflight
	msgInflight, err := newMessage(ctl, []byte("body"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	q.inflightMessages.PushBack(msgInflight)
	// delayed
	msgDelayed, err := newMessage(ctl, []byte("body"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	q.delayedMessages.PushBack(msgDelayed)

	testsSet := []struct {
		description   string
		qName         string
		params        map[string]string
		attrs         map[string]string
		expectedAttrs map[string]string
		expectedErr   error
	}{
		{
			"Gets All attributes",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"All": "",
			},
			map[string]string{
				"DelaySeconds":                          "0",
				"MaximumMessageSize":                    "262144",
				"MessageRetentionPeriod":                "345600",
				"ReceiveMessageWaitTimeSeconds":         "0",
				"RedrivePolicy":                         `{"DeadLetterTargetArn":"` + dlq.lrn + `","MaxReceiveCount":"1000"}`,
				"VisibilityTimeout":                     "30",
				"QueueArn":                              q.lrn,
				"CreatedTimestamp":                      strconv.FormatInt(q.createdTimestamp, 10),
				"LastModifiedTimestamp":                 strconv.FormatInt(q.createdTimestamp, 10),
				"ApproximateNumberOfMessages":           "1",
				"ApproximateNumberOfMessagesDelayed":    "1",
				"ApproximateNumberOfMessagesNotVisible": "1",
			},
			nil,
		},
		{
			"Gets attributes of an non-existing queue",
			"queue1",
			map[string]string{
				"QueueUrl": "abc",
			},
			map[string]string{
				"All": "",
			},
			nil,
			ErrNonExistentQueue,
		},
	}

	for _, test := range testsSet {
		req = newReq("GetQueueAttributes", "c", test.params, test.attrs)
		go func() {
			ctl.getQueueAttributes(req)
		}()
		res := <-req.resC

		if test.expectedErr != nil || res.err != nil {
			if test.expectedErr != res.err {
				t.Errorf(
					"Error mismatch. %s. Expects %q, got %q with errData: %+v",
					test.description, test.expectedErr, res.err, res.errData,
				)
			}
			continue
		}

		attrs := res.data.(map[string]string)
		if len(attrs) != len(test.expectedAttrs) {
			t.Errorf(
				"%s. Expects %d attributes, Got %d. %+v ** %+v",
				test.description, len(test.expectedAttrs), len(attrs), test.expectedAttrs, attrs,
			)
			continue
		}

		for k, v := range test.expectedAttrs {
			if attrs[k] != v {
				t.Errorf(
					"%s. Attribute %s does not match. Expects %q, Got %q",
					test.description, k, v, attrs[k],
				)
				continue
			}
		}
	}
}

// Tests SetQueueAttributes action.
func TestSetQueueAttributes(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues:    map[string]*queue{},
	}

	req := newReq(
		"CreateQueue",
		"b",
		map[string]string{"QueueName": "deadletter"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC
	dlq := queueByName("deadletter", ctl.queues)

	req = newReq(
		"CreateQueue",
		"a",
		map[string]string{"QueueName": "queue1"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC

	q := queueByName("queue1", ctl.queues)

	testsSet := []struct {
		description   string
		qName         string
		params        map[string]string
		attrs         map[string]string
		expectedAttrs *queue
		expectedErr   error
	}{
		{
			"Sets all supported attributes",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"DelaySeconds":                  "900",
				"MaximumMessageSize":            "262144",
				"MessageRetentionPeriod":        "1209600",
				"ReceiveMessageWaitTimeSeconds": "20",
				"RedrivePolicy":                 `{"DeadLetterTargetArn":"` + dlq.lrn + `","MaxReceiveCount":"1000"}`,
				"VisibilityTimeout":             "43200",
			},
			&queue{
				delaySeconds:                  900,
				maximumMessageSize:            262144,
				messageRetentionPeriod:        1209600,
				receiveMessageWaitTimeSeconds: 20,
				deadLetterTargetArn:           dlq.lrn,
				maxReceiveCount:               1000,
				visibilityTimeout:             43200,
			},
			nil,
		},
		{
			"Sets attributes of an non-existing queue",
			"queue1",
			map[string]string{
				"QueueUrl": "abc",
			},
			map[string]string{
				"DelaySeconds": "0",
			},
			nil,
			ErrNonExistentQueue,
		},
		{
			"Tests upper limit for DelaySeconds",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"DelaySeconds": "901",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests lower limit for MaximumMessageSize",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"MaximumMessageSize": "1023",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests upper limit for MaximumMessageSize",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"MaximumMessageSize": "262145",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests lower limit for MessageRetentionPeriod",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"MessageRetentionPeriod": "59",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests upper limit for MessageRetentionPeriod",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"MessageRetentionPeriod": "1209601",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests upper limit for ReceiveMessageWaitTimeSeconds",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"ReceiveMessageWaitTimeSeconds": "21",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests upper limit for VisibilityTimeout",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"VisibilityTimeout": "43201",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests error on invalid attribute names",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"wtf": "43201",
			},
			nil,
			ErrInvalidAttributeName,
		},
		{
			"Tests error on invalid attribute value (1)",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"VisibilityTimeout": "wtf",
			},
			nil,
			ErrInvalidParameterValue,
		},
		{
			"Tests error on invalid attribute value (2)",
			"queue1",
			map[string]string{
				"QueueUrl": q.url,
			},
			map[string]string{
				"RedrivePolicy": "wtf",
			},
			nil,
			ErrInvalidParameterValue,
		},
	}

	for _, test := range testsSet {
		req = newReq("SetQueueAttributes", "c", test.params, test.attrs)
		go func() {
			ctl.setQueueAttributes(req)
		}()
		res := <-req.resC

		if test.expectedErr != nil || res.err != nil {
			if test.expectedErr != res.err {
				t.Errorf(
					"Error mismatch. %s. Expects %q, got %q with errData: %+v",
					test.description, test.expectedErr, res.err, res.errData,
				)
			}
			continue
		}

		for k := range test.params {
			switch k {
			case "DelaySeconds":
				if q.delaySeconds != test.expectedAttrs.delaySeconds {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.delaySeconds, q.delaySeconds,
					)
					continue
				}
			case "MaximumMessageSize":
				if q.maximumMessageSize != test.expectedAttrs.maximumMessageSize {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.maximumMessageSize,
						q.maximumMessageSize,
					)
					continue
				}
			case "MessageRetentionPeriod":
				if q.messageRetentionPeriod != test.expectedAttrs.messageRetentionPeriod {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.messageRetentionPeriod,
						q.messageRetentionPeriod,
					)
					continue
				}
			case "ReceiveMessageWaitTimeSeconds":
				if q.receiveMessageWaitTimeSeconds != test.expectedAttrs.receiveMessageWaitTimeSeconds {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.receiveMessageWaitTimeSeconds,
						q.receiveMessageWaitTimeSeconds,
					)
					continue
				}
			case "RedrivePolicy":
				if q.deadLetterTargetArn != test.expectedAttrs.deadLetterTargetArn {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %q, Got %q",
						test.description, k, test.expectedAttrs.deadLetterTargetArn,
						q.deadLetterTargetArn,
					)
					continue
				}
				if q.maxReceiveCount != test.expectedAttrs.maxReceiveCount {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.maxReceiveCount,
						q.maxReceiveCount,
					)
					continue
				}
			case "VisibilityTimeout":
				if q.visibilityTimeout != test.expectedAttrs.visibilityTimeout {
					t.Errorf(
						"%s. Attribute %s does not match. Expects %d, Got %d",
						test.description, k, test.expectedAttrs.visibilityTimeout,
						q.visibilityTimeout,
					)
					continue
				}
			}
		}
	}
}

func TestInflightHandler(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues:    map[string]*queue{},
	}

	req := newReq(
		"CreateQueue",
		"a",
		map[string]string{"QueueName": "queue1"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC
	q1 := queueByName("queue1", ctl.queues)

	req = newReq(
		"CreateQueue",
		"b",
		map[string]string{"QueueName": "queue2", "MessageRetentionPeriod": "60"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC

	q2 := queueByName("queue2", ctl.queues)

	msg10, err := newMessage(ctl, []byte("body10"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	msg10.deadline = time.Now().UTC().Add(time.Second * -10)
	q1.inflightMessages.PushBack(msg10)

	msg11, err := newMessage(ctl, []byte("body11"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	msg11.deadline = time.Now().UTC().Add(time.Second * 10)
	q1.inflightMessages.PushBack(msg11)

	msg20, err := newMessage(ctl, []byte("body20"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	msg20.deadline = time.Now().UTC().Add(time.Second * -10)
	// mock retention deadline to force it to be dropped
	msg20.retentionDeadline = time.Now().UTC().Add(time.Second * -61)
	q2.inflightMessages.PushBack(msg20)

	msg21, err := newMessage(ctl, []byte("body21"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	msg21.deadline = time.Now().UTC().Add(time.Second * -10)
	q2.inflightMessages.PushBack(msg21)

	ctl.handleInflight()

	// test
	if q1.inflightMessages.Len() != 1 {
		t.Errorf("expects 1 inflight message, got %d for 'queue1'", q1.inflightMessages.Len())
	}

	if q2.inflightMessages.Len() != 0 {
		t.Errorf("expects 0 inflight messages, got %d for 'queue2'", q2.inflightMessages.Len())
	}

	if q2.messages.Len() != 1 {
		t.Errorf("expects 1 queued messages, got %d for 'queue2'", q2.messages.Len())
	}
}

func TestRetentionHandler(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues:    map[string]*queue{},
	}

	req := newReq(
		"CreateQueue",
		"a",
		map[string]string{"QueueName": "queue1", "MessageRetentionPeriod": "60"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC
	q1 := queueByName("queue1", ctl.queues)

	req = newReq(
		"CreateQueue",
		"b",
		map[string]string{"QueueName": "queue2", "MessageRetentionPeriod": "60"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC

	q2 := queueByName("queue2", ctl.queues)

	msg1, err := newMessage(ctl, []byte("body1"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	q1.messages.PushBack(msg1)

	msg20, err := newMessage(ctl, []byte("body20"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	q2.messages.PushBack(msg20)

	msg21, err := newMessage(ctl, []byte("body21"), 0, 60*time.Second)
	if err != nil {
		t.Error(err)
		return
	}
	// mock retention deadline to force it to be dropped
	msg21.retentionDeadline = time.Now().UTC().Add(time.Second * -61)
	q2.messages.PushBack(msg21)

	ctl.handleRetention()

	// test
	if q1.messages.Len() != 1 {
		t.Errorf("expects 1 queued message, got %d for 'queue1'", q1.messages.Len())
	}

	if q2.messages.Len() != 1 {
		t.Errorf("expects 1 queued message, got %d for 'queue2'", q2.messages.Len())
	}
}

// Tests the deletion of queue messages, inflight messages, non-existing messages and on a
// non-existing queue.
func TestDeleteMessage(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues:    map[string]*queue{},
	}

	req := newReq(
		"CreateQueue",
		"a",
		map[string]string{"QueueName": "queue1", "MessageRetentionPeriod": "60"},
		map[string]string{},
	)
	go func() {
		ctl.createQueue(req)
	}()
	<-req.resC
	q1 := queueByName("queue1", ctl.queues)

	createAndPushMsg := func(instance *lSqs, l *list.List, body, rHandle string) {
		msg, err := newMessage(instance, []byte(body), 0, 60*time.Second)
		if err != nil {
			t.Fatal(err)
			return
		}
		msg.receiptHandle = rHandle
		l.PushBack(msg)
	}

	createAndPushMsg(ctl, q1.messages, "body10", "receiptHandle10")
	createAndPushMsg(ctl, q1.messages, "body11", "")
	createAndPushMsg(ctl, q1.messages, "body12", "receiptHandle12")
	createAndPushMsg(ctl, q1.inflightMessages, "body20", "receiptHandle20")
	createAndPushMsg(ctl, q1.inflightMessages, "body21", "receiptHandle21")

	// Delete queued message
	req = newReq(
		"DeleteMessage",
		"a",
		map[string]string{
			"QueueUrl":      q1.url,
			"ReceiptHandle": "receiptHandle12",
		},
		map[string]string{},
	)

	go func() {
		ctl.deleteMessage(req)
	}()
	res := <-req.resC

	if res.err != nil {
		t.Errorf(res.err.Error())
	}

	// test
	if q1.messages.Len() != 2 {
		t.Errorf("expects 2 queued message, got %d", q1.messages.Len())
		return
	}

	// Delete inflight message
	req = newReq(
		"DeleteMessage",
		"a",
		map[string]string{
			"QueueUrl":      q1.url,
			"ReceiptHandle": "receiptHandle21",
		},
		map[string]string{},
	)

	go func() {
		ctl.deleteMessage(req)
	}()
	res = <-req.resC

	if res.err != nil {
		t.Errorf(res.err.Error())
		return
	}

	// test
	if q1.inflightMessages.Len() != 1 {
		t.Errorf("expects 1 queued message, got %d", q1.inflightMessages.Len())
		return
	}

	// tries to delete non-existing receipt handle
	req = newReq(
		"DeleteMessage",
		"a",
		map[string]string{
			"QueueUrl":      q1.url,
			"ReceiptHandle": "wtf",
		},
		map[string]string{},
	)

	go func() {
		ctl.deleteMessage(req)
	}()
	res = <-req.resC

	if res.err != nil {
		t.Errorf(res.err.Error())
		return
	}

	// make sure no messages were deleted
	if q1.messages.Len() != 2 {
		t.Errorf("a queued message got deleted: %d", q1.messages.Len())
		return
	}
	if q1.inflightMessages.Len() != 1 {
		t.Errorf("an inflight message got deleted: %d", q1.inflightMessages.Len())
		return
	}

	// tries to delete a message on a non-existing queue
	req = newReq(
		"DeleteMessage",
		"a",
		map[string]string{
			"QueueUrl":      "wtfqueue",
			"ReceiptHandle": "wtf",
		},
		map[string]string{},
	)

	go func() {
		ctl.deleteMessage(req)
	}()
	res = <-req.resC

	if res.err != ErrNonExistentQueue {
		t.Errorf(
			"Error mismatch. Expects %q, got %q",
			ErrNonExistentQueue, res.err,
		)
	}
}
