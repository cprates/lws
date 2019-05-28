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
	owner             *lSqs
	deadline          time.Time
	retentionDeadline time.Time

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
func newMessage(
	owner *lSqs,
	body []byte,
	delaySeconds time.Duration,
	retentionSeconds time.Duration,
) (msg *message, err error) {

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

	creationTs := time.Now().UTC()
	msg = &message{
		owner:             owner,
		body:              body,
		retentionDeadline: creationTs.Add(retentionSeconds),
		createdTimestamp:  creationTs,
		delaySeconds:      delaySeconds,
		messageID:         mID,
		md5OfMessageBody:  fmt.Sprintf("%x", bodyMD5.Sum(nil)),
	}

	return
}

func deadlineCmp(i, j interface{}) int {
	return i.(*message).deadline.Second() - j.(*message).deadline.Second()
}
