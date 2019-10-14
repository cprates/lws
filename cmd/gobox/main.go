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

const readySignal string = "#booted#"

type Server struct {
	listener net.Listener
}

func (s *Server) Exec(args *llambda.LambdaArgs, reply *messages.InvokeResponse) (err error) {
	// TODO: env vars are being shared with the host which I think is expected the way
	//  this is done right now but, maybe it shouldn't have some env vars from the host...
	//fmt.Println(os.Environ())
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
	fmt.Println("res:::", string(reply.Payload))
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

	entryPoint := os.Args[2]
	if err = os.Setenv("AWS_LAMBDA_FUNCTION_NAME", os.Args[3]); err != nil {
		err = fmt.Errorf("setting AWS_LAMBDA_FUNCTION_NAME: %s", err)
		return
	}
	if err = os.Setenv("_LAMBDA_SERVER_PORT", lambdaPort); err != nil {
		err = fmt.Errorf("setting _LAMBDA_SERVER_PORT: %s", err)
		return
	}

	// setup env vars
	for i := 4; len(os.Args) > i+1; i += 2 {
		if err = os.Setenv(os.Args[i], os.Args[i+1]); err != nil {
			err = fmt.Errorf("gobox: setting user env vars: %s", err)
			return
		}
	}

	cmd := exec.Command(entryPoint)
	// TODO: re-drive this later
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
func lambdaReady(addr string) bool {

	for i := 0; i < 4; i++ {
		// this is a local connection without DNS resolution so it should be fast. The testes
		// revealed to take an average of ~300 microseconds. But because this may vary depending
		// on the hardware it is running, let's make it a bit more robust with 2ms
		conn, err := net.DialTimeout("tcp", addr, 2*time.Millisecond)

		if err != nil {
			e, ok := err.(*net.OpError)

			if ok && e.Op == "dial" {
				fmt.Printf("Failed to check lambda status, attempt %d: %s\n", i+1, e)
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

// Usage: ./gobox port entrypoint boxName <env vars>...
//        Env vars are passed in the form: EnvName1 EnvVal1 EnvVar2 EnvVal2
func main() {

	addr := ":" + os.Args[1]

	fmt.Println("Starting rpc server at", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		// ignore error
		_ = l.Close()
	}()

	rpcSrv := rpc.NewServer()
	err = rpcSrv.Register(&Server{listener: l})
	if err != nil {
		fmt.Println("Failed to register rpc service:", err)
		return
	}

	err = runLambda()
	if err != nil {
		fmt.Println("Failed to launch lambda:", err)
		return
	}

	lambdaReady(lambdaAddr)

	// Send the signal to the Box system indicating the lambda has been launched successfully
	n, err := os.Stdout.WriteString(readySignal)
	if err != nil {
		fmt.Println("Failed to signal box system:", err)
		return
	}
	if n != len(readySignal) {
		fmt.Printf(
			"Failed to write signal to box system, wrote %d bytes, expected %d",
			n, len(readySignal),
		)
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
		os.Args[3], os.Args[1],
	)

	wg.Wait()

	fmt.Printf("Terminating gobox %q running at %s\n", os.Args[3], os.Args[1])
}
