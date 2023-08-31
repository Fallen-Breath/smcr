package config

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"time"
)

const DefaultRouteName = "default"

type Route struct {
	Name           string        `yaml:"name"`
	Matches        []string      `yaml:"matches"` // match any of them
	Target         string        `yaml:"target"`
	TargetSrv      string        `yaml:"target_srv"`
	Mimic          string        `yaml:"mimic"`           // optional
	Timeout        time.Duration `yaml:"timeout_ms"`      // optional, default 3s
	TimeoutMessage string        `yaml:"timeout_message"` // optional
}

type Config struct {
	Listen   string  `yaml:"listen"`
	LogLevel string  `yaml:"log_level"`
	Routes   []Route `yaml:"routes"`

	routeMap     map[string]*Route `yaml:"-"` // match_addr -> route
	defaultRoute *Route            `yaml:"-"`
}

func (r *Route) ResolveTarget() (string, error) {
	if len(r.Target) > 0 {
		return r.Target, nil
	} else if len(r.TargetSrv) > 0 {
		_, addrs, err := net.LookupSRV("minecraft", "tcp", r.TargetSrv)
		if err != nil {
			return "", fmt.Errorf("resolve srv %s failed: %v", r.TargetSrv, err)
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("srv %s has empty result", r.TargetSrv)
		}
		return fmt.Sprintf("%s:%d", addrs[0].Target, addrs[0].Port), nil
	} else {
		return "", fmt.Errorf("route %+v does not have any valid target", r)
	}
}

func validateAddress(what string, address string) {
	if len(address) == 0 {
		log.Fatalf("Field %s is empty", what)
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		log.Fatalf("Field '%s' with value %s is not a valid address: %v", what, address, err)
	}
}

func (c *Config) Init() {
	// fill default values
	for i := range c.Routes {
		route := &c.Routes[i]
		if route.Timeout <= 0 {
			route.Timeout = 5 * time.Second
		}
	}

	// validate

	validateAddress("listen", c.Listen)
	for i := range c.Routes {
		route := &c.Routes[i]
		for j := range route.Matches {
			validateAddress(fmt.Sprintf("routes[%d]match[%d]", i, j), route.Matches[j])
		}
		if len(route.Target) > 0 {
			validateAddress(fmt.Sprintf("routes[%d]target", i), route.Target)
		} else if len(route.TargetSrv) == 0 {
			log.Fatalf("routes[%d] does not specify any valid target", i)
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
				log.Warnf("matches field for default route is useless")
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
		log.Infof("- %s -> %s", addr, sr(route))
	}
	if c.defaultRoute != nil {
		log.Infof("- default route -> %s", sr(c.defaultRoute))
	}
}

// RouteFor might return nullable
func (c *Config) RouteFor(address string) *Route {
	if route, ok := c.routeMap[address]; ok {
		log.Debugf("Selected route %s for address %s", route.Name, address)
		return route
	}
	if c.defaultRoute != nil {
		log.Debugf("Selected default route for address %s", address)
		return c.defaultRoute
	}
	log.Debugf("No valid route for address %s", address)
	return nil
}
