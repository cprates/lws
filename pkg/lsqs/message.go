package lsqs

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

// Message represents a Queue's message.
type Message struct {
	owner             *lSqs
	deadline          time.Time
	retentionDeadline time.Time

	Body             []byte
	CreatedTimestamp time.Time
	DelaySeconds     time.Duration
	MessageID        string
	Md5OfMessageBody string
	ReceiptHandle    string
	Received         uint32
}

// NewMessage generates a new message with the given body and delay seconds ready to send
// to a queue.
func newMessage(
	owner *lSqs,
	body []byte,
	delaySeconds time.Duration,
	retentionSeconds time.Duration,
) (msg *Message, err error) {

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
	msg = &Message{
		owner:             owner,
		Body:              body,
		retentionDeadline: creationTs.Add(retentionSeconds),
		CreatedTimestamp:  creationTs,
		DelaySeconds:      delaySeconds,
		MessageID:         mID,
		Md5OfMessageBody:  fmt.Sprintf("%x", bodyMD5.Sum(nil)),
	}

	return
}

func deadlineCmp(i, j interface{}) int {
	return i.(*Message).deadline.Second() - j.(*Message).deadline.Second()
}
