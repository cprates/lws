package lsns

import (
	log "github.com/sirupsen/logrus"
)

// LSns is the interface to implement if you want to implement your own SNS service.
type LSns interface {
	Process(chan request)
}

// lSns represents an instance of LSns core.
type lSns struct {
	accountID string
	region    string
	scheme    string
	host      string
	queues    []*topic
	// TODO: align
}

type request struct {
	action string
	reqID  string
	// input data
	data map[string]string
	// channel to read the instruction result from
	resC chan interface{}
}

// TODO: doc
func (l *lSns) Process(reqC chan request) {

	for ac := range reqC {
		switch ac.action {
		case "CreateTopic":
			l.createTopic(ac)
		}
	}

	log.Println("Shutting down LSNS...")
}

func newReq(action, reqID string, data map[string]string) request {
	return request{
		action: action,
		reqID:  reqID,
		data:   data,
		resC:   make(chan interface{}),
	}
}

func (l *lSns) createTopic(req request) {
	// TODO
}
