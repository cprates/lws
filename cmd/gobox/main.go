package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"

	"github.com/aws/aws-lambda-go/lambda/messages"

	"github.com/cprates/lws/pkg/box"
	"github.com/cprates/lws/pkg/llambda"
)

const lambdaPort string = "12345"

type Server struct {
	listener net.Listener
}

func (s *Server) Exec(args *llambda.LambdaArgs, reply *string) (err error) {
	// TODO: this is being shared with the host a it shouldn't
	//fmt.Println(os.Environ())
	fmt.Println("Agent PID:", os.Getpid())
	fmt.Print("Agent CWD: ")
	fmt.Println(os.Getwd())
	fmt.Print("Agent Hostname: ")
	fmt.Println(os.Hostname())

	lambdaArgs := args

	// executes our lambda
	conn, err := net.Dial("tcp", "127.0.0.1:"+lambdaPort)
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
	res := messages.InvokeResponse{}

	err = c.Call("Function.Invoke", &req, &res)
	if err != nil {
		fmt.Println("Err2:", err)
		fmt.Println("Err2:", res)
		return
	}
	fmt.Println("res:", string(res.Payload))
	if res.Error != nil {
		// ShouldExit is set at Function.Invoke when the function panics, but don't know what it does...
		//   May be this has something to do with the InvokeOutput.Function error Handled and UnHandled.
		//   invoke a function that panics and check the results. Could be used to shutdown the lambda
		//    container because it panic
		// StackTrace is for panics as well
		fmt.Printf("res: %+v\n", *res.Error)
	}

	*reply = string(res.Payload)
	return
}

func (s *Server) Ping(args *box.BoxArg, reply *string) (err error) {
	*reply = "Pong!"
	return
}

func (s *Server) Shutdown(args *box.BoxArg, reply *string) (err error) {
	fmt.Println("Shutting down...")
	//err = s.listener.Close()
	if err != nil {
		// TODO: right way to log? or should be to stderr directly?
		fmt.Println(err)
		return
	}

	*reply = "Shutting down..."

	return
}

func runLambda() {

	entryPoint := os.Args[2]
	if err := os.Setenv("AWS_LAMBDA_FUNCTION_NAME", os.Args[3]); err != nil {
		return
	}
	if err := os.Setenv("_LAMBDA_SERVER_PORT", lambdaPort); err != nil {
		return
	}

	// setup env vars
	for i := 4; len(os.Args) > i+1; i += 2 {
		e := os.Setenv(os.Args[i], os.Args[i+1])
		if e != nil {
			fmt.Println("error setting user env vars:", e)
		}
	}

	// initialise our lambda. At this stage it will only be waiting for us to send the request
	execErrC := make(chan error, 1)

	cmd := exec.Command(entryPoint)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		e := cmd.Run()
		fmt.Println("Error from lambda execution:", e)
		execErrC <- e
	}()
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
		err := l.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
	}()

	err = rpc.Register(&Server{listener: l})
	if err != nil {
		fmt.Println(err)
		return
	}

	runLambda()
	rpc.Accept(l)

	fmt.Println()
	fmt.Printf("terminating gobox %q running at %s", os.Args[3], os.Args[1])
}
