package lsqs

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

type message struct {
	owner    *lSqs
	deadline time.Time

	body             []byte
	createdTimestamp time.Time
	delaySeconds     time.Duration
	messageID        string
	md5OfMessageBody string
	receiptHandle    string
	received         uint32
}

// NewMessage generates a new message with the given body and delay seconds ready to send
// to a queue.
func newMessage(owner *lSqs, body []byte, delaySeconds time.Duration) (msg *message, err error) {

	bodyMD5 := md5.New()
	_, err = io.Copy(bodyMD5, bytes.NewReader(body))
	if err != nil {
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return
	}
	mID := u.String()

	// simplified version of an receipt handler
	u, err = uuid.NewRandom()
	if err != nil {
		return
	}
	rHandle := u.String()

	msg = &message{
		owner:            owner,
		body:             body,
		createdTimestamp: time.Now().UTC(),
		delaySeconds:     delaySeconds,
		messageID:        mID,
		md5OfMessageBody: fmt.Sprintf("%x", bodyMD5.Sum(nil)),
		receiptHandle:    rHandle,
	}

	return
}
