[![Build Status](https://travis-ci.com/cprates/lws.svg?token=xhTpgcEXoSMvxWuq6XB2&branch=master)](https://travis-ci.com/cprates/lws)
[![Go Report Card](https://goreportcard.com/badge/github.com/cprates/lws)](https://goreportcard.com/report/github.com/cprates/lws)
[![License](https://img.shields.io/badge/license-Apache%202-blue.svg)](https://github.com/cprates/lws/blob/master/LICENSE)

# LWS
LWS (Local Web Services) is a set of mocks for AWS services that allows you to test you code offline
and without the need of an account.

## Services

* **SQS**
* **Lambda** (in development)

## Requirements
* [Golang](https://golang.org/doc/install): Currently it only runs localy so, you'll need to
download and install Golang. Make sure you have configured your [GOPATH](https://github.com/golang/go/wiki/SettingGOPATH).

## SQS
For a list of supported features check the [Wiki](https://github.com/cprates/lws/wiki/LSQS).

## Lambda
For a list of supported features check the [Wiki](https://github.com/cprates/lws/wiki/LLambda).

### Logs
Lambda's log files are stored in ```{your-lws-folder}/logs/lambda``` by default. Change it by setting
```$LWS_LOGS_FOLDER``` to a different folder.

Log files are owned by the user that started LWS. Set ```USER_UID``` and ```GROUP_UID``` to change the 
ownership of the log files.


## Installation
After installing Golang and configuring your GOPATH clone the *LWS* repository:
```
git clone https://github.com/cprates/lws.git
```

## Run LWS
```
> cd lws
> make install
> make run
```

## Testing SQS
Create a new queue
```
aws sqs create-queue --endpoint-url http://localhost:8080 --queue-name queue1
```

Check if it was created
```
aws sqs list-queues --endpoint-url http://localhost:8080
```

Send a message (use the queue-url returned by the previous command)
```
aws sqs send-message --endpoint-url http://localhost:8080 --queue-url "$QueueURL" --message-body "Hello!"
```

Read the message
```
aws sqs receive-message --endpoint-url http://localhost:8080 --queue-url "$QueueURL"
```

## Testing Lambda
Build and package the sample lambda function
```
go build cmd/sample-lambda/main.go && zip main.zip main && rm main
```

Create a new Lambda function
```
aws lambda create-function \
    --function-name MyFunc1 \
    --runtime go1.x \
    --role dummy-role \
    --handler main \
    --zip-file fileb://main.zip \
    --endpoint-url http://localhost:8080 \
    --environment '{"Variables":{"MyEnvVar":"value1"}}'
```

Invoke the new function
```
aws lambda invoke \
    --endpoint-url http://localhost:8080 \
    --function-name MyFunc1 \
    --payload '{"value":"Running a test"}' \
    --log-type None result.log 
```

Check the logs
```
cat logs/lambda/MyFunc1/0/stdout
```

Check the return
```
cat result.log
```
