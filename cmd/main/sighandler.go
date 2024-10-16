// signal handler handles signals sent by processes
// https://pkg.go.dev/syscall
// signals are sent to the signal channel

package main

import (
	"os/signal"
	"syscall"
)

func sigHandler() {
	signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for signal := range signalChannel {
		switch signal {
		case syscall.SIGHUP:
			// fmt.Println("Caught signal SIGHUP")
			d.Log().Info("Caught signal SIGHUP")
		default:
			d.Log().Warning("Caught signal ", signal.String())
			d.Shutdown()
			return
		}
	}
}
