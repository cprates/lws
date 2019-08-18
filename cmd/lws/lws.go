package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/cprates/lws/cmd/lws/api/aws"
	"github.com/cprates/lws/pkg/lsqs"
)

type Config struct {
	Debug   bool
	Service struct {
		Protocol  string
		Addr      string
		Region    string
		AccountId string
	}
	Lambda struct {
		Workfolder string
	}
}

var Conf Config

func init() {

	f, err := os.Open("./config.yaml")
	if err != nil {
		panic(fmt.Errorf("fatal error opening config file: %s", err))
	}
	defer f.Close()

	confBuf, err := ioutil.ReadAll(f)
	if err != nil {
		panic(fmt.Errorf("fatal error reading config file: %s", err))
	}

	Conf = Config{}
	err = yaml.Unmarshal(confBuf, &Conf)
	if err != nil {
		panic(fmt.Errorf("fatal error loading config: %s", err))
	}

	log.StandardLogger().SetNoLock()
	if Conf.Debug {
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

	region := Conf.Service.Region
	account := Conf.Service.AccountId
	proto := Conf.Service.Protocol
	addr := Conf.Service.Addr
	lambdaFolder := Conf.Lambda.Workfolder

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Errorln(err)
		return
	}
	if host == "" {
		log.Panic("'service.addr' must be on the form 'host:port'")
	}

	s := newServer()
	cwd, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}
	lambdaWorkdir := filepath.Join(cwd, lambdaFolder)
	awsAPI := aws.New(
		region,
		account,
		proto,
		addr,
	)

	wg := sync.WaitGroup{}
	stopC := make(chan struct{})
	wg.Add(1)
	_, err = awsAPI.InstallLambda(s.router, region, account, proto, addr, lambdaWorkdir, stopC, &wg)
	if err != nil {
		log.Errorln(err)
		return
	}

	wg.Add(1)
	pushC := lsqs.Start(account, region, proto, addr, stopC, &wg)
	awsAPI.InstallSQS(s.router, pushC)

	httpSrv := &http.Server{Addr: addr, Handler: s.router}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println()
		log.Println("Shutting down...")
		e := httpSrv.Shutdown(context.Background())
		if e != nil {
			log.Errorln(err)
		}
		close(stopC)
	}()

	log.Println("Listening on", addr)
	err = httpSrv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatalln(err)
	}

	shutTimeout := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		shutTimeout <- struct{}{}
	}()

	select {
	case <-shutTimeout:
	case <-time.After(1 * time.Second):
		log.Errorln("some services din't shutdown in time")
	}
}
