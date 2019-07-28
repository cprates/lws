package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/cprates/lws/cmd/lws/api/aws"
	"github.com/cprates/lws/pkg/lsqs"
)

func init() {

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	log.StandardLogger().SetNoLock()
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.SetReportCaller(true)
	log.SetFormatter(
		&log.TextFormatter{
			DisableLevelTruncation: true,
			FullTimestamp:          true,
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				_, fileName := filepath.Split(frame.File)
				file = " " + fileName + ":" + strconv.Itoa(frame.Line) + " #"
				return
			},
		},
	)
}

func main() {

	log.Println("Starting LWS...")

	region := viper.GetString("service.region")
	account := viper.GetString("service.accountId")
	proto := viper.GetString("service.protocol")
	addr := viper.GetString("service.addr")
	codePath := viper.GetString("lambda.codePath")

	s := newServer()
	awsAPI := aws.New(
		region,
		account,
		proto,
		addr,
		codePath,
	)

	stopC := make(chan struct{})
	pushC := lsqs.Init(account, region, proto, addr, stopC)

	awsAPI.InstallSQS(
		s.router,
		func(action, reqID string, params, attributes map[string]string) (res aws.SqsResult) {
			r := lsqs.PushReq(pushC, action, reqID, params, attributes)
			res.Data = r.Data
			res.Err = r.Err
			res.ErrData = r.ErrData
			return
		},
	)

	awsAPI.InstallLambda(s.router, region, account, proto, addr, codePath)

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, s.router))
}
