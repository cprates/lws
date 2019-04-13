package api

import (
	"errors"
	"net/url"
)

func createQueue(params url.Values, method string) error {
	return errors.New("Not Implemented")
}

func listQueues(params url.Values, method string) error {
	return errors.New("Not Implemented")
}

func deleteQueue(params url.Values, method string) error {
	return errors.New("Not Implemented")
}

func (a AwsCli) InstallSQS() {
	a.regAction("CreateQueue", createQueue).
		regAction("ListQueues", listQueues).
		regAction("DeleteQueue", deleteQueue)
}
