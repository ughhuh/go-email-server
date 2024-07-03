package main

import (
	"fmt"

	"github.com/phires/go-guerrilla"
	"github.com/phires/go-guerrilla/backends"
	"github.com/phires/go-guerrilla/log"
)

func main() {
	cfg := &guerrilla.AppConfig{LogFile: log.OutputStdout.String()}
	sc := guerrilla.ServerConfig{
		ListenInterface: "127.0.0.1:2526",
		IsEnabled:       true,
	}
	cfg.Servers = append(cfg.Servers, sc)
	bcfg := backends.BackendConfig{
		"save_workers_size":  3,
		"save_process":       "HeadersParser|Header|Hasher|Debugger",
		"log_received_mails": true,
		"primary_mail_host":  "example.com",
	}
	cfg.BackendConfig = bcfg

	d := guerrilla.Daemon{Config: cfg}

	err := d.Start()

	if err != nil {
		fmt.Println("start error", err)
	}
}
