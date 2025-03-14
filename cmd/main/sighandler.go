package main

import (
	"os/signal"
	"syscall"
)

// Listen for system signals and handle them accordingly
func sigHandler() {
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGABRT,
	)
	for signal := range signalChannel {
		switch signal {
		case syscall.SIGHUP:
			d.Log().Infof("Caught signal %s, reloading configuration file")
			err := d.ReloadConfigFile(configFile)
			if err != nil {
				d.Log().Error("Failed to reload configuration file")
			}
		default:
			d.Log().Warningf("Caught signal %s, initiating graceful shutdown", signal.String())
			d.Shutdown()
			return
		}
	}
}
