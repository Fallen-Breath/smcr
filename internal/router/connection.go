package router

import (
	"encoding/json"
	"fmt"
	"github.com/Fallen-Breath/smcr/internal/config"
	"github.com/Fallen-Breath/smcr/internal/protocol"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"strconv"
	"sync"
)

type ConnectionHandler struct {
	id         int
	config     *config.Config
	clientConn net.Conn
}

func (h *ConnectionHandler) handleConnection() {
	// ============================== Prepare ==============================

	closeConnection := func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Errorf("[%d] Failed to close connection: %v", h.id, err)
		}
	}
	defer closeConnection(h.clientConn)

	// ============================== Read Handshake Packet ==============================

	connReadWriter := protocol.NewPacketReadWriter(h.clientConn)
	handshakePacket, err := protocol.ReadHandshakePacket(connReadWriter)
	if err != nil {
		log.Errorf("[%d] Failed to read HandshakePacket from client: %v", h.id, err)
		return
	}

	// ============================== Do Route ==============================

	address := fmt.Sprintf("%s:%d", handshakePacket.Address, handshakePacket.Port)
	log.Infof("[%d] Address in HandShake: %s, from %s", h.id, address, h.clientConn.RemoteAddr())
	route := h.config.RouteFor(address)
	if route == nil {
		log.Infof("[%d] Cannot found any endpoint for address %+v, closing connection", h.id, h.clientConn.RemoteAddr())
		return
	}

	log.Infof("[%d] Selected route %s", h.id, route.Name)

	if len(route.Mimic) > 0 {
		host, portStr, err := net.SplitHostPort(route.Mimic)
		if err == nil {
			port, err := strconv.Atoi(portStr)
			if err == nil {
				handshakePacket.Address = host
				handshakePacket.Port = uint16(port)
				log.Errorf("[%d] Modified address&port to %s:%d", h.id, host, port)
			} else {
				log.Errorf("[%d] Invalid port %s: %v", h.id, portStr, err)
			}
		} else {
			log.Errorf("[%d] Invalid mimic address %s: %v", h.id, route.Mimic, err)
		}
	}

	// ============================== Connect to Target ==============================

	target, err := route.ResolveTarget()
	if err != nil {
		log.Errorf("[%d] Failed to resolve target for route %s: %v", h.id, route.Name, err)
		return
	}
	log.Infof("[%d] Connecting to %s", h.id, target)
	targetConn, err := net.DialTimeout("tcp", target, route.Timeout)
	if err != nil {
		log.Errorf("[%d] dial to target address %s failed: %v", h.id, address, err)
		if handshakePacket.NextState == protocol.HandshakeNextStateLogin && len(route.TimeoutMessage) > 0 {
			var data string
			if json.Unmarshal([]byte(route.TimeoutMessage), &json.RawMessage{}) == nil { // it's already a valid json
				data = route.TimeoutMessage
			} else { // not a valid json, treat as plain string
				b, _ := json.Marshal(route.TimeoutMessage)
				data = string(b)
			}
			disconnectPacket := protocol.DisconnectPacket{Reason: data}
			err := protocol.WritePacket(connReadWriter, &disconnectPacket)
			if err != nil {
				log.Errorf("[%d] Failed to send disconnect packet to client: %v", h.id, err)
			}
			log.Debugf("[%d] Sent disconnect packet %+v", h.id, disconnectPacket)
		}
		return
	}
	defer closeConnection(targetConn)

	// ============================== Write Handshake Packet ==============================

	if err := protocol.WritePacket(protocol.NewPacketReadWriter(targetConn), handshakePacket); err != nil {
		log.Errorf("[%d] Failed to write HandshakePacket to target: %v", h.id, err)
		return
	}

	// ============================== Start Forwarding ==============================

	h.forward(h.clientConn, targetConn)
	log.Infof("[%d] handleConnection end", h.id)
}

func (h *ConnectionHandler) forward(source net.Conn, target net.Conn) {
	var wg sync.WaitGroup

	singleForward := func(desc string, s net.Conn, t net.Conn) {
		defer wg.Done()
		log.Debugf("[%d] Forward start for %s", h.id, desc)
		n, err := io.Copy(t, s)
		if err != nil {
			log.Warningf("[%d] Forward error for %s: %v", h.id, desc, err)
		}
		log.Debugf("[%d] Forward end for %s, bytes transfered = %d", h.id, desc, n)
	}

	wg.Add(1)
	go singleForward("client -> target", source, target)
	wg.Add(1)
	go singleForward("client <- target", target, source)
	wg.Wait()
}
