// meat and potatoes

package main

import (
	"fmt"
	"os"

	"github.com/phires/go-guerrilla"
	"github.com/ughhuh/go-email-server/backend"
)

var (
	signalChannel = make(chan os.Signal, 1) // a channel for storing signals
	d             guerrilla.Daemon
)

func init() {
	// confirm config file exists
}

func serve() {
	// start server
	// cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String()}

	d := guerrilla.Daemon{}
	d.AddProcessor("PSQL", backend.PSQL)

	_, err := d.LoadConfig("../../config.json")
	if err != nil {
		fmt.Println("Error loading config!")
		fmt.Println(err)
	}

	err = d.Start()

	if err != nil {
		fmt.Println("start error", err)
	}
	// check max clients is OK

	// call signal handler
	sigHandler()
}
