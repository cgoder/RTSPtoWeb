package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {

	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()

	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server CORE start")

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	go HTTPAPIServer()
	go RTSPServer()
	go Storage.StreamChannelRunAll(ctx)

	signalChanel := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(signalChanel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChanel
		log.WithFields(logrus.Fields{
			"module": "main",
			"func":   "main",
		}).Info("Server receive signal", sig)
		done <- true
	}()
	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server start success a wait signals")
	<-done
	Storage.StopAll()
	time.Sleep(2 * time.Second)
	log.WithFields(logrus.Fields{
		"module": "main",
		"func":   "main",
	}).Info("Server stop working by signal")
}
