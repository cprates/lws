package main

import (
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"

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
}

func main() {

	log.Println("Starting LWS...") // TODO: log port and host

	awsCli := api.NewAwsCli()
	awsCli.InstallSQS("us-est-2", "0000000000", "http", "localhost:8080") // TODO: this must come from a config file
	awsCli.InstallSNS("us-est-2", "0000000000", "http", "localhost:8080") // TODO: this must come from a config file

	s := newServer()
	s.regRoute("/", awsCli.Dispatcher())

	log.Fatal(http.ListenAndServe(":8080", nil)) // TODO: host and port must be in a config file
}
