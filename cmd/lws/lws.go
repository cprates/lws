package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/cprates/lws/cmd/lws/api/awscli"
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

	addr := viper.GetString("service.addr")
	proto := viper.GetString("service.protocol")
	region := viper.GetString("service.region")
	account := viper.GetString("service.accountId")

	s := newServer()
	awscli.Install(s.router, region, account, proto, addr)

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, s.router))
}
