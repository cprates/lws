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
Lambda service is in early development stage. If you are keen and want to follow the development,
check out the branch ```llambda```.

## Installation
After installing Golang and configuring your GOPATH clone the *LWS* repository:
```
git clone https://github.com/cprates/lws.git
```

## Running
```
> cd lws
> make install
> make run
```

## Testing your installation
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