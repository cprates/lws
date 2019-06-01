package llambda

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// LLambda is the interface to implement if you want to implement your own Lambda service.
type LLambda interface {
	Process(<-chan request, <-chan struct{})
}

// lLambda represents an instance of lLambda core.
type lLambda struct {
	accountID string
	region    string
	scheme    string
	host      string
	// TODO: align
}

type reqResult struct {
	data interface{}
	err  error
	// extra data to be used by some errors like custom messages
	errData interface{}
}

type request struct {
	action string
	id     string
	// request params
	params map[string]string
	// request attributes
	attributes map[string]string
	// channel to read the request result from
	resC chan *reqResult
}

func (l *lLambda) Process(reqC <-chan request, stopC <-chan struct{}) {

forloop:
	for {
		select {
		case req := <-reqC:
			switch req.action {
			default:
				req.resC <- &reqResult{err: fmt.Errorf("%q not implemented", req.action)}
				break
			}
		case <-stopC:
			break forloop
		}
	}

	log.Println("Shutting down LLambda...")
}
