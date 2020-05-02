package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda/messages"

	"github.com/cprates/lws/pkg/llambda"
)

const (
	lambdaPort string = "12345"
	lambdaAddr        = "127.0.0.1:" + lambdaPort
)

// Server exposes the go runtime API.
type Server struct {
	listener net.Listener
}

// Exec executes the lambda.
func (s *Server) Exec(args *llambda.LambdaArgs, reply *messages.InvokeResponse) (err error) {
	fmt.Println("Env vars:", os.Environ())
	fmt.Println("Agent PID:", os.Getpid())
	fmt.Print("Agent CWD: ")
	fmt.Println(os.Getwd())
	fmt.Print("Agent Hostname: ")
	fmt.Println(os.Hostname())

	lambdaArgs := args

	// executes our lambda
	conn, err := net.DialTimeout("tcp", lambdaAddr, 2*time.Millisecond)
	if err != nil {
		err = fmt.Errorf("dail to lambda failed: %s", err)
		return
	}
	defer func() {
		if e := conn.Close(); err != nil {
			err = e
		}
	}()

	c := rpc.NewClient(conn)

	req := messages.InvokeRequest{
		Payload:   lambdaArgs.Body,
		RequestId: lambdaArgs.RequestId,
		// XAmznTraceId          string
		Deadline: messages.InvokeRequest_Timestamp{ // TODO
			Seconds: 5,
			Nanos:   56,
		},
		InvokedFunctionArn: lambdaArgs.Arn,
		// CognitoIdentityId     string
		// CognitoIdentityPoolId string
		// TODO: check lambdacontext.LambdaContext and its exported vars set based on env vars
		// ClientContext: []byte("sdfsdfsdsgdfg"),
	}

	err = c.Call("Function.Invoke", &req, reply)
	if err != nil {
		// TODO: still need to be tested when the process is finished end to end
		fmt.Println("Err2:", err)
		fmt.Println("Err2:", reply)
		return
	}

	if reply.Error != nil {
		// ShouldExit is set at Function.Invoke when the function panics, but don't know what it does...
		//   May be this has something to do with the InvokeOutput.Function error Handled and UnHandled.
		//   invoke a function that panics and check the results. Could be used to shutdown the lambda
		//    container because it panic
		// StackTrace is for panics as well
		fmt.Printf("res: %+v\n", *reply.Error)
	}

	return
}

// Shutdown shuts down the lambda by closing the listener and waiting for request being served to
// finish.
func (s *Server) Shutdown(arg string, reply *string) (err error) {
	fmt.Println("Shutting down gobox...")
	err = s.listener.Close()
	if err != nil {
		return
	}

	*reply = "OK"
	return
}

func runLambda() (err error) {
	if err = os.Setenv("_LAMBDA_SERVER_PORT", lambdaPort); err != nil {
		err = fmt.Errorf("setting _LAMBDA_SERVER_PORT: %s", err)
		return
	}

	cmd := exec.Command(os.Getenv("HANDLER"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go func() {
		e := cmd.Run()
		fmt.Println("Lambda execution returned an error:", e)
	}()

	return
}

// lambdaReady checks if the lambda rpc server is ready to accept connections by trying to connect
// and retrying on dial errors up to 4 times.
func lambdaReady() bool {
	for i := 0; i < 4; i++ {
		// this is a local connection without DNS resolution so it should be fast. The testes
		// revealed to take an average of ~300 microseconds. But because this may vary depending
		// on the hardware it is running, let's make it a bit more robust with 2ms
		conn, err := net.DialTimeout("tcp", lambdaAddr, 2*time.Millisecond)
		if err != nil {
			e, ok := err.(*net.OpError)

			if ok && e.Op == "dial" {
				fmt.Printf("Failed to check lambda status, attempt %d: %s\n", i+1, err)
				// try again after a small delay
				// This value really depends on the machine this is running, so rather have
				// a delay to make this process more robust with less retries
				time.Sleep(5 * time.Millisecond)
				continue
			}
			fmt.Println("Failed checking lambda status:", err)
			return false
		}

		_ = conn.Close()
		return true
	}

	fmt.Println("Lambda seems to not be responding")
	return false
}

func main() {
	addr := ":" + os.Getenv("GOBOX_PORT")

	err := runLambda()
	if err != nil {
		fmt.Println("Failed to launch lambda:", err)
		return
	}

	lambdaReady()

	fmt.Println("Starting rpc server at", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		_ = l.Close()
	}()

	rpcSrv := rpc.NewServer()
	err = rpcSrv.Register(&Server{listener: l})
	if err != nil {
		fmt.Println("Failed to register rpc service:", err)
		return
	}

	wg := sync.WaitGroup{}
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("gobox: accept:", err)
			break
		}
		wg.Add(1)
		go func() {
			rpcSrv.ServeConn(conn)
			wg.Done()
		}()
	}

	fmt.Printf(
		"Waiting for request being served before terminating gobox %q at %s\n",
		os.Getenv("AWS_LAMBDA_FUNCTION_NAME"), addr,
	)

	wg.Wait()

	fmt.Printf("Terminating gobox %q\n", os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))
}
