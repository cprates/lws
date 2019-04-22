package lsqs

import (
	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
)

// ErrNonExistentQueueRes is for generate a result when the specified queue doesn't exist.
func ErrNonExistentQueueRes(reqID string) common.Result {
	return common.Result{
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
func ErrQueueAlreadyExistsRes(msg, reqID string) common.Result {
	return common.Result{
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
