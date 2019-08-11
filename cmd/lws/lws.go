package main

import (
	"fmt"
	"net"
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

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Errorln(err)
		return
	}
	if host == "" {
		log.Panic("'service.addr' must be on the form 'host:port'")
	}

	s := newServer()
	awsAPI := aws.New(
		region,
		account,
		proto,
		addr,
		codePath,
	)

	stopC := make(chan struct{})
	pushC := lsqs.Start(account, region, proto, addr, stopC)
	awsAPI.InstallSQS(s.router, pushC)

	awsAPI.InstallLambda(s.router, region, account, proto, addr, codePath)

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, s.router))
}
