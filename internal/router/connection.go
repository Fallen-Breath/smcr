package router

import (
	"encoding/json"
	"fmt"
	"github.com/Fallen-Breath/smcr/internal/config"
	"github.com/Fallen-Breath/smcr/internal/dns"
	"github.com/Fallen-Breath/smcr/internal/protocol"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ConnectionHandler struct {
	id         int
	config     *config.Config
	clientConn net.Conn
	logger     *log.Entry
}

const handshakeMaxTimeWait = 20 * time.Second

func NewConnectionHandler(id int, cfg *config.Config, clientConn net.Conn) *ConnectionHandler {
	h := &ConnectionHandler{
		id:         id,
		config:     cfg,
		clientConn: clientConn,
	}
	h.logger = log.WithField("client_id", id)
	return h
}

func (h *ConnectionHandler) closeConnection(name string, conn net.Conn) {
	err := conn.Close()
	if err != nil {
		h.logger.Errorf("Failed to close %s connection: %v", name, err)
	}
}

func (h *ConnectionHandler) handleConnection() {
	// ============================== Prepare ==============================

	var once sync.Once
	closeClientConn := func() {
		h.closeConnection("client", h.clientConn)
	}
	defer once.Do(closeClientConn)

	// ============================== Read Handshake Packet ==============================

	handshakeTimeout := false
	timer := time.AfterFunc(handshakeMaxTimeWait, func() {
		h.logger.Errorf("Wait for handshake packet times out (%.0fs), closing connection", handshakeMaxTimeWait.Seconds())
		handshakeTimeout = true
		once.Do(closeClientConn)
	})
	connReadWriter := protocol.NewPacketReadWriter(h.clientConn)
	handshakePacket, err := protocol.ReadHandshakePacket(connReadWriter)
	timer.Stop()
	if err != nil {
		if !handshakeTimeout {
			h.logger.Errorf("Failed to read handshake packet from client: %v", err)
		}
		return
	}

	// ============================== Do Route ==============================

	h.logger.Infof("Address in handshake packet: %s:%d", handshakePacket.Hostname, handshakePacket.Port)
	route := h.RouteFor(handshakePacket.Hostname, handshakePacket.Port)
	if route == nil {
		h.logger.Infof("Cannot found any endpoint for address %+v, closing connection", h.clientConn.RemoteAddr())
		return
	}

	h.logger.Infof("Selected route '%s'", route.Name)

	if len(route.Mimic) > 0 {
		host, portStr, err := net.SplitHostPort(route.Mimic)
		if err == nil {
			port, err := strconv.Atoi(portStr)
			if err == nil {
				handshakePacket.Hostname = host
				handshakePacket.Port = uint16(port)
				h.logger.Infof("Modified address in handshake packet to %s:%d", host, port)
			} else {
				h.logger.Errorf("Invalid port %s: %v", portStr, err)
			}
		} else {
			h.logger.Errorf("Invalid mimic address %s: %v", route.Mimic, err)
		}
	}

	// ============================== Connect to Target ==============================

	target, err := h.resolveTarget(route)
	if err != nil {
		h.logger.Errorf("Failed to resolve target for route %s: %v", route.Name, err)
		return
	}

	h.logger.Infof("Dialing to target %s", target)
	t := time.Now()
	targetConn, err := net.DialTimeout("tcp", target, route.Timeout)
	h.logger.Debugf("Dial cost %dms", time.Now().Sub(t).Milliseconds())
	if err != nil {
		h.logger.Errorf("Dial to target %s failed: %v", target, err)
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
				h.logger.Errorf("Failed to send disconnect packet to client: %v", err)
			}
			h.logger.Debugf("Sent disconnect packet %+v", disconnectPacket)
		}
		return
	}
	defer h.closeConnection("target", targetConn)

	// ============================== Write Handshake Packet ==============================

	if err := protocol.WritePacket(protocol.NewPacketReadWriter(targetConn), handshakePacket); err != nil {
		h.logger.Errorf("Failed to write handshake packet to target: %v", err)
		return
	}

	// ============================== Start Forwarding ==============================

	h.logger.Infof("Start forwarding")
	h.forward(h.clientConn, targetConn)

	h.logger.Infof("Client connection end")
}

func (h *ConnectionHandler) forward(source net.Conn, target net.Conn) {
	var wg sync.WaitGroup

	singleForward := func(desc string, s net.Conn, t net.Conn) {
		defer wg.Done()
		h.logger.Debugf("Forward start for %s", desc)
		n, err := io.Copy(t, s)
		if err != nil {
			h.logger.Warningf("Forward error for %s: %v", desc, err)
		}
		h.logger.Debugf("Forward end for %s, bytes transfered = %d", desc, n)
	}

	wg.Add(1)
	go singleForward("client -> target", source, target)
	wg.Add(1)
	go singleForward("client <- target", target, source)
	wg.Wait()
}

// RouteFor might return nullable
func (h *ConnectionHandler) RouteFor(hostname string, port uint16) *config.Route {
	address := fmt.Sprintf("%s:%d", hostname, port)
	routeMap := h.config.GetRouteMap()

	if route, ok := routeMap[address]; ok {
		h.logger.Debugf("Selected route %s for address %s", route.Name, address)
		return route
	}
	if route, ok := routeMap[hostname]; ok {
		h.logger.Debugf("Selected route %s for hostname %s", route.Name, address)
		return route
	}

	if defaultRoute := h.config.GetDefaultRoute(); defaultRoute != nil {
		h.logger.Debugf("Selected default route for address %s", address)
		return defaultRoute
	}

	h.logger.Debugf("No valid route for address %s", address)
	return nil
}

func (h *ConnectionHandler) resolveTarget(route *config.Route) (string, error) {
	if !strings.Contains(route.Target, ":") { // no port, might be an SRV record
		t := time.Now()
		resolved, err := dns.ResolveSrv(route.Target, h.config.SrvLookupTimeout)
		h.logger.Debugf("SRV Resolution for %s cost %dms", route.Target, time.Now().Sub(t).Milliseconds())

		if err == nil {
			return resolved, nil
		} else {
			h.logger.Debugf("Resolved SRV record for %s failed: %v", route.Target, err)
		}
		return fmt.Sprintf("%s:25565", route.Target), nil
	}
	return route.Target, nil
}
