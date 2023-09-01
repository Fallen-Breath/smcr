package router

import (
	"github.com/Fallen-Breath/smcr/internal/config"
	log "github.com/sirupsen/logrus"
	"net"
	"sync"
)

type MinecraftRouter struct {
	stopCh chan struct{}
	config *config.Config
}

func NewMinecraftRouter(config *config.Config) *MinecraftRouter {
	r := &MinecraftRouter{
		stopCh: make(chan struct{}),
		config: config,
	}
	return r
}

func (r *MinecraftRouter) Run() {
	listener, err := net.Listen("tcp", r.config.Listen)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", r.config.Listen, err)
	}
	log.Infof("Listening on %s", r.config.Listen)

	go func() {
		<-r.stopCh
		log.Infof("Closing connection listener")
		_ = listener.Close()
	}()

	var wg sync.WaitGroup

	i := 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("Error accepting connection: %v", err)
			break
		}
		i += 1
		log.Infof("[%d] Accepted connection #%d from %s", i, i, conn.RemoteAddr())

		wg.Add(1)
		go func(id int, conn net.Conn) {
			defer wg.Done()
			handler := NewConnectionHandler(id, r.config, conn)
			handler.handleConnection()
		}(i, conn)
	}

	wg.Wait()
	log.Infof("All connection closed")
}

func (r *MinecraftRouter) Stop() {
	r.stopCh <- struct{}{}
}
