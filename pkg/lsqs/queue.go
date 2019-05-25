package lsqs

import (
	"sort"
	"time"

	"github.com/cprates/lws/pkg/datastructs"
)

type inflightArray []*message

type queue struct {
	name                  string
	lrn                   string // arn
	url                   string
	createdTimestamp      int64
	lastModifiedTimestamp int64
	messages              *datastructs.SList
	inflightMessages      *datastructs.SList // Messages stored here are sorted by their deadline

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

func (q *queue) takeUpTo(n int) (msgs []*message) {

	nodes := q.messages.TakeUpToN(n)
	for _, n := range nodes {
		msgs = append(msgs, n.Data.(*message))
	}

	return
}

func (q *queue) setInflight(messages inflightArray) {

	if len(messages) == 0 {
		return
	}

	sort.Sort(messages)

	var nodes []*datastructs.Node
	for _, msg := range messages {
		msg.received++
		msg.deadline = time.Now().UTC().Add(time.Duration(q.visibilityTimeout) * time.Second)
		nodes = append(nodes, &datastructs.Node{Data: msg})
	}

	q.inflightMessages.AppendN(nodes)
}

func (ia inflightArray) Len() int           { return len(ia) }
func (ia inflightArray) Swap(i, j int)      { ia[i], ia[j] = ia[j], ia[i] }
func (ia inflightArray) Less(i, j int) bool { return ia[i].deadline.Before(ia[j].deadline) }
