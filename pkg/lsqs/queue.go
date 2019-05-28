package lsqs

import (
	"time"

	"github.com/google/uuid"

	"github.com/cprates/lws/pkg/list"
)

type queue struct {
	name                  string
	lrn                   string // arn
	url                   string
	createdTimestamp      int64
	lastModifiedTimestamp int64
	messages              *list.List
	inflightMessages      *list.List // Messages stored here are sorted by deadline
	delayedMessages       *list.List // Messages stored here are sorted by deadline
	longPollQueue         *list.List

	// Configurable
	delaySeconds                  uint32
	maximumMessageSize            uint32
	messageRetentionPeriod        uint32
	receiveMessageWaitTimeSeconds uint32
	visibilityTimeout             uint32
	// Redrive policy
	deadLetterTargetArn string
	maxReceiveCount     uint32
	// FIFO
	fifoQueue                 bool
	contentBasedDeduplication bool
}

type longPollRequest struct {
	originalReq         *request
	deadline            time.Time
	maxNumberOfMessages int
}

func (q *queue) deleteMessage(receiptHandle string) bool {

	for e := q.inflightMessages.Front(); e != nil; e = e.Next() {
		if e.Value.(*message).receiptHandle == receiptHandle {
			q.inflightMessages.Remove(e)
			return true
		}
	}

	for e := q.messages.Front(); e != nil; e = e.Next() {
		msg := e.Value.(*message)
		if msg.receiptHandle != "" && msg.receiptHandle == receiptHandle {
			q.messages.Remove(e)
			return true
		}
	}

	return false
}

func (q *queue) takeUpTo(n int) (msgs []*message) {

	for i := 0; i < n; i++ {
		elem := q.messages.PullFront()
		if elem == nil {
			break
		}

		msgs = append(msgs, elem.(*message))
	}

	return
}

func (q *queue) setInflight(messages []*message) {

	if len(messages) == 0 {
		return
	}

	elements := list.New()
	for _, msg := range messages {
		msg.received++
		// simplified version of an receipt handler
		u, err := uuid.NewRandom()
		if err != nil {
			return
		}
		msg.receiptHandle = u.String()
		msg.deadline = time.Now().UTC().Add(time.Duration(q.visibilityTimeout) * time.Second)
		elements.PushBack(msg)
	}

	q.inflightMessages.PushFrontList(elements)
	q.inflightMessages.Sort(deadlineCmp)
}

func (q *queue) setDelayed(msg *message, delaySeconds time.Duration) {

	msg.deadline = time.Now().UTC().Add(delaySeconds)
	q.delayedMessages.PushFront(msg)
	q.delayedMessages.Sort(deadlineCmp)
}

func (q *queue) setOnWait(originalReq *request, uptoMessages int, waitSeconds time.Duration) {

	req := &longPollRequest{
		originalReq:         originalReq,
		deadline:            time.Now().UTC().Add(waitSeconds),
		maxNumberOfMessages: uptoMessages,
	}

	q.longPollQueue.PushBack(req)
}
