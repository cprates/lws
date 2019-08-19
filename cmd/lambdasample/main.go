package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

type MyEvent struct {
	Name string `json:"name"`
}

func HandleRequest(ctx context.Context, name MyEvent) (string, error) {
	fmt.Println("Handling request...")
	fmt.Println("Env TEST_ENV:", os.Getenv("TEST_ENV"))
	fmt.Println("Env _LAMBDA_SERVER_PORT:", os.Getenv("_LAMBDA_SERVER_PORT"))
	fmt.Println("Env AWS_LAMBDA_FUNCTION_NAME:", os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))
	return fmt.Sprintf("Hello %s!", name.Name), nil
}

func main() {
	lambda.Start(HandleRequest)
}
