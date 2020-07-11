package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

type MyEvent struct {
	Value string `json:"value"`
}

func HandleRequest(ctx context.Context, evt MyEvent) (string, error) {
	log.Println("Handling request...")
	log.Println("Event Value:", evt.Value)
	log.Println("Env MyEnvVar:", os.Getenv("MyEnvVar"))

	resp, err := http.Get("http://example.com/")
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return "error!", err
	}

	log.Printf("Page source:\n%s", string(body))

	return "done!", err
}

func main() {
	lambda.Start(HandleRequest)
}
