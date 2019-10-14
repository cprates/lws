package llambda

import (
	"fmt"
	"net"
	"net/rpc"
	"time"

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
	timeout       int // seconds
	version       string
	idleInstances *list.List
}

type instance struct {
	parent       *function
	id           string
	ip           string
	port         string
	lifeDeadline time.Time
}

func (i *instance) Exec(arg interface{}) (reply *messages.InvokeResponse, err error) {

	instanceAddr := net.JoinHostPort(i.ip, i.port)

	log.Printf("Connecting to %s[%s] at %s...\n", i.parent.name, i.id, instanceAddr)

	// instead of keeping a connection alive, connect to the box every time we need
	conn, err := net.DialTimeout("tcp", instanceAddr, 5*time.Millisecond)
	if err != nil {
		err = fmt.Errorf("dial to box at %q failed: %s", instanceAddr, err)
		return
	}
	c := rpc.NewClient(conn)

	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	// update deadline
	i.lifeDeadline = time.Now().Add(time.Duration(i.parent.timeout) * time.Second)

	log.Printf("Executing agent on box %s[%s]\n", i.parent.name, i.id)
	reply = &messages.InvokeResponse{}
	err = c.Call("Server.Exec", arg, reply)
	if err != nil {
		err = fmt.Errorf("call to box failed: %s", err)
		return
	}

	return
}

// Shutdown is for shutdown a lambda's instance by calling Shutdown method on the remote
// instance using the RPC protocol.
func (i *instance) Shutdown() (err error) {

	instanceAddr := net.JoinHostPort(i.ip, i.port)
	conn, err := net.DialTimeout("tcp", instanceAddr, 5*time.Millisecond)
	if err != nil {
		err = fmt.Errorf("dial to %q: %s", instanceAddr, err)
		return
	}
	c := rpc.NewClient(conn)

	defer func() {
		e := c.Close()
		if err == nil {
			err = e
		}
	}()

	err = c.Call("Server.Shutdown", "", nil)
	if err != nil {
		err = fmt.Errorf("rpc call: %s", err)
		return
	}

	return
}
