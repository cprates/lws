package llambda

import (
	"fmt"
	"net/rpc"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda/messages" // TODO: instead of adding the dependency, replicate the struct?
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/pkg/list"
)

type function struct {
	description   string
	envVars       map[string]string
	handler       string
	lrn           string
	memorySize    int
	name          string
	publish       bool
	revID         string
	role          string
	runtime       string
	version       string
	idleInstances *list.List
}

type instance struct {
	parent *function
	id     string
	port   int
}

func (i *instance) Exec(arg interface{}) (reply *messages.InvokeResponse, err error) {

	// TODO: log debug on the given logger
	log.Printf(
		"Connecting to %s[%s] at %s:%d...\n",
		i.parent.name, i.id, "lws", i.port, // TODO: "lws" was box.agentHost before
	)
	// instead of keeping a connection alive, connect to the box every time we need
	boxAddr := "lws" + ":" + strconv.Itoa(i.port) // TODO: "lws" was box.agentHost before
	c, err := rpc.Dial("tcp", boxAddr)
	if err != nil {
		err = fmt.Errorf("dial to box at %q failed: %s", boxAddr, err)
		return
	}
	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	// TODO: log debug on the given logger
	log.Printf("Executing agent on box %s[%s]\n", i.parent.name, i.id)
	reply = &messages.InvokeResponse{}
	err = c.Call("Server.Exec", arg, reply)
	if err != nil {
		err = fmt.Errorf("call to box failed: %s", err)
		return
	}

	return
}
