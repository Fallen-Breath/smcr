package config

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
	"time"
)

const DefaultRouteName = "default"

type RouteAction string

const (
	Forward RouteAction = "forward" // forward the connection to the given target
	Reject  RouteAction = "reject"  // reject and close the connection
)

type Route struct {
	Name    string      `yaml:"name"`
	Matches []string    `yaml:"matches"`          // match any of them -> use this route. Port is optional. Addresses with port has higher priority
	Action  RouteAction `yaml:"action,omitempty"` // how to deal with the client connection

	// forward action
	Target          string        `yaml:"target,omitempty"`            // The target server to route for. Port is optional (use 25565 if absent)
	Mimic           string        `yaml:"mimic,omitempty"`             // optional
	Timeout         time.Duration `yaml:"timeout_ms,omitempty"`        // optional, default DefaultConnectTimeout
	DialFailMessage string        `yaml:"dial_fail_message,omitempty"` // if given, send this to the client if dial failed

	// haproxy protocol
	ProxyProtocol int `yaml:"proxy_protocol,omitempty"` // if given, send proxy protocol header to the target server using given version (1 or 2)

	// reject action
	RejectMessage string `yaml:"reject_message,omitempty"` // if given, disconnect the client with the given message, so client knows what happens

	// processed json version of RejectMessage and TimeoutMessage
	rejectMessageJson   string `yaml:"-"`
	dialFailMessageJson string `yaml:"-"`
}

type Config struct {
	Listen                string        `yaml:"listen"`
	Debug                 bool          `yaml:"debug"`
	Routes                []Route       `yaml:"routes"`
	DefaultConnectTimeout time.Duration `yaml:"default_connect_timeout"`  // optional, default 3s
	SrvLookupTimeout      time.Duration `yaml:"srv_lookup_timeout"`       // optional, default 3s
	ProxyProtocol         bool          `yaml:"proxy_protocol,omitempty"` // if client can send proxy protocol header to smcr. if true, PP header will be required

	routeMap     map[string]*Route `yaml:"-"` // match_addr (lowered case) -> route
	defaultRoute *Route            `yaml:"-"`
}

func validateAddress(what string, address string, mustWithPort bool) {
	if len(address) == 0 {
		log.Fatalf("Field %s is empty", what)
	}

	addrToTest := address
	if !mustWithPort && !strings.Contains(address, ":") {
		addrToTest = address + ":25565"
	}

	if _, _, err := net.SplitHostPort(addrToTest); err != nil {
		log.Fatalf("Field '%s' with value %s is not a valid address: %v", what, address, err)
	}
}

func formatMessageJson(msg string) string {
	if json.Unmarshal([]byte(msg), &json.RawMessage{}) == nil { // it's already a valid json
		return msg
	} else { // not a valid json, treat as a plain string
		b, _ := json.Marshal(msg)
		return string(b)
	}
}

func (c *Config) Init() {
	// set log level first
	if c.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// fill default values
	if c.DefaultConnectTimeout <= 0 {
		c.DefaultConnectTimeout = 3 * time.Second
	}
	if c.SrvLookupTimeout <= 0 {
		c.SrvLookupTimeout = 3 * time.Second
	}
	for i := range c.Routes {
		route := &c.Routes[i]
		if route.Timeout <= 0 {
			route.Timeout = c.DefaultConnectTimeout
		}
		if len(route.Action) == 0 {
			route.Action = Forward
		}
	}

	// validate
	validateAddress("listen", c.Listen, true)
	for i := range c.Routes {
		route := &c.Routes[i]
		for j := range route.Matches {
			validateAddress(fmt.Sprintf("routes[%d]match[%d]", i, j), route.Matches[j], false)
		}
		if len(route.Target) > 0 {
			validateAddress(fmt.Sprintf("routes[%d]target", i), route.Target, false)
		} else {
			log.Fatalf("routes[%d] does not specify the target", i)
		}
		if len(route.Mimic) > 0 {
			validateAddress(fmt.Sprintf("routes[%d]mimic", i), route.Mimic, true)
		}
		switch route.Action {
		case Forward, Reject:
			// ok
		default:
			log.Fatalf("unknown route acion %s", route.Action)
		}
		if !(0 <= route.ProxyProtocol && route.ProxyProtocol <= 2) {
			log.Fatalf("routes[%d] declares invalid proxy protocol version %d, should be 1 or 2", i, route.ProxyProtocol)
		}
	}

	// adjust values
	for i := range c.Routes {
		route := &c.Routes[i]
		if len(route.RejectMessage) > 0 {
			route.rejectMessageJson = formatMessageJson(route.RejectMessage)
		}
		if len(route.DialFailMessage) > 0 {
			route.dialFailMessageJson = formatMessageJson(route.DialFailMessage)
		}
	}

	// gather
	c.routeMap = make(map[string]*Route)
	c.defaultRoute = nil
	for i := range c.Routes {
		route := &c.Routes[i]
		if route.Name == DefaultRouteName {
			c.defaultRoute = route
			if len(route.Matches) > 0 {
				log.Warnf("'matches' field for default route is useless")
			}
		} else {
			for _, addr := range route.Matches {
				key := strings.ToLower(addr)
				if existed, ok := c.routeMap[key]; ok {
					log.Warnf("Duplicated route match %s, found in %s and %s", addr, existed.Name, route.Name)
				}
				c.routeMap[key] = route
			}
		}
	}
}

func (c *Config) Dump() {
	sr := func(r *Route) string {
		s := r.Target
		if len(r.Mimic) > 0 {
			s += fmt.Sprintf(" (mimic %s)", r.Mimic)
		}
		return s
	}

	log.Debugf("Route map (len=%d):", len(c.routeMap))
	for addr, route := range c.routeMap {
		log.Debugf("- %s -> %s", addr, sr(route))
	}
	if c.defaultRoute != nil {
		log.Debugf("* default route -> %s", sr(c.defaultRoute))
	}
}

// ---------------------- getters ----------------------

func (r *Route) GetRejectMessageJson() string {
	return r.rejectMessageJson
}

func (r *Route) GetDialFailMessageJson() string {
	return r.dialFailMessageJson
}

func (c *Config) GetRouteMap() map[string]*Route {
	return c.routeMap
}

func (c *Config) GetDefaultRoute() *Route {
	return c.defaultRoute
}
