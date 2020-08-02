package main

import (
	"context"
	"fmt"
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

	"github.com/cprates/lws/cmd/lws/api/aws"
	"github.com/cprates/lws/pkg/lsqs"
)

func init() {

	log.StandardLogger().SetNoLock()
	if os.Getenv("LWS_DEBUG") == "1" {
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
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)

	log.Println("Starting LWS...")

	region := os.Getenv("AWS_DEFAULT_REGION")
	account := os.Getenv("LWS_ACCOUNT_ID")
	proto := os.Getenv("LWS_PROTO")
	addr := net.JoinHostPort(os.Getenv("LWS_IP"), os.Getenv("LWS_PORT"))
	lambdaFolder := os.Getenv("LWS_LAMBDA_WORKDIR")
	network := os.Getenv("LWS_DOCKER_SUBNET")
	bridgeIfName := os.Getenv("LWS_IF_BRIDGE")
	gatewayIP := os.Getenv("LWS_IP")
	nameServerIP := os.Getenv("LWS_NAMESERVER")

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
	_, err = awsAPI.InstallLambda(
		s.router, region, account, proto, addr, network, gatewayIP, bridgeIfName, nameServerIP,
		lambdaWorkdir, stopC, &wg, log.NewEntry(log.StandardLogger()), os.Getenv("LWS_DEBUG"),
	)
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
