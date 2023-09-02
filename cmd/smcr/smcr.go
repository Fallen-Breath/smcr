package main

import (
	"flag"
	"fmt"
	"github.com/Fallen-Breath/smcr/internal/config"
	"github.com/Fallen-Breath/smcr/internal/constants"
	"github.com/Fallen-Breath/smcr/internal/logging"
	"github.com/Fallen-Breath/smcr/internal/router"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logging.InitLog()

	flagConfig := flag.String("c", "config.yml", "Path to the config yaml file")
	flagShowHelp := flag.Bool("h", false, "Show help and exit")
	flagShowVersion := flag.Bool("v", false, "Show version and exit")
	flag.Parse()

	if *flagShowHelp {
		flag.Usage()
		return
	}
	if *flagShowVersion {
		fmt.Printf("SMCR v%s\n", constants.Version)
		return
	}

	cfg := config.LoadConfigOrDie(*flagConfig)
	cfg.Dump()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Infof("SMCR v%s starting", constants.Version)
	r := router.NewMinecraftRouter(cfg)
	go r.Run()

	sig := <-ch
	log.Infof("Terminating by signal %s", sig)
	r.Stop()
	log.Infof("SMCR stopped")
}
