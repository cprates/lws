package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/cprates/lws/cmd/lws/internal/api"
)

func init() {
	log.StandardLogger().SetNoLock()
	log.SetLevel(log.DebugLevel)
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

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %s", err))
	}
}

func main() {

	log.Println("Starting LWS...")

	addr := viper.GetString("service.addr")
	proto := viper.GetString("service.protocol")
	region := viper.GetString("service.region")
	account := viper.GetString("service.accountId")

	s := newServer()
	api.InstallAwsCli(s.router, region, account, proto, addr)

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, s.router))
}
