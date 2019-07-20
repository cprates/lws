package awsapi

import "github.com/gorilla/mux"

// AwsAPI represents an instance of an AWS CLI compatible interface.
type AwsAPI struct {
	region   string
	account  string
	proto    string
	addr     string
	codePath string
}

// Install an AWS CLI compatible interface to serve HTTP requests.
func Install(router *mux.Router, region, account, proto, addr, codePath string) AwsAPI {

	awsAPI := AwsAPI{
		region:   region,
		account:  account,
		proto:    proto,
		addr:     addr,
		codePath: codePath,
	}
	awsAPI.InstallSQS(router, region, account, proto, addr)
	awsAPI.InstallLambda(router, region, account, proto, addr, codePath)

	return awsAPI
}
