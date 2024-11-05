package main

import "flag"

func main() {
	// get config from cmd and load into system
	var cflag string
	flag.StringVar(&cflag, "config", "config.json", "configuration file with path")
	flag.Parse()
	serve(cflag)
}
