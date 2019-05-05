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

import "testing"

// Tests if a Queue is created and its properties are correctly set and within the limits.
func TestCreateQueueAndProperties(t *testing.T) {

	ctl := &lSqs{
		accountID: "0000000000",
		region:    "dummy-region",
		scheme:    "http",
		host:      "localhost:1234",
		queues: map[string]*queue{
			"dummyQ": {
				lrn: "arn:aws:sqs:us-east-1:80398EXAMPLE:MyDeadLetterQueue",
			},
		},
	}

	testsSet := []struct {
		description string
		qName       string
		params      map[string]string
		expectedQ   queue
		expectedErr error
	}{
		{
			"Tests all attribute's default values",
			"queue1",
			map[string]string{
				"QueueName": "queue1",
			},
			queue{
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
				"QueueName":                     "queue2",
				"DelaySeconds":                  "900",
				"MaximumMessageSize":            "1024",
				"MessageRetentionPeriod":        "1209600",
				"ReceiveMessageWaitTimeSeconds": "20",
				"RedrivePolicy":                 `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:80398EXAMPLE:MyDeadLetterQueue","maxReceiveCount":"1000"}`,
				"VisibilityTimeout":             "43200",
				"FifoQueue":                     "true",
				"ContentBasedDeduplication":     "true",
			},
			queue{
				name:                          "queue2",
				lrn:                           "arn:aws:sqs:dummy-region:0000000000:queue2",
				url:                           "http://dummy-region.queue.localhost:1234/0000000000/queue2",
				delaySeconds:                  900,
				maximumMessageSize:            1024,
				messageRetentionPeriod:        1209600,
				receiveMessageWaitTimeSeconds: 20,
				deadLetterTargetArn:           "arn:aws:sqs:us-east-1:80398EXAMPLE:MyDeadLetterQueue",
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
				"QueueName":    "dummyQ",
				"DelaySeconds": "901",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MaximumMessageSize attribute lower limit",
			"dummyQ",
			map[string]string{
				"QueueName":          "dummyQ",
				"MaximumMessageSize": "1023",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MaximumMessageSize attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName":          "dummyQ",
				"MaximumMessageSize": "262145",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MessageRetentionPeriod attribute lower limit",
			"dummyQ",
			map[string]string{
				"QueueName":              "dummyQ",
				"MessageRetentionPeriod": "59",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests MessageRetentionPeriod attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName":              "dummyQ",
				"MessageRetentionPeriod": "1209601",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests ReceiveMessageWaitTimeSeconds attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName":                     "dummyQ",
				"ReceiveMessageWaitTimeSeconds": "21",
			},
			queue{},
			ErrInvalidParameterValue,
		},
		{
			"Tests VisibilityTimeout attribute upper limit",
			"dummyQ",
			map[string]string{
				"QueueName":         "dummyQ",
				"VisibilityTimeout": "43201",
			},
			queue{},
			ErrInvalidParameterValue,
		},
	}

	for _, test := range testsSet {

		req := newReq("CreateQueue", "a", test.params)

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

		if *q != test.expectedQ {
			t.Errorf(
				"Queue doesn't match. %s. Got %+v, expects %+v",
				test.description, *q, test.expectedQ,
			)
			continue
		}
	}

}
