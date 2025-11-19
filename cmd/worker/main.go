package main

import (
	"flag"
	"log"

	"gocerery/internal/config"
	"gocerery/internal/worker"

	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/gocerery-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	if err := worker.Run(&c); err != nil {
		log.Fatalf("worker exited: %v", err)
	}
}
