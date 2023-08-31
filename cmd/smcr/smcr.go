package main

import (
	"flag"
	"github.com/Fallen-Breath/smcr/internal/config"
	"github.com/Fallen-Breath/smcr/internal/logging"
	"github.com/Fallen-Breath/smcr/internal/router"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logging.InitLog()

	flagConfig := flag.String("c", "config.yml", "Path to the config yaml file. Default: config.yml")
	flagHelp := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *flagHelp {
		flag.Usage()
		return
	}

	cfg := config.LoadConfigOrDie(*flagConfig)
	cfg.Dump()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Infof("SMCR starting")
	r := router.NewMinecraftRouter(cfg)
	go r.Run()

	sig := <-ch
	log.Infof("Terminating by %s", sig)
	r.Stop()
	log.Infof("SMCR stopped")
}
