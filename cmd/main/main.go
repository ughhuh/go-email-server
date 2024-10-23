package main

import "flag"

func main() {
	// get config from cmd
	var cflag string
	var lflag string
	flag.StringVar(&cflag, "config", "config.json", "configuration file with path")
	flag.StringVar(&lflag, "logdir", "./log", "log directory path")
	flag.Parse()
	// pass to serve
	serve(cflag, lflag)
}
