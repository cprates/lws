package lsqs

import (
	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/lerr"
)

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
