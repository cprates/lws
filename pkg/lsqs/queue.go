package lsqs

type queue struct {
	name string
	lrn  string // arn
	url  string

	// Common
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
