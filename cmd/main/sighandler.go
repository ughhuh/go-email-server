// signal handler handles signals sent by processes
// https://pkg.go.dev/syscall
// signals are sent to the signal channel

package main

import (
	"fmt"
	"os/signal"
	"syscall"
)

func sigHandler() {
	signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for signal := range signalChannel {
		switch signal {
		case syscall.SIGHUP:
			fmt.Println("Caugh signal SIGHUP")
		default:
			fmt.Println("a weird signal caught, shutting down just in case")
		}
	}
}
