package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
	"time"
)

const DefaultRouteName = "default"

type Route struct {
	Name           string        `yaml:"name"`
	Matches        []string      `yaml:"matches"`         // match any of them -> use this route. Port is optional. Addresses with port has higher priority
	Target         string        `yaml:"target"`          // The target server to route for. Port is optional (use 25565 if absent)
	Mimic          string        `yaml:"mimic"`           // optional
	Timeout        time.Duration `yaml:"timeout_ms"`      // optional, default DefaultConnectTimeout
	TimeoutMessage string        `yaml:"timeout_message"` // optional
}

type Config struct {
	Listen                string        `yaml:"listen"`
	LogLevel              string        `yaml:"log_level"`
	DefaultConnectTimeout time.Duration `yaml:"default_connect_timeout"` // optional, default 3s
	SrvLookupTimeout      time.Duration `yaml:"srv_lookup_timeout"`      // optional, default 3s
	Routes                []Route       `yaml:"routes"`

	routeMap     map[string]*Route `yaml:"-"` // match_addr -> route
	defaultRoute *Route            `yaml:"-"`
}

func validateAddress(what string, address string) {
	if len(address) == 0 {
		log.Fatalf("Field %s is empty", what)
	}

	addrToTest := address
	if !strings.Contains(address, ":") {
		addrToTest = address + ":25565"
	}

	if _, _, err := net.SplitHostPort(addrToTest); err != nil {
		log.Fatalf("Field '%s' with value %s is not a valid address: %v", what, address, err)
	}
}

func (c *Config) Init() {
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
	}

	// validate && apply
	level, err := log.ParseLevel(c.LogLevel)
	if err != nil {
		log.Fatalf("Invalid log level %s: %v", c.LogLevel, err)
	}
	log.SetLevel(level)

	validateAddress("listen", c.Listen)
	for i := range c.Routes {
		route := &c.Routes[i]
		for j := range route.Matches {
			validateAddress(fmt.Sprintf("routes[%d]match[%d]", i, j), route.Matches[j])
		}
		if len(route.Target) > 0 {
			validateAddress(fmt.Sprintf("routes[%d]target", i), route.Target)
		} else {
			log.Fatalf("routes[%d] does not specify the target", i)
		}
		if len(route.Mimic) > 0 {
			validateAddress(fmt.Sprintf("routes[%d]mimic", i), route.Mimic)
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
				if existed, ok := c.routeMap[addr]; ok {
					log.Warnf("Duplicated route matches %s, found in %s and %s", addr, existed.Name, route.Name)
				}
				c.routeMap[addr] = route
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

func (c *Config) GetRouteMap() map[string]*Route {
	return c.routeMap
}

func (c *Config) GetDefaultRoute() *Route {
	return c.defaultRoute
}
